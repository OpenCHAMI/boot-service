// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"fmt"
	"sort"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/uid"
)

var (
	ErrConflict     = errors.New("assignment conflict")
	ErrPrecondition = errors.New("assignment precondition failed")
	ErrBadCondition = errors.New("ambiguous assignment conditions")
	ErrInvalid      = errors.New("invalid assignment")
	ErrNotFound     = errors.New("assignment not found")
	// ErrStoreUncertain requires rebuilding the Manager before further operations.
	ErrStoreUncertain = errors.New("assignment store state uncertain")
)

// ReferenceResolver validates that a reference resolves within its named series
// and is not retired. It is called while the service gate is held.
type ReferenceResolver interface {
	ResolveReference(context.Context, revision.Reference) error
}

// Manager is the single in-process writer for one assignment data directory.
// File storage has no cross-process writer coordination.
type Manager struct {
	gate        chan struct{}
	store       AssignmentStore
	resolver    ReferenceResolver
	byName      map[string]string
	byUID       map[string]v1.BootAssignment
	unavailable error
}

// Snapshot is a detached committed assignment and its strong ETag.
type Snapshot struct {
	Assignment v1.BootAssignment
	ETag       string
}

// NewManager rebuilds the memory-only index without resolving references, so
// restored missing references can be repaired through management operations.
func NewManager(ctx context.Context, store AssignmentStore, resolver ReferenceResolver) (*Manager, error) {
	if store == nil || resolver == nil {
		return nil, fmt.Errorf("manager requires store and resolver: %w", ErrInvalid)
	}
	m := &Manager{gate: make(chan struct{}, 1), store: store, resolver: resolver, byName: map[string]string{}, byUID: map[string]v1.BootAssignment{}}
	ids, err := store.ListUIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	for _, id := range ids {
		a, err := store.Load(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load assignment %q: %w", id, err)
		}
		if a == nil {
			return nil, fmt.Errorf("nil assignment %q: %w", id, ErrInvalid)
		}
		parsed, parseErr := (uid.Codec{}).Parse(a.Metadata.UID)
		if parseErr != nil || parsed.Format != uid.FormatTypeID || parsed.Prefix != "bootassignment" || a.Metadata.UID != id || a.Metadata.CreatedAt.IsZero() || a.Metadata.UpdatedAt.IsZero() || a.Metadata.UpdatedAt.Before(a.Metadata.CreatedAt) || a.Kind != "BootAssignment" || a.APIVersion != "v1" {
			return nil, fmt.Errorf("invalid stored assignment identity %q: %w", id, ErrInvalid)
		}
		copy := cloneAssignment(*a)
		if err := copy.Validate(ctx); err != nil {
			return nil, fmt.Errorf("invalid stored assignment %q: %w", id, err)
		}
		if _, exists := m.byName[a.Metadata.Name]; exists {
			return nil, fmt.Errorf("duplicate assignment name %q: %w", a.Metadata.Name, ErrConflict)
		}
		m.byName[a.Metadata.Name] = id
		m.byUID[id] = cloneAssignment(*a)
	}
	return m, nil
}

// LockedView exposes detached assignment snapshots only during WithMutation.
type LockedView struct{ manager *Manager }

// Assignments returns independent snapshots without acquiring the gate again.
func (v LockedView) Assignments() []Snapshot { return v.manager.listLocked() }

// WithMutation serializes assignment and revision mutations and coherent read
// snapshots. Lock order is service gate then native revision-store mutex. The
// callback must not call Manager methods or retain the view after it returns.
func (m *Manager) WithMutation(ctx context.Context, fn func(LockedView) error) error {
	select {
	case m.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-m.gate }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.unavailable != nil {
		return m.unavailable
	}
	return fn(LockedView{manager: m})
}

func (m *Manager) listLocked() []Snapshot {
	names := make([]string, 0, len(m.byName))
	for name := range m.byName {
		names = append(names, name)
	}
	sort.Strings(names)
	results := make([]Snapshot, 0, len(names))
	for _, name := range names {
		results = append(results, snapshot(m.byUID[m.byName[name]]))
	}
	return results
}

// List returns detached snapshots in name order.
func (m *Manager) List(ctx context.Context) ([]Snapshot, error) {
	var result []Snapshot
	err := m.WithMutation(ctx, func(v LockedView) error { result = v.Assignments(); return nil })
	return result, err
}

// GetByName and GetByUID return detached committed snapshots.
func (m *Manager) GetByName(ctx context.Context, name string) (Snapshot, error) {
	var result Snapshot
	err := m.WithMutation(ctx, func(_ LockedView) error {
		id, ok := m.byName[name]
		if !ok {
			return ErrNotFound
		}
		result = snapshot(m.byUID[id])
		return nil
	})
	return result, err
}

func (m *Manager) GetByUID(ctx context.Context, id string) (Snapshot, error) {
	var result Snapshot
	err := m.WithMutation(ctx, func(_ LockedView) error {
		a, ok := m.byUID[id]
		if !ok {
			return ErrNotFound
		}
		result = snapshot(a)
		return nil
	})
	return result, err
}
