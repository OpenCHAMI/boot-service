// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func selectionInventory(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	files := make(map[string][32]byte)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestSelectionSnapshotDoesNotWriteStore(t *testing.T) {
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
	created, err := svc.CreateSeries(ctx, revisionRequest("os", "A", "/kernel-a", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "all"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	before := selectionInventory(t, dir)
	snapshot, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
	if err != nil || !matched || snapshot.RevisionUID != created.Record.Metadata.UID {
		t.Fatalf("snapshot=%#v matched=%v err=%v", snapshot, matched, err)
	}
	if after := selectionInventory(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatalf("selection changed store contents: before=%v after=%v", before, after)
	}
}
