// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/clients/hsm"
	"github.com/openchami/boot-service/pkg/controllers/bootscript"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/boot-service/pkg/handlers/boot"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

type legacyDelivery struct{}

func (legacyDelivery) GenerateBootScript(context.Context, string, string) (string, error) {
	return "legacy-exact", nil
}

func TestDeclarativeDeliveryRealRouterAndSMD(t *testing.T) {
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
	proposal := func(rev, kernel string) declarativeboot.ProposalRequest {
		return declarativeboot.ProposalRequest{Name: "os", RevisionName: rev, Resource: v1.BootConfig{Spec: v1.BootConfigSpec{Kernel: kernel, Initrd: "/initrd-" + rev, Params: "chosen=" + rev}}, Capabilities: declarativeboot.Capabilities{Propose: true}}
	}
	a, err := svc.CreateSeries(ctx, proposal("A", "/kernel-A"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, proposal("B", "/kernel-B"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "group"}, Spec: v1.BootAssignmentSpec{Selector: &v1.BootAssignmentSelector{Groups: []string{"compute"}}, ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "pin"}, Spec: v1.BootAssignmentSpec{Selector: &v1.BootAssignmentSelector{XNames: []string{"x0c0s0b0n1"}}, ConfigRef: revision.Reference{Name: "os", UID: a.Record.Metadata.UID}}})
	if err != nil {
		t.Fatal(err)
	}
	smd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hsm/v2/State/Components":
			_, _ = io.WriteString(w, `{"Components":[{"ID":"x0c0s0b0n0","Type":"Node","NID":10},{"ID":"x0c0s0b0n1","Type":"Node","NID":11},{"ID":"x0c0s0b0n2","Type":"Node","NID":12}]}`)
		case "/hsm/v2/Inventory/EthernetInterfaces":
			_, _ = io.WriteString(w, `[{"ComponentID":"x0c0s0b0n0","MACAddress":"02:00:00:00:00:10"},{"ComponentID":"x0c0s0b0n1","MACAddress":"02:00:00:00:00:11"},{"ComponentID":"x0c0s0b0n2","MACAddress":"02:00:00:00:00:12"}]`)
		case "/hsm/v2/memberships":
			if r.URL.Query().Get("type") != "Node" {
				http.Error(w, "bad type", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `[{"id":"x0c0s0b0n0","groupLabels":["compute"]},{"id":"x0c0s0b0n1","groupLabels":[]},{"id":"x0c0s0b0n2","groupLabels":[]}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(smd.Close)
	config := hsm.DefaultHSMConfig()
	config.BaseURL = smd.URL
	hsmClient, err := hsm.NewHSMClient(config, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	controller := bootscript.NewDeclarativeController(bootscript.DeclarativeOptions{Legacy: legacyDelivery{}, Nodes: hsm.NewDeclarativeNodeProvider(hsmClient), Revisions: svc})
	handler := boot.NewHandlerWithController(client.Client{}, controller, log.New(io.Discard, "", 0))
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(bootscript.WithRequestProfile(r.Context(), r.URL.Query().Get("profile"))))
		})
	})
	handler.RegisterModernPublicRoutes(router)
	request := func(mac string) string {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bootscript?mac="+mac, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	assertArtifact := func(mac, present, absent string) {
		t.Helper()
		body := request(mac)
		if !strings.Contains(body, present) || strings.Contains(body, absent) {
			t.Fatalf("incorrect artifact %q", body)
		}
	}
	assertArtifact("02:00:00:00:00:10", "/kernel-A", "/kernel-B")
	_, etag, err := svc.GetSeries(ctx, "os")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.PromoteDefault(ctx, declarativeboot.PromotionRequest{Name: "os", UID: b.Record.Metadata.UID, IfMatch: etag, Capabilities: declarativeboot.Capabilities{Promote: true}})
	if err != nil {
		t.Fatal(err)
	}
	assertArtifact("02:00:00:00:00:10", "/kernel-B", "/kernel-A")
	assertArtifact("02:00:00:00:00:11", "/kernel-A", "/kernel-B")
	if got := request("02:00:00:00:00:12"); got != "legacy-exact" {
		t.Fatalf("legacy output %q", got)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", b.Record.Metadata.UID+".json")); err != nil {
		t.Fatal(err)
	}
	_, err = controller.GenerateBootScript(ctx, "02:00:00:00:00:10", "")
	var failure *declarativeboot.ReferenceFailure
	if !errors.As(err, &failure) {
		t.Fatalf("expected broken winner failure: %v", err)
	}
}
