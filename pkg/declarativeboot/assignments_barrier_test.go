// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/storage"
)

type blockingStore struct {
	AssignmentStore
	entered chan struct{}
	release chan struct{}
}

func (s blockingStore) Save(ctx context.Context, a *v1.BootAssignment) error {
	close(s.entered)
	select {
	case <-s.release:
		return s.AssignmentStore.Save(ctx, a)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestAssignmentWaitsForDurableSave(t *testing.T) {
	// Given a real store whose save is held at a deterministic barrier.
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
	store := blockingStore{AssignmentStore: NewBackendStore(backend), entered: make(chan struct{}), release: make(chan struct{})}
	m, err := NewManager(context.Background(), store, newReferenceFixture(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := m.Create(context.Background(), assignment("target")); done <- err }()
	<-store.entered
	// When save has not completed.
	select {
	case err := <-done:
		t.Fatalf("returned before save: %v", err)
	default:
	}
	close(store.release)
	// Then successful completion follows the real file write.
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	ids, err := backend.List(context.Background(), "BootAssignment")
	if err != nil || len(ids) != 1 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
}

func TestAssignmentCancellationWhileWaiting(t *testing.T) {
	// Given the gate is held by another operation.
	m, _ := testManager(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- m.WithMutation(context.Background(), func(LockedView) error { close(entered); <-release; return nil })
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// When a cancelled request waits for the gate.
	_, err := m.Create(ctx, assignment("target"))
	close(release)
	if gateErr := <-done; gateErr != nil {
		t.Fatal(gateErr)
	}
	// Then no assignment was persisted or indexed.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if _, err := m.GetByName(context.Background(), "target"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
