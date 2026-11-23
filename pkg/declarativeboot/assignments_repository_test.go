// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/storage"
)

type referenceFixture struct {
	store *revision.FileStore[*v1.BootConfig]
}

func (f referenceFixture) ResolveReference(ctx context.Context, ref revision.Reference) error {
	_, err := f.store.Resolve(ctx, ref)
	return err
}

func newReferenceFixture(t *testing.T, dir string) referenceFixture {
	t.Helper()
	store, err := revision.NewFileStore[*v1.BootConfig](dir, "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.CreateSeries(context.Background(), "config", "A", "digest", &v1.BootConfig{})
	if err != nil {
		t.Fatal(err)
	}
	return referenceFixture{store: store}
}

func TestAssignmentPersistsAcrossReopen(t *testing.T) {
	// Given a real, isolated file backend and a valid reference.
	ctx := context.Background()
	dir := t.TempDir()
	backend, err := storage.NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	refs := newReferenceFixture(t, dir)
	manager, err := NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatal(err)
	}
	before, err := backend.List(ctx, "BootAssignment")
	if err != nil || len(before) != 0 {
		t.Fatalf("initial UIDs %v: %v", before, err)
	}
	input := v1.BootAssignment{Metadata: resource.Metadata{Name: "Example"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "config"}}}
	// When the assignment is committed and the backend is reopened.
	created, err := manager.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	backend, err = storage.NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	reopened, err := NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.GetByName(ctx, "Example")
	// Then the UID and strong ETag are identical to the committed result.
	if err != nil {
		t.Fatal(err)
	}
	if got.Assignment.Metadata.UID != created.Assignment.Metadata.UID || got.ETag != created.ETag {
		t.Fatalf("reopened %#v, created %#v", got, created)
	}
	if _, err := reopened.Create(ctx, input); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate after reopen: %v", err)
	}
	t.Logf("initial UID count=%d; reopened UID=%s ETag=%s", len(before), got.Assignment.Metadata.UID, got.ETag)
}
