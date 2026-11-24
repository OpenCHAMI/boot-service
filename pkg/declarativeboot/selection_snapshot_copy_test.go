// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"bytes"
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

func TestSelectionSnapshotRejectsFullRecordCopy(t *testing.T) {
	for _, selector := range []string{"default", "latest", "uid"} {
		t.Run(selector, func(t *testing.T) {
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
			b, err := svc.CreateSeries(ctx, revisionRequest("os", "B", "/kernel-b", caps))
			if err != nil {
				t.Fatal(err)
			}
			a, err := svc.EnsureRevision(ctx, revisionRequest("os", "A", "/kernel-a", caps))
			if err != nil {
				t.Fatal(err)
			}
			_, etag, err := svc.GetSeries(ctx, "os")
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.PromoteDefault(ctx, PromotionRequest{Name: "os", UID: a.Record.Metadata.UID, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
			if err != nil {
				t.Fatal(err)
			}
			ref := revision.Reference{Name: "os"}
			switch selector {
			case "latest":
				ref.Selector = revision.SelectorLatest
			case "uid":
				ref.UID = a.Record.Metadata.UID
			}
			if _, err := manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "winner"}, Spec: v1.BootAssignmentSpec{ConfigRef: ref}}); err != nil {
				t.Fatal(err)
			}
			records := filepath.Join(dir, "revisions", "bootconfig", "records")
			pathA, pathB := filepath.Join(records, a.Record.Metadata.UID+".json"), filepath.Join(records, b.Record.Metadata.UID+".json")
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
			if err := os.WriteFile(pathA, data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(pathB, data, 0600); err != nil {
				t.Fatal(err)
			}
			copyData, err := os.ReadFile(pathB)
			if err != nil || !bytes.Equal(data, copyData) {
				t.Fatalf("copied fixture: %v", err)
			}
			snapshot, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
			var failure *ReferenceFailure
			if !matched || !errors.As(err, &failure) || !errors.Is(err, ErrRevisionCorrupt) || failure.AssignmentName != "winner" || failure.Reference != ref || errors.Is(err, ErrInvalid) || snapshot.Content.Kernel != "" {
				t.Fatalf("matched=%v snapshot=%#v corruption-classified=%v error=%v", matched, snapshot, errors.Is(err, ErrRevisionCorrupt), err)
			}
		})
	}
}
