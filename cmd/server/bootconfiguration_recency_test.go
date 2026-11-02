// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	apiv1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	bootclient "github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/handlers/boot"
)

// Fabrica v0.4.13's process-wide prefix registration rejects duplicate calls.
var registerTestResourcePrefixes = sync.OnceValue(registerResourcePrefixes)

func TestBootConfigurationRecencyThroughAPI(t *testing.T) {
	if err := registerTestResourcePrefixes(); err != nil {
		t.Fatal(err)
	}
	// Exercise generated CRUD handlers, file persistence, the production client,
	// and both bootscript routes with one long-lived controller and warm cache.
	backend := httptest.NewServer(newGeneratedRouterForTest(t))
	t.Cleanup(backend.Close)
	logger := log.New(io.Discard, "", 0)
	client, err := bootclient.NewClient(backend.URL, backend.Client(), bootclient.DefaultLogger())
	if err != nil {
		t.Fatal(err)
	}
	handler := boot.NewHandler(*client, logger)
	router := chi.NewRouter()
	handler.RegisterModernRoutes(router)
	handler.RegisterLegacyRoutes(router)
	frontend := httptest.NewServer(router)
	t.Cleanup(frontend.Close)
	ctx := context.Background()
	node := bootclient.CreateNodeRequest{Spec: apiv1.NodeSpec{
		XName: "x0c0s0b0n0", BootMAC: "02:00:00:00:00:01", NID: 1,
	}}
	node.Metadata.Name = node.Spec.XName
	if _, err := client.CreateNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	spec := func(image string) apiv1.BootConfigurationSpec {
		return apiv1.BootConfigurationSpec{
			MACs:   []string{node.Spec.BootMAC},
			Kernel: "http://example.invalid/" + image,
			Initrd: "http://example.invalid/" + image + "-initrd",
			Params: "image=" + image,
		}
	}
	assertScript := func(expected apiv1.BootConfigurationSpec) {
		t.Helper()
		for _, path := range []string{"/bootscript", "/boot/v1/bootscript"} {
			response, err := frontend.Client().Get(frontend.URL + path + "?mac=" + node.Spec.BootMAC)
			if err != nil {
				t.Fatal(err)
			}
			body, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status=%d body=%s", path, response.StatusCode, body)
			}
			for _, value := range []string{expected.Kernel, expected.Initrd, expected.Params} {
				if !strings.Contains(string(body), value) {
					t.Fatalf("GET %s missing %q: %s", path, value, body)
				}
			}
		}
	}
	// The name "default" also exercises the old profile/cache-key collision.
	debug, err := client.CreateBootConfigurationSimple(ctx, "default", spec("debug"))
	if err != nil {
		t.Fatal(err)
	}
	if debug.Metadata.CreatedAt.IsZero() || debug.Metadata.UpdatedAt.IsZero() {
		t.Fatal("create must assign server timestamps")
	}
	assertScript(debug.Spec)
	assertScript(debug.Spec)
	base, err := client.CreateBootConfigurationSimple(ctx, "z-base", spec("base"))
	if err != nil {
		t.Fatal(err)
	}
	if !base.Metadata.UpdatedAt.After(debug.Metadata.UpdatedAt) {
		t.Fatal("new configuration should have a later server timestamp")
	}
	assertScript(base.Spec)
	updated, err := client.UpdateBootConfigurationSimple(ctx, debug.Metadata.UID, spec("debug-updated"))
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Metadata.UpdatedAt.After(base.Metadata.UpdatedAt) ||
		!updated.Metadata.CreatedAt.Equal(debug.Metadata.CreatedAt) {
		t.Fatal("PUT must advance updatedAt while retaining createdAt")
	}
	assertScript(updated.Spec)
	patched, err := client.PatchBootConfiguration(ctx, base.Metadata.UID,
		[]byte(`{
  "kernel": "http://example.invalid/base-patched",
  "initrd": "http://example.invalid/base-patched-initrd",
  "params": "image=base-patched"
}`),
		"application/merge-patch+json")
	if err != nil {
		t.Fatal(err)
	}
	if !patched.Metadata.UpdatedAt.After(updated.Metadata.UpdatedAt) ||
		!patched.Metadata.CreatedAt.Equal(base.Metadata.CreatedAt) {
		t.Fatal("PATCH must advance updatedAt while retaining createdAt")
	}
	assertScript(patched.Spec)
	// A status write is also a resource write in the current API contract.
	statusUpdated, err := client.UpdateBootConfigurationStatus(ctx, debug.Metadata.UID,
		apiv1.BootConfigurationStatus{Phase: "Active"})
	if err != nil {
		t.Fatal(err)
	}
	if !statusUpdated.Metadata.UpdatedAt.After(patched.Metadata.UpdatedAt) {
		t.Fatal("status write must advance updatedAt")
	}
	assertScript(statusUpdated.Spec)
	// Updating the same winner must also invalidate its previous rendered content.
	refreshed, err := client.UpdateBootConfigurationSimple(ctx, debug.Metadata.UID, spec("debug-refreshed"))
	if err != nil {
		t.Fatal(err)
	}
	assertScript(refreshed.Spec)
	prioritizedSpec := refreshed.Spec
	prioritizedSpec.Priority = 1
	prioritized, err := client.UpdateBootConfigurationSimple(ctx, debug.Metadata.UID, prioritizedSpec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateBootConfigurationSimple(ctx, base.Metadata.UID, spec("base-newest")); err != nil {
		t.Fatal(err)
	}
	assertScript(prioritized.Spec)
}
