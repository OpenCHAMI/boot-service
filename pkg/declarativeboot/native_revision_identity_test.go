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

func TestNativeRevisionIdentityMismatchClassification(t *testing.T) {
	// Given two committed revisions and an assignment that follows B.
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
	caps := Capabilities{Propose: true, Promote: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", caps))
	if err != nil {
		t.Fatal(err)
	}
	ref := revision.Reference{Name: "config", RevisionName: "B"}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "winner"}, Spec: v1.BootAssignmentSpec{ConfigRef: ref}})
	if err != nil {
		t.Fatal(err)
	}
	before, etag, err := svc.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "revisions", "bootconfig", "records")
	data, err := os.ReadFile(filepath.Join(root, a.Record.Metadata.UID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, b.Record.Metadata.UID+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		run  func() error
	}{
		{"reference resolver", func() error { return (NativeReferenceResolver{}).ResolveReference(ctx, ref) }},
		{"list", func() error { _, err := svc.ListRevisions(ctx, "config"); return err }},
		{"get", func() error {
			_, err := svc.GetRevision(ctx, revision.Reference{Name: "config", UID: b.Record.Metadata.UID})
			return err
		}},
		{"resolve", func() error { _, err := svc.ResolveRevision(ctx, ref); return err }},
		{"ensure", func() error {
			_, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", caps))
			return err
		}},
		{"create", func() error { _, err := svc.CreateRevision(ctx, revisionRequest("config", "", "/c", caps)); return err }},
		{"promote", func() error {
			_, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: b.Record.Metadata.UID, IfMatch: etag, Capabilities: caps})
			return err
		}},
		{"snapshot", func() error {
			snapshot, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
			var failure *ReferenceFailure
			if !matched || snapshot.Content.Kernel != "" || !errors.As(err, &failure) || failure.AssignmentName != "winner" || failure.Reference != ref {
				t.Errorf("snapshot=%#v matched=%v error=%v", snapshot, matched, err)
			}
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			// When the service encounters the copied record's wrong UID.
			err := tc.run()
			// Then it reports operational corruption rather than a user error.
			if !errors.Is(err, ErrRevisionCorrupt) || !errors.Is(err, revision.ErrRevisionIdentityMismatch) || errors.Is(err, ErrInvalid) || errors.Is(err, revision.ErrRevisionNotFound) {
				t.Errorf("got %v, want ErrRevisionCorrupt", err)
			}
		})
	}
	for _, condition := range []string{"", `"stale"`} {
		result, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: b.Record.Metadata.UID, IfMatch: condition, Capabilities: caps})
		if !errors.Is(err, revision.ErrPreconditionFailed) || result.ETag != etag {
			t.Errorf("condition=%q result=%#v error=%v", condition, result, err)
		}
	}
	after, afterETag, err := svc.GetSeries(ctx, "config")
	if err != nil || after != before || afterETag != etag {
		t.Fatalf("series changed: %#v %#v %q %v", before, after, afterETag, err)
	}
}
