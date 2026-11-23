// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestSelectionSnapshotUsesCurrentDefault(t *testing.T) {
	ctx := context.Background()
	svc, manager := revisionServiceFixture(t)
	caps := Capabilities{Propose: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("os", "A", "/kernel-a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("os", "B", "/kernel-b", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "all"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	first, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{XName: "x0c0s0b0n0"}, "")
	if err != nil || !matched || first.Content.Kernel != "/kernel-a" || first.RevisionUID != a.Record.Metadata.UID {
		t.Fatalf("initial: %#v, %v, %v", first, matched, err)
	}
	_, etag, err := svc.GetSeries(ctx, "os")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.PromoteDefault(ctx, PromotionRequest{Name: "os", UID: b.Record.Metadata.UID, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
	if err != nil {
		t.Fatal(err)
	}
	second, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{XName: "x0c0s0b0n0"}, "")
	if err != nil || !matched || second.Content.Kernel != "/kernel-b" || second.RevisionUID != b.Record.Metadata.UID {
		t.Fatalf("promoted: %#v, %v, %v", second, matched, err)
	}
	if first.Content.Kernel != "/kernel-a" {
		t.Fatal("previous snapshot changed")
	}
}

func TestSelectedLostRecordRetainsDiagnosticPointer(t *testing.T) {
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
	created, err := svc.CreateSeries(ctx, revisionRequest("os", "A", "/kernel", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "selected"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", created.Record.Metadata.UID+".json")); err != nil {
		t.Fatal(err)
	}
	_, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
	var failure *ReferenceFailure
	if !matched || !errors.As(err, &failure) || failure.AssignmentName != "selected" || failure.ResolvedUID != created.Record.Metadata.UID || !errors.Is(err, ErrRevisionCorrupt) {
		t.Fatalf("lost record diagnosis: matched=%v, failure=%#v, err=%v", matched, failure, err)
	}
}
