// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/storage"
)

func TestAssignmentStatusConditions(t *testing.T) {
	// Given a persisted assignment with a current ETag.
	m, _ := testManager(t)
	ctx := context.Background()
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		c    Conditions
		want error
	}{
		{"missing", Conditions{}, ErrPrecondition},
		{"wildcard", Conditions{IfMatch: []string{"*"}}, ErrPrecondition},
		{"weak", Conditions{IfMatch: []string{"W/" + created.ETag}}, ErrPrecondition},
		{"stale", Conditions{IfMatch: []string{`"stale"`}}, ErrPrecondition},
		{"none-match", Conditions{IfNoneMatch: []string{"*"}}, ErrPrecondition},
		{"ambiguous", Conditions{IfMatch: []string{created.ETag}, IfNoneMatch: []string{"*"}}, ErrBadCondition},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// When a status mutation uses a nonmatching condition.
			called := false
			_, err := m.TransformStatusByUID(ctx, StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: tc.c}, func(s v1.BootAssignmentStatus) (v1.BootAssignmentStatus, error) { called = true; return s, nil })
			// Then it rejects the request before running the callback or changing state.
			if !errors.Is(err, tc.want) || called {
				t.Fatalf("err=%v called=%t", err, called)
			}
			got, err := m.GetByUID(ctx, created.Assignment.Metadata.UID)
			if err != nil || got.ETag != created.ETag {
				t.Fatalf("got=%#v err=%v", got, err)
			}
		})
	}
}

type statusWriteCounter struct {
	AssignmentStore
	updates int
}

func (s *statusWriteCounter) Update(ctx context.Context, a *v1.BootAssignment) error {
	s.updates++
	return s.AssignmentStore.Update(ctx, a)
}

func TestAssignmentStatusReplayDoesNotWrite(t *testing.T) {
	// Given a file-backed manager with a counted storage adapter.
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
	store := &statusWriteCounter{AssignmentStore: NewBackendStore(backend)}
	m, err := NewManager(ctx, store, newReferenceFixture(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	// When the unchanged initial status is applied under its ETag.
	result, err := m.PutStatusByUID(ctx, StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}, created.Assignment.Status)
	// Then no storage update is attempted and identity is unchanged.
	if err != nil || store.updates != 0 || result.ETag != created.ETag || !result.Assignment.Metadata.UpdatedAt.Equal(created.Assignment.Metadata.UpdatedAt) {
		t.Fatalf("writes=%d result=%#v err=%v", store.updates, result, err)
	}
}

func TestAssignmentStatusReplayAndIdentity(t *testing.T) {
	// Given a stored assignment with labels, annotations and a selector.
	ctx := context.Background()
	m, backend := testManager(t)
	a := assignment("target")
	a.Metadata.Labels = map[string]string{"l": "v"}
	a.Metadata.Annotations = map[string]string{"a": "v"}
	a.Spec.Selector = &v1.BootAssignmentSelector{Groups: []string{"compute"}}
	created, err := m.Create(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	target := StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}
	updated, err := m.TransformStatusByUID(ctx, target, func(s v1.BootAssignmentStatus) (v1.BootAssignmentStatus, error) { s.Phase = "ready"; return s, nil })
	if err != nil {
		t.Fatal(err)
	}
	before, err := backend.Load(ctx, "BootAssignment", target.UID)
	if err != nil {
		t.Fatal(err)
	}
	// When the same status is put again under its new ETag.
	target.Conditions.IfMatch = []string{updated.ETag}
	replay, err := m.PutStatusByUID(ctx, target, updated.Assignment.Status)
	// Then the complete stored resource, timestamp, identity and ETag are unchanged.
	if err != nil {
		t.Fatal(err)
	}
	after, err := backend.Load(ctx, "BootAssignment", target.UID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || replay.ETag != updated.ETag || !reflect.DeepEqual(replay.Assignment, updated.Assignment) {
		t.Fatal("status replay changed committed data")
	}
	if !reflect.DeepEqual(created.Assignment.Spec, updated.Assignment.Spec) || !reflect.DeepEqual(created.Assignment.Metadata.Labels, updated.Assignment.Metadata.Labels) || !reflect.DeepEqual(created.Assignment.Metadata.Annotations, updated.Assignment.Metadata.Annotations) || created.Assignment.Metadata.Name != updated.Assignment.Metadata.Name || created.Assignment.Metadata.UID != updated.Assignment.Metadata.UID {
		t.Fatal("status changed spec or identity")
	}
}

func TestAssignmentStatusPersistsOnReopen(t *testing.T) {
	// Given a status mutation committed to an isolated FileBackend.
	ctx := context.Background()
	dir := t.TempDir()
	backend, err := storage.NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	refs := newReferenceFixture(t, dir)
	m, err := NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatal(err)
	}
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := m.PutStatusByUID(ctx, StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}, v1.BootAssignmentStatus{Phase: "ready", Error: "detail"})
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	// When a fresh backend and manager reopen the directory.
	backend, err = storage.NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	m, err = NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.GetByUID(ctx, created.Assignment.Metadata.UID)
	// Then the committed status and strong ETag survive restart.
	if err != nil || got.ETag != updated.ETag || got.Assignment.Status != updated.Assignment.Status {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}
