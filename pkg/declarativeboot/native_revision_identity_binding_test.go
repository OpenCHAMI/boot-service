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
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestNativeRevisionIdentityMismatchDuringSelectionBinding(t *testing.T) {
	// Given a selected UID whose named binding points to a copied record.
	ctx := context.Background()
	dir := t.TempDir()
	if err := storage.InitFileBackend(dir); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ctx, GeneratedStore(), NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewRevisionService(manager)
	caps := Capabilities{Propose: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", caps))
	if err != nil {
		t.Fatal(err)
	}
	ref := revision.Reference{Name: "config", UID: a.Record.Metadata.UID}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "winner"}, Spec: v1.BootAssignmentSpec{ConfigRef: ref}})
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "revisions", "bootconfig", "records")
	pathA, pathB := filepath.Join(root, a.Record.Metadata.UID+".json"), filepath.Join(root, b.Record.Metadata.UID+".json")
	data, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	var record revision.Record[*v1.BootConfig]
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	record.Metadata.RevisionName = "B"
	data, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathA, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// When selection validates the revision's named binding.
	snapshot, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
	// Then it fails closed with both native and BOOT integrity classifications.
	var failure *ReferenceFailure
	if !matched || snapshot.Content.Kernel != "" || !errors.As(err, &failure) || !errors.Is(err, ErrRevisionCorrupt) || !errors.Is(err, revision.ErrRevisionIdentityMismatch) {
		t.Fatalf("matched=%v snapshot=%#v error=%v", matched, snapshot, err)
	}
}
