// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/storage"
)

func TestAssignmentRejectsWrongSeriesAndRetiredReference(t *testing.T) {
	// Given real series with an unreferenced retired revision and a second series.
	ctx := context.Background()
	dir := t.TempDir()
	backend, err := storage.NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	refs := newReferenceFixture(t, dir)
	b, _, _, err := refs.store.EnsureRevision(ctx, "config", "B", "b", &v1.BootConfig{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = refs.store.EnsureRevision(ctx, "config", "C", "c", &v1.BootConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := refs.store.RetireRevision(ctx, "config", b.Metadata.UID); err != nil {
		t.Fatal(err)
	}
	other, _, err := refs.store.CreateSeries(ctx, "other", "A", "x", &v1.BootConfig{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		id   string
	}{{"retired", b.Metadata.UID}, {"wrong-series", other.Metadata.UID}} {
		t.Run(tc.name, func(t *testing.T) {
			// When a mutation references that revision through the wrong or retired identity.
			a := assignment("target")
			a.Spec.ConfigRef.UID = tc.id
			_, err := m.Create(ctx, a)
			// Then the reference is rejected without an assignment write.
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v", err)
			}
			if _, err := m.GetByName(ctx, "target"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

type cancellationResolver struct{ entered chan struct{} }

func (r cancellationResolver) ResolveReference(ctx context.Context, _ revision.Reference) error {
	close(r.entered)
	<-ctx.Done()
	return ctx.Err()
}

func TestAssignmentCancellationAfterGateAcquired(t *testing.T) {
	// Given a resolver that waits while the mutation holds the gate.
	dir := t.TempDir()
	backend, err := storage.NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	r := cancellationResolver{entered: make(chan struct{})}
	m, err := NewManager(context.Background(), NewBackendStore(backend), r)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := m.Create(ctx, assignment("target")); done <- err }()
	<-r.entered
	// When the request is cancelled during reference validation.
	cancel()
	err = <-done
	// Then cancellation is propagated and nothing is persisted.
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
	ids, err := backend.List(context.Background(), "BootAssignment")
	if err != nil || len(ids) != 0 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
}
