// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/storage"
)

func TestAssignmentMissingReferenceSurvivesReopen(t *testing.T) {
	// Given a valid assignment whose referenced series is absent in a restored store.
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
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	missing := created.Assignment
	missing.Spec.ConfigRef.Name = "missing"
	raw, err := json.Marshal(missing)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Save(ctx, "BootAssignment", missing.Metadata.UID, raw); err != nil {
		t.Fatal(err)
	}
	// When startup rebuilds its index from that restored assignment.
	reopened, err := NewManager(ctx, store, refs)
	// Then management can read and repair the assignment.
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.GetByName(ctx, "target")
	if err != nil {
		t.Fatal(err)
	}
	if got.Assignment.Spec.ConfigRef.Name != "missing" {
		t.Fatal("reference changed")
	}
	if _, err := reopened.PutByName(ctx, "target", assignment("target"), Conditions{IfMatch: []string{got.ETag}}); err != nil {
		t.Fatal(err)
	}
}

func TestAssignmentCaseDistinctAndLongName(t *testing.T) {
	// Given distinct case-sensitive names and a valid 253-byte name.
	m, _ := testManager(t)
	ctx := context.Background()
	long := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	// When each assignment is stored.
	for _, name := range []string{"aaG", "aaa", long} {
		if _, err := m.Create(ctx, assignment(name)); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
	}
	// Then they remain independent UID-file entries.
	list, err := m.List(ctx)
	if err != nil || len(list) != 3 {
		t.Fatalf("list=%v err=%v", list, err)
	}
}

func TestAssignmentRejectsRenameAndDeletes(t *testing.T) {
	// Given a committed assignment and its strong ETag.
	m, _ := testManager(t)
	ctx := context.Background()
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	condition := Conditions{IfMatch: []string{created.ETag}}
	// When an update attempts to rename it.
	_, err = m.PutByUID(ctx, created.Assignment.Metadata.UID, assignment("other"), condition)
	// Then the rename is rejected and the original can be deleted conditionally.
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("rename: %v", err)
	}
	if err := m.DeleteByName(ctx, "target", Conditions{}); !errors.Is(err, ErrPrecondition) {
		t.Fatalf("missing precondition: %v", err)
	}
	if err := m.DeleteByName(ctx, "target", condition); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetByUID(ctx, created.Assignment.Metadata.UID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: %v", err)
	}
}

func TestAssignmentReplayWithOldNoncanonicalSpec(t *testing.T) {
	// Given a valid stored assignment with older noncanonical selector spelling.
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
	a := assignment("target")
	a.Spec.Selector = &v1.BootAssignmentSelector{MACs: []string{"aa:bb:cc:dd:ee:ff"}}
	created, err := m.Create(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	old := created.Assignment
	old.Spec.Profile = "default"
	old.Spec.Selector.MACs[0] = "AA-BB-CC-DD-EE-FF"
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Save(ctx, "BootAssignment", old.Metadata.UID, raw); err != nil {
		t.Fatal(err)
	}
	m, err = NewManager(ctx, store, refs)
	if err != nil {
		t.Fatal(err)
	}
	before, err := m.GetByName(ctx, "target")
	if err != nil {
		t.Fatal(err)
	}
	// When the equivalent normalized desired assignment is applied.
	after, err := m.PutByName(ctx, "target", a, Conditions{IfMatch: []string{before.ETag}})
	// Then no timestamp or ETag changes.
	if err != nil || after.ETag != before.ETag {
		t.Fatalf("before=%q after=%q err=%v", before.ETag, after.ETag, err)
	}
}
