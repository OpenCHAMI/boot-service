// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/uid"
)

// MutationResult reports whether the operation created a new assignment.
type MutationResult struct {
	Snapshot
	Created bool
}

// Create is create-only; a duplicate name returns ErrConflict.
func (m *Manager) Create(ctx context.Context, input v1.BootAssignment) (MutationResult, error) {
	var result MutationResult
	err := m.WithMutation(ctx, func(_ LockedView) error {
		if _, exists := m.byName[input.Metadata.Name]; exists {
			return ErrConflict
		}
		var err error
		result, err = m.createLocked(ctx, input)
		return err
	})
	return result, err
}

// PutByName creates only with If-None-Match: *; updates require the current ETag.
func (m *Manager) PutByName(ctx context.Context, name string, input v1.BootAssignment, c Conditions) (MutationResult, error) {
	var result MutationResult
	err := m.WithMutation(ctx, func(_ LockedView) error {
		id, exists := m.byName[name]
		current := ""
		if exists {
			current = snapshot(m.byUID[id]).ETag
		}
		if err := c.check(current, exists); err != nil {
			return err
		}
		if input.Metadata.Name != "" && input.Metadata.Name != name {
			return ErrConflict
		}
		input.Metadata.Name = name
		var err error
		if !exists {
			result, err = m.createLocked(ctx, input)
		} else {
			result, err = m.updateLocked(ctx, m.byUID[id], input)
		}
		return err
	})
	return result, err
}

// PutByUID applies a complete desired assignment under an exact strong ETag.
func (m *Manager) PutByUID(ctx context.Context, id string, input v1.BootAssignment, c Conditions) (MutationResult, error) {
	var result MutationResult
	err := m.WithMutation(ctx, func(_ LockedView) error {
		if err := c.Validate(); err != nil {
			return err
		}
		old, exists := m.byUID[id]
		if !exists {
			return ErrPrecondition
		}
		if err := c.check(snapshot(old).ETag, true); err != nil {
			return err
		}
		var err error
		result, err = m.updateLocked(ctx, old, input)
		return err
	})
	return result, err
}

// TransformByUID applies a spec-only transform to a detached current spec while
// holding the gate. The callback must not call Manager methods or retain state.
func (m *Manager) TransformByUID(ctx context.Context, id string, c Conditions, transform func(v1.BootAssignmentSpec) (v1.BootAssignmentSpec, error)) (MutationResult, error) {
	var result MutationResult
	err := m.WithMutation(ctx, func(_ LockedView) error {
		var err error
		result, err = m.transformLocked(ctx, id, c, transform)
		return err
	})
	return result, err
}

// TransformByName has the same atomic spec-only contract as TransformByUID.
func (m *Manager) TransformByName(ctx context.Context, name string, c Conditions, transform func(v1.BootAssignmentSpec) (v1.BootAssignmentSpec, error)) (MutationResult, error) {
	var result MutationResult
	err := m.WithMutation(ctx, func(_ LockedView) error {
		if err := c.Validate(); err != nil {
			return err
		}
		id, exists := m.byName[name]
		if !exists {
			return ErrPrecondition
		}
		var err error
		result, err = m.transformLocked(ctx, id, c, transform)
		return err
	})
	return result, err
}

func (m *Manager) transformLocked(ctx context.Context, id string, c Conditions, transform func(v1.BootAssignmentSpec) (v1.BootAssignmentSpec, error)) (MutationResult, error) {
	if err := c.Validate(); err != nil {
		return MutationResult{}, err
	}
	old, exists := m.byUID[id]
	if !exists {
		return MutationResult{}, ErrPrecondition
	}
	if err := c.check(snapshot(old).ETag, true); err != nil {
		return MutationResult{}, err
	}
	input := cloneAssignment(old)
	spec, err := transform(input.Spec)
	if err != nil {
		return MutationResult{}, err
	}
	input.Spec = spec
	return m.updateLocked(ctx, old, input)
}

