// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"sync"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/storage"
)

func testManager(t *testing.T) (*Manager, *storage.FileBackend) {
	t.Helper()
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
	m, err := NewManager(context.Background(), NewBackendStore(backend), newReferenceFixture(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	return m, backend
}

func assignment(name string) v1.BootAssignment {
	return v1.BootAssignment{Metadata: resource.Metadata{Name: name}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "config"}}}
}

func TestAssignmentConcurrentFirstCreate(t *testing.T) {
	// Given a shared manager and identical create preconditions.
	m, _ := testManager(t)
	const count = 20
	start := make(chan struct{})
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := m.PutByName(context.Background(), "shared", assignment("shared"), Conditions{IfNoneMatch: []string{"*"}})
			errs <- err
		}()
	}
	// When all requests contend for the first creation.
	close(start)
	wg.Wait()
	close(errs)
	// Then exactly one commits and all others fail their precondition.
	wins := 0
	for err := range errs {
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrPrecondition) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("winners = %d", wins)
	}
}

func TestAssignmentConcurrentUpdates(t *testing.T) {
	// Given two different updates using one committed ETag.
	m, _ := testManager(t)
	created, err := m.Create(context.Background(), assignment("shared"))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, priority := range []int{1, 2} {
		go func() {
			<-start
			next := assignment("shared")
			next.Spec.Priority = priority
			_, err := m.PutByUID(context.Background(), created.Assignment.Metadata.UID, next, Conditions{IfMatch: []string{created.ETag}})
			errs <- err
		}()
	}
	// When both updates race.
	close(start)
	// Then exactly one succeeds and the other is stale.
	wins, stale := 0, 0
	for range 2 {
		err := <-errs
		if err == nil {
			wins++
		} else if errors.Is(err, ErrPrecondition) {
			stale++
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("wins=%d stale=%d", wins, stale)
	}
}
