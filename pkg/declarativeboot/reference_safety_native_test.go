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

func TestReferenceIntegrityNativeMissingRecordRemainsOperational(t *testing.T) {
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
	created, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	ref := revision.Reference{Name: "config", UID: created.Record.Metadata.UID}
	pin, err := manager.Create(ctx, v1.BootAssignment{
		Metadata: resource.Metadata{Name: "winner"},
		Spec:     v1.BootAssignmentSpec{ConfigRef: ref},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", ref.UID+".json")); err != nil {
		t.Fatal(err)
	}
	restored, err := NewManager(ctx, GeneratedStore(), NativeReferenceResolver{})
	if err != nil {
		t.Fatalf("missing reference must not prevent startup: %v", err)
	}
	winner, err := restored.GetByUID(ctx, pin.Assignment.Metadata.UID)
	if err != nil {
		t.Fatal(err)
	}
	cause := (NativeReferenceResolver{}).ResolveReference(ctx, winner.Assignment.Spec.ConfigRef)
	if !errors.Is(cause, ErrRevisionCorrupt) || errors.Is(cause, ErrInvalid) {
		t.Fatalf("corruption was misclassified: %v", cause)
	}
	err = WrapSelectedReferenceFailure(winner.Assignment, cause)
	var failure *ReferenceFailure
	if !errors.As(err, &failure) || !errors.Is(err, ErrRevisionCorrupt) || errors.Is(err, ErrInvalid) || failure.Reference != ref {
		t.Fatalf("selected failure lost corruption identity: %v", err)
	}
}