func (m *Manager) createLocked(ctx context.Context, input v1.BootAssignment) (MutationResult, error) {
	a := cloneAssignment(input)
	if a.Metadata.UID != "" {
		return MutationResult{}, ErrConflict
	}
	a.APIVersion, a.Kind = "v1", "BootAssignment"
	if err := normalizeAndValidate(ctx, &a); err != nil {
		return MutationResult{}, err
	}
	if err := m.resolver.ResolveReference(ctx, a.Spec.ConfigRef); err != nil {
		return MutationResult{}, referenceError(err)
	}
	id, err := (uid.Codec{}).Generate("bootassignment")
	if err != nil {
		return MutationResult{}, err
	}
	a.Metadata.UID = id
	a.Metadata.CreatedAt = time.Now().UTC()
	a.Metadata.UpdatedAt = a.Metadata.CreatedAt
	if err := ctx.Err(); err != nil {
		return MutationResult{}, err
	}
	if err := m.store.Save(ctx, &a); err != nil {
		return MutationResult{}, fmt.Errorf("save assignment: %w", err)
	}
	m.byName[a.Metadata.Name] = id
	m.byUID[id] = cloneAssignment(a)
	return MutationResult{Snapshot: snapshot(a), Created: true}, nil
}

func (m *Manager) updateLocked(ctx context.Context, old, input v1.BootAssignment) (MutationResult, error) {
	if input.Metadata.Name != "" && input.Metadata.Name != old.Metadata.Name || input.Metadata.UID != "" && input.Metadata.UID != old.Metadata.UID {
		return MutationResult{}, ErrConflict
	}
	a := cloneAssignment(old)
	a.Spec = cloneAssignment(input).Spec
	if err := normalizeAndValidate(ctx, &a); err != nil {
		return MutationResult{}, err
	}
	if err := m.resolver.ResolveReference(ctx, a.Spec.ConfigRef); err != nil {
		return MutationResult{}, referenceError(err)
	}
	previous := cloneAssignment(old)
	if err := normalizeAndValidate(ctx, &previous); err != nil {
		return MutationResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return MutationResult{}, err
	}
	if reflect.DeepEqual(a.Spec, previous.Spec) {
		return MutationResult{Snapshot: snapshot(old)}, nil
	}
	return m.commitAssignmentLocked(ctx, old, a)
}

func referenceError(err error) error {
	if errors.Is(err, ErrInvalid) || errors.Is(err, revision.ErrSeriesNotFound) || errors.Is(err, revision.ErrRevisionNotFound) || errors.Is(err, revision.ErrRevisionRetired) || errors.Is(err, revision.ErrInvalidReference) || errors.Is(err, revision.ErrUnsupportedSelector) {
		return fmt.Errorf("reference: %w: %w", ErrInvalid, err)
	}
	return fmt.Errorf("resolve reference: %w", err)
}

func normalizeAndValidate(ctx context.Context, a *v1.BootAssignment) error {
	if err := a.Validate(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if a.Spec.Profile == "default" {
		a.Spec.Profile = ""
	}
	if s := a.Spec.Selector; s != nil {
		slices.Sort(s.MACs)
		s.MACs = slices.Compact(s.MACs)
		slices.Sort(s.NIDs)
		s.NIDs = slices.Compact(s.NIDs)
		slices.Sort(s.XNames)
		s.XNames = slices.Compact(s.XNames)
		slices.Sort(s.Groups)
		s.Groups = slices.Compact(s.Groups)
		if len(s.MACs) == 0 {
			s.MACs = nil
		}
		if len(s.NIDs) == 0 {
			s.NIDs = nil
		}
		if len(s.XNames) == 0 {
			s.XNames = nil
		}
		if len(s.Groups) == 0 {
			s.Groups = nil
		}
		if len(s.MACs)+len(s.NIDs)+len(s.XNames)+len(s.Groups) == 0 {
			a.Spec.Selector = nil
		}
	}
	return nil
}
