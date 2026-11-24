// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/storage"
)

type failedChanges struct {
	AssignmentStore
	failure error
}

func (s failedChanges) Update(context.Context, *v1.BootAssignment) error { return s.failure }
func (s failedChanges) Delete(context.Context, string) error             { return s.failure }

func TestAssignmentFailedChangesKeepCommittedSnapshot(t *testing.T) {
	// Given a committed resource and an adapter that fails updates and deletes.
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
	store := NewBackendStore(backend)
	initial, err := NewManager(ctx, store, refs)
	if err != nil {
		t.Fatal(err)
	}
	created, err := initial.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("write failure")
	m, err := NewManager(ctx, failedChanges{store, failure}, refs)
	if err != nil {
		t.Fatal(err)
	}
	conditions := Conditions{IfMatch: []string{created.ETag}}
	// When a changed spec and a delete both fail in storage.
	next := assignment("target")
	next.Spec.Priority = 50
	_, updateErr := m.PutByUID(ctx, created.Assignment.Metadata.UID, next, conditions)
	deleteErr := m.DeleteByUID(ctx, created.Assignment.Metadata.UID, conditions)
	// Then the original index, spec and ETag remain visible.
	if !errors.Is(updateErr, failure) || !errors.Is(deleteErr, failure) {
		t.Fatalf("update=%v delete=%v", updateErr, deleteErr)
	}
	got, err := m.GetByName(ctx, "target")
	if err != nil || got.ETag != created.ETag || got.Assignment.Spec.Priority != 0 {
		t.Fatalf("got %#v: %v", got, err)
	}
}

func TestAssignmentStartupRejectsInvalidMetadata(t *testing.T) {
	// Given a file whose metadata UID no longer matches its filename.
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
	store := NewBackendStore(backend)
	m, err := NewManager(ctx, store, refs)
	if err != nil {
		t.Fatal(err)
	}
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	broken := created.Assignment
	broken.Metadata.UID = ""
	raw, err := json.Marshal(broken)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Save(ctx, "BootAssignment", created.Assignment.Metadata.UID, raw); err != nil {
		t.Fatal(err)
	}
	// When the manager rebuilds its index.
	_, err = NewManager(ctx, store, refs)
	// Then invalid identity fails startup.
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}
