// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

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
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestDeclarativeDeliveryRejectsFullRecordCopy(t *testing.T) {
	for _, selector := range []string{"default", "latest", "uid"} {
		t.Run(selector, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			if err := storage.InitFileBackend(dir); err != nil {
				t.Fatal(err)
			}
			manager, err := declarativeboot.NewManager(ctx, declarativeboot.GeneratedStore(), declarativeboot.NativeReferenceResolver{})
			if err != nil {
				t.Fatal(err)
			}
			svc := declarativeboot.NewRevisionService(manager)
			proposal := declarativeboot.ProposalRequest{Name: "os", RevisionName: "B", Resource: v1.BootConfig{Spec: v1.BootConfigSpec{Kernel: "/kernel-b"}}, Capabilities: declarativeboot.Capabilities{Propose: true}}
			b, err := svc.CreateSeries(ctx, proposal)
			if err != nil {
				t.Fatal(err)
			}
			proposal.RevisionName, proposal.Resource.Spec.Kernel = "A", "/kernel-a"
			a, err := svc.EnsureRevision(ctx, proposal)
			if err != nil {
				t.Fatal(err)
			}
			_, etag, err := svc.GetSeries(ctx, "os")
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.PromoteDefault(ctx, declarativeboot.PromotionRequest{Name: "os", UID: a.Record.Metadata.UID, IfMatch: etag, Capabilities: declarativeboot.Capabilities{Promote: true}})
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
			_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "winner"}, Spec: v1.BootAssignmentSpec{Selector: &v1.BootAssignmentSelector{Groups: []string{"compute"}}, ConfigRef: ref}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "fallback"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os", RevisionName: "B"}}})
			if err != nil {
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
			legacy := &reviewLegacy{}
			provider := &deliveryProvider{node: &v1.Node{Spec: v1.NodeSpec{XName: "x0c0s0b0n0", Groups: []string{"compute"}}}}
			controller := NewDeclarativeController(DeclarativeOptions{Legacy: legacy, Nodes: provider, Revisions: svc})
			script, err := controller.GenerateBootScript(ctx, "42", "")
			var failure *declarativeboot.ReferenceFailure
			if !errors.As(err, &failure) || !errors.Is(err, declarativeboot.ErrRevisionCorrupt) || failure.AssignmentName != "winner" || failure.Reference != ref || errors.Is(err, declarativeboot.ErrInvalid) || script != "" || legacy.calls != 0 {
				t.Fatalf("script=%q calls=%d corruption-classified=%v err=%v", script, legacy.calls, errors.Is(err, declarativeboot.ErrRevisionCorrupt), err)
			}
		})
	}
}
