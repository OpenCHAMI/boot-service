// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/storage"
)

func TestAssignmentStartupRejectsCorruption(t *testing.T) {
	// Given a valid committed assignment in real UID-file storage.
	dir := t.TempDir()
	ctx := context.Background()
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
	m, err := NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatal(err)
	}
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	// When a malformed UID file is present during startup.
	path := filepath.Join(dir, "bootassignments", created.Assignment.Metadata.UID+".json")
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = NewManager(ctx, NewBackendStore(backend), refs)
	// Then startup fails rather than silently omitting the corrupt file.
	if err == nil {
		t.Fatal("corrupt file accepted")
	}
}

func TestAssignmentStartupRejectsDuplicateNames(t *testing.T) {
	// Given two valid persisted UID files sharing one name.
	dir := t.TempDir()
	ctx := context.Background()
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
	one, err := m.Create(ctx, assignment("first"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := m.Create(ctx, assignment("second"))
	if err != nil {
		t.Fatal(err)
	}
	duplicate := two.Assignment
	duplicate.Metadata.Name = one.Assignment.Metadata.Name
	raw, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Save(ctx, "BootAssignment", duplicate.Metadata.UID, raw); err != nil {
		t.Fatal(err)
	}
	// When a new manager rebuilds the index.
	_, err = NewManager(ctx, store, refs)
	// Then it fails instead of choosing a first match.
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v", err)
	}
}

type failingStore struct {
	AssignmentStore
	failure error
}

func (s failingStore) Save(context.Context, *v1.BootAssignment) error { return s.failure }

func TestAssignmentSaveFailureDoesNotPublishIndex(t *testing.T) {
	// Given a store that fails before saving.
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
	failure := errors.New("storage failure")
	m, err := NewManager(context.Background(), failingStore{NewBackendStore(backend), failure}, newReferenceFixture(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	// When create fails at storage.
	_, err = m.Create(context.Background(), assignment("target"))
	// Then the error is returned and the name is not published.
	if !errors.Is(err, failure) {
		t.Fatalf("got %v", err)
	}
	if _, err := m.GetByName(context.Background(), "target"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
