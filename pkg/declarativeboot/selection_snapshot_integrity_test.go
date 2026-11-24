// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestSelectionSnapshotRejectsChangedContent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := storage.InitFileBackend(dir); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(ctx, GeneratedStore(), NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewRevisionService(m)
	a, err := s.CreateSeries(ctx, revisionRequest("os", "A", "/kernel-a", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "all"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "revisions", "bootconfig", "records", a.Record.Metadata.UID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), "/kernel-a", "/kernel-b", 1)
	if changed == string(data) {
		t.Fatal("fixture content not found")
	}
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	_, matched, err := s.SelectSnapshot(ctx, v1.NodeSpec{}, "")
	var failure *ReferenceFailure
	if !matched || !errors.As(err, &failure) || !errors.Is(err, ErrRevisionCorrupt) {
		t.Fatalf("matched=%v error=%v", matched, err)
	}
}

func TestSelectionSnapshotRejectsChangedRevisionIdentity(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := storage.InitFileBackend(dir); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(ctx, GeneratedStore(), NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewRevisionService(m)
	caps := Capabilities{Propose: true}
	a, err := s.CreateSeries(ctx, revisionRequest("os", "A", "/kernel-a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.EnsureRevision(ctx, revisionRequest("os", "B", "/kernel-b", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "all"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "revisions", "bootconfig", "records", a.Record.Metadata.UID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), a.Record.Metadata.UID, b.Record.Metadata.UID, 1)
	if changed == string(data) {
		t.Fatal("fixture revision UID not found")
	}
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	_, matched, err := s.SelectSnapshot(ctx, v1.NodeSpec{}, "")
	var failure *ReferenceFailure
	if !matched || !errors.As(err, &failure) || !errors.Is(err, ErrRevisionCorrupt) {
		t.Fatalf("matched=%v error=%v", matched, err)
	}
}
