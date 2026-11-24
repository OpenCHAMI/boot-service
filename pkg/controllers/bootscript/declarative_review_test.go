// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/boot-service/pkg/clients/hsm"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

type reviewLegacy struct{ calls int }

func (l *reviewLegacy) GenerateBootScript(context.Context, string, string) (string, error) {
	l.calls++
	return "legacy", nil
}

func TestDeclarativeReviewRefusesIncompleteMembershipAndCorruptName(t *testing.T) {
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
	a, err := svc.CreateSeries(ctx, declarativeboot.ProposalRequest{Name: "os", RevisionName: "A", Resource: v1.BootConfig{Spec: v1.BootConfigSpec{Kernel: "/kernel-a"}}, Capabilities: declarativeboot.Capabilities{Propose: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "group"}, Spec: v1.BootAssignmentSpec{Selector: &v1.BootAssignmentSelector{Groups: []string{"compute"}}, ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, membership  string
		refusal, selected bool
	}{
		{"missing", `{"id":"x0c0s0b0n0"}`, true, false},
		{"null", `{"id":"x0c0s0b0n0","groupLabels":null}`, true, false},
		{"empty", `{"id":"x0c0s0b0n0","groupLabels":[]}`, false, false},
		{"selected", `{"id":"x0c0s0b0n0","groupLabels":["compute"]}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture-token" {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/hsm/v2/State/Components":
					_, _ = io.WriteString(w, `{"Components":[{"ID":"x0c0s0b0n0","Type":"Node","NID":42}]}`)
				case "/hsm/v2/Inventory/EthernetInterfaces":
					_, _ = io.WriteString(w, `[{"ComponentID":"x0c0s0b0n0","MACAddress":"aa:bb:cc:dd:ee:ff"}]`)
				case "/hsm/v2/memberships":
					_, _ = io.WriteString(w, "["+tc.membership+"]")
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			config := hsm.DefaultHSMConfig()
			config.BaseURL, config.AuthToken = server.URL, "fixture-token"
			hsmClient, err := hsm.NewHSMClient(config, log.New(io.Discard, "", 0))
			if err != nil {
				t.Fatal(err)
			}
			legacy := &reviewLegacy{}
			controller := NewDeclarativeController(DeclarativeOptions{Legacy: legacy, Nodes: hsm.NewDeclarativeNodeProvider(hsmClient), Revisions: svc})
			script, err := controller.GenerateBootScript(ctx, "42", "")
			if tc.refusal {
				var failure *DeliveryFailure
				if !errors.As(err, &failure) || script != "" || legacy.calls != 0 {
					t.Fatalf("refusal: script=%q calls=%d err=%v", script, legacy.calls, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.selected && (!strings.Contains(script, "/kernel-a") || legacy.calls != 0) {
				t.Fatalf("selection: %q calls=%d", script, legacy.calls)
			}
			if !tc.selected && (script != "legacy" || legacy.calls != 1) {
				t.Fatalf("fallback: %q calls=%d", script, legacy.calls)
			}
		})
	}
	path := filepath.Join(dir, "revisions", "bootconfig", "records", a.Record.Metadata.UID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record revision.Record[*v1.BootConfig]
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	record.Metadata.RevisionName = "invalid\nboot"
	data, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	legacy := &reviewLegacy{}
	provider := &deliveryProvider{node: &v1.Node{Spec: v1.NodeSpec{XName: "x0c0s0b0n0", Groups: []string{"compute"}}}}
	controller := NewDeclarativeController(DeclarativeOptions{Legacy: legacy, Nodes: provider, Revisions: svc})
	script, err := controller.GenerateBootScript(ctx, "42", "")
	var failure *declarativeboot.ReferenceFailure
	if !errors.As(err, &failure) || !errors.Is(err, declarativeboot.ErrRevisionCorrupt) || script != "" || legacy.calls != 0 {
		t.Fatalf("corrupt winner: script=%q calls=%d err=%v", script, legacy.calls, err)
	}
}

func TestDeclarativeReviewMisboundWinnerDoesNotFallThrough(t *testing.T) {
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
	proposal := declarativeboot.ProposalRequest{Name: "os", RevisionName: "A", Resource: v1.BootConfig{Spec: v1.BootConfigSpec{Kernel: "/kernel-a"}}, Capabilities: declarativeboot.Capabilities{Propose: true}}
	a, err := svc.CreateSeries(ctx, proposal)
	if err != nil {
		t.Fatal(err)
	}
	proposal.RevisionName, proposal.Resource.Spec.Kernel = "B", "/kernel-b"
	if _, err := svc.EnsureRevision(ctx, proposal); err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "winner"}, Spec: v1.BootAssignmentSpec{Selector: &v1.BootAssignmentSelector{Groups: []string{"compute"}}, ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "fallback"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os", RevisionName: "B"}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "revisions", "bootconfig", "records", a.Record.Metadata.UID+".json")
	data, err := os.ReadFile(path)
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
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	legacy := &reviewLegacy{}
	provider := &deliveryProvider{node: &v1.Node{Spec: v1.NodeSpec{XName: "x0c0s0b0n0", Groups: []string{"compute"}}}}
	controller := NewDeclarativeController(DeclarativeOptions{Legacy: legacy, Nodes: provider, Revisions: svc})
	script, err := controller.GenerateBootScript(ctx, "42", "")
	var failure *declarativeboot.ReferenceFailure
	if !errors.As(err, &failure) || !errors.Is(err, declarativeboot.ErrRevisionCorrupt) || script != "" || legacy.calls != 0 {
		t.Fatalf("misbound winner: script=%q calls=%d err=%v", script, legacy.calls, err)
	}
}
