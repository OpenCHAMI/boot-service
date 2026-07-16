// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package boot

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	apiv1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/fabrica/pkg/resource"
)

func TestGetBootScript_ProfileQueryParameterIgnored(t *testing.T) {
	nodes := []apiv1.Node{
		{
			Spec: apiv1.NodeSpec{
				XName:    "x0c0s0b0n0",
				NID:      42,
				BootMAC:  "aa:bb:cc:dd:ee:ff",
				Hostname: "node-42",
			},
		},
	}

	configs := []apiv1.BootConfiguration{
		{
			Metadata: resource.Metadata{Name: "default-config"},
			Spec: apiv1.BootConfigurationSpec{
				Profile: "",
				Kernel:  "http://files.example.com/vmlinuz-default",
				Params:  "console=ttyS0,115200 profile=default",
			},
		},
		{
			Metadata: resource.Metadata{Name: "compute-config"},
			Spec: apiv1.BootConfigurationSpec{
				Profile: "compute",
				Kernel:  "http://files.example.com/vmlinuz-compute",
				Params:  "console=ttyS0,115200 profile=compute",
			},
		},
	}

	// Create backend API server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes":
			writeJSONResponse(t, w, nodes)
		case "/bootconfigurations":
			writeJSONResponse(t, w, configs)
		default:
			http.NotFound(w, r)
		}
	}))
	defer backendServer.Close()

	// Create boot client pointing to backend
	bootClient, err := client.NewClient(backendServer.URL, backendServer.Client(), client.DefaultLogger())
	if err != nil {
		t.Fatalf("failed to create boot client: %v", err)
	}

	// Create boot handler with bootscript controller
	handler := NewHandler(*bootClient, log.New(io.Discard, "", 0))

	// Create router and register modern routes
	router := chi.NewRouter()
	handler.RegisterModernRoutes(router)

	// Test Case 1: Request with explicit compute profile query should still
	// auto-select based on score/priority and ignore the query parameter.
	req := httptest.NewRequest("GET", "/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "profile=compute") {
		t.Errorf("expected compute profile in response, got: %s", body)
	}
	if !strings.Contains(body, "#!ipxe") {
		t.Errorf("expected iPXE header in response")
	}

	// Test Case 2: Request with empty profile parameter (auto-select best across profiles)
	req = httptest.NewRequest("GET", "/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body = w.Body.String()
	if !strings.Contains(body, "profile=compute") {
		t.Errorf("expected best matching profile in response, got: %s", body)
	}

	// Test Case 3: Request without profile parameter (auto-select best across profiles)
	req = httptest.NewRequest("GET", "/bootscript?mac=aa:bb:cc:dd:ee:ff", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body = w.Body.String()
	if !strings.Contains(body, "profile=compute") {
		t.Errorf("expected best matching profile when no profile param provided, got: %s", body)
	}

	// Test Case 4: Request with XName identifier
	req = httptest.NewRequest("GET", "/bootscript?host=x0c0s0b0n0&profile=compute", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body = w.Body.String()
	if !strings.Contains(body, "profile=compute") {
		t.Errorf("expected best matching profile with XName identifier, got: %s", body)
	}

	// Test Case 5: Request with profile=default should still ignore profile and
	// auto-select compute due to higher match score.
	req = httptest.NewRequest("GET", "/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=default", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body = w.Body.String()
	if !strings.Contains(body, "profile=compute") {
		t.Errorf("expected best matching profile when profile query is ignored, got: %s", body)
	}
}

func TestGetBootScript_MissingNodeIdentifier(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes":
			writeJSONResponse(t, w, []apiv1.Node{})
		case "/bootconfigurations":
			writeJSONResponse(t, w, []apiv1.BootConfiguration{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backendServer.Close()

	bootClient, err := client.NewClient(backendServer.URL, backendServer.Client(), client.DefaultLogger())
	if err != nil {
		t.Fatalf("failed to create boot client: %v", err)
	}

	handler := NewHandler(*bootClient, log.New(io.Discard, "", 0))
	router := chi.NewRouter()
	handler.RegisterModernRoutes(router)

	// Request without any node identifier
	req := httptest.NewRequest("GET", "/bootscript", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing identifier, got %d", w.Code)
	}
}

func TestRegisterModernAndLegacyRoutes_Separately(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes":
			writeJSONResponse(t, w, []apiv1.Node{})
		case "/bootconfigurations":
			writeJSONResponse(t, w, []apiv1.BootConfiguration{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backendServer.Close()

	bootClient, err := client.NewClient(backendServer.URL, backendServer.Client(), client.DefaultLogger())
	if err != nil {
		t.Fatalf("failed to create boot client: %v", err)
	}

	handler := NewHandler(*bootClient, log.New(io.Discard, "", 0))

	// Test 1: Only modern routes registered
	router1 := chi.NewRouter()
	handler.RegisterModernRoutes(router1)

	// Modern bootscript endpoint should be present
	req := httptest.NewRequest("GET", "/bootscript?mac=aa:bb:cc:dd:ee:ff", nil)
	w := httptest.NewRecorder()
	router1.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for modern bootscript route, got %d", w.Code)
	}

	// Legacy endpoint should not be present
	req = httptest.NewRequest("GET", "/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff", nil)
	w = httptest.NewRecorder()
	router1.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for legacy route when not registered, got %d", w.Code)
	}

	// Test 2: Both modern and legacy routes registered
	router2 := chi.NewRouter()
	handler.RegisterModernRoutes(router2)
	handler.RegisterLegacyRoutes(router2)

	// Modern bootscript endpoint should work
	req = httptest.NewRequest("GET", "/bootscript?mac=aa:bb:cc:dd:ee:ff", nil)
	w = httptest.NewRecorder()
	router2.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for modern bootscript route, got %d", w.Code)
	}

	// Legacy bootscript endpoint should also work
	req = httptest.NewRequest("GET", "/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff", nil)
	w = httptest.NewRecorder()
	router2.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for legacy bootscript route, got %d", w.Code)
	}

	// Modern bootparameters endpoint should work
	req = httptest.NewRequest("GET", "/bootparameters", nil)
	w = httptest.NewRecorder()
	router2.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for modern bootparameters route, got %d", w.Code)
	}

	// Legacy bootparameters endpoint should also work
	req = httptest.NewRequest("GET", "/boot/v1/bootparameters", nil)
	w = httptest.NewRecorder()
	router2.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for legacy bootparameters route, got %d", w.Code)
	}
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, data interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		t.Fatalf("failed to encode JSON response: %v", err)
	}
}
