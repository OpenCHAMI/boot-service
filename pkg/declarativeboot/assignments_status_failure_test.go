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

type uncertainStatusStore struct {
	AssignmentStore
	failure     error
	loadFailure error
}

func (s uncertainStatusStore) Update(ctx context.Context, a *v1.BootAssignment) error {
	if err := s.AssignmentStore.Update(ctx, a); err != nil {
		return err
	}
	return s.failure
}

func (s uncertainStatusStore) Load(ctx context.Context, uid string) (*v1.BootAssignment, error) {
	if s.loadFailure != nil {
		return nil, s.loadFailure
	}
	return s.AssignmentStore.Load(ctx, uid)
}

func TestAssignmentStatusWriteErrorReconcilesCommittedFile(t *testing.T) {
	// Given a store that commits and then returns an error.
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
	base := NewBackendStore(backend)
	refs := newReferenceFixture(t, dir)
	original, err := NewManager(ctx, base, refs)
	if err != nil {
		t.Fatal(err)
	}
	created, err := original.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("ambiguous write")
	m, err := NewManager(ctx, uncertainStatusStore{AssignmentStore: base, failure: failure}, refs)
	if err != nil {
		t.Fatal(err)
	}
	target := StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}
	// When a status write returns an error after reaching storage.
	_, err = m.PutStatusByUID(ctx, target, v1.BootAssignmentStatus{Phase: "ready"})
	// Then the error is returned but subsequent reads see the committed status and ETag.
	if !errors.Is(err, failure) {
		t.Fatalf("write error: %v", err)
	}
	got, err := m.GetByUID(ctx, target.UID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Assignment.Status.Phase != "ready" || got.ETag == created.ETag {
		t.Fatalf("manager retained stale state: %#v", got)
	}
	_, err = m.PutByUID(ctx, target.UID, assignment("target"), target.Conditions)
	if !errors.Is(err, ErrPrecondition) {
		t.Fatalf("old ETag accepted: %v", err)
	}
}

func TestAssignmentStatusWriteErrorFailsClosedWhenReloadFails(t *testing.T) {
	// Given a committed assignment whose store later fails both update and reload.
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
	base := NewBackendStore(backend)
	refs := newReferenceFixture(t, dir)
	original, err := NewManager(ctx, base, refs)
	if err != nil {
		t.Fatal(err)
	}
	created, err := original.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(ctx, base, refs)
	if err != nil {
		t.Fatal(err)
	}
	m.store = uncertainStatusStore{AssignmentStore: base, failure: errors.New("write"), loadFailure: errors.New("reload")}
	target := StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}
	// When an uncertain update cannot be reconciled.
	_, err = m.PutStatusByUID(ctx, target, v1.BootAssignmentStatus{Phase: "ready"})
	if err == nil {
		t.Fatal("update succeeded")
	}
	// Then the manager fails closed rather than returning its stale snapshot.
	if _, err := m.GetByUID(ctx, target.UID); !errors.Is(err, ErrStoreUncertain) {
		t.Fatalf("manager must fail closed: %v", err)
	}
}

type cancelAfterCommitStore struct {
	AssignmentStore
	cancel context.CancelFunc
}

func (s cancelAfterCommitStore) Update(ctx context.Context, a *v1.BootAssignment) error {
	if err := s.AssignmentStore.Update(ctx, a); err != nil {
		return err
	}
	s.cancel()
	return nil
}

func TestAssignmentStatusCancellationAfterStorageCommit(t *testing.T) {
	// Given a store that cancels the request just after committing.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
	base := NewBackendStore(backend)
	refs := newReferenceFixture(t, dir)
	m, err := NewManager(ctx, cancelAfterCommitStore{AssignmentStore: base, cancel: cancel}, refs)
	if err != nil {
		t.Fatal(err)
	}
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	// When status is written and cancellation arrives before returning.
	_, err = m.PutStatusByUID(ctx, StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}, v1.BootAssignmentStatus{Phase: "ready"})
	// Then cancellation is returned and the Manager still reflects the committed file.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	got, err := m.GetByUID(context.Background(), created.Assignment.Metadata.UID)
	if err != nil || got.Assignment.Status.Phase != "ready" || got.ETag == created.ETag {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}
