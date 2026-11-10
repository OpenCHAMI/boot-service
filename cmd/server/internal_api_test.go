// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openchami/fabrica/pkg/fabrica"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/boot-service/pkg/client"
)

func TestInternalAPIClientServesResourcesInProcess(t *testing.T) {
	registerResourcePrefixesForTest(t)
	if err := storage.InitFileBackend(filepath.Join(t.TempDir(), "data")); err != nil {
		t.Fatal(err)
	}
	internal, err := newInternalAPIClient()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	created, err := internal.CreateBootConfiguration(ctx, client.CreateBootConfigurationRequest{
		Metadata: fabrica.Metadata{Name: "compute"},
		Spec: v1.BootConfigurationSpec{
			MACs:   []string{"aa:bb:cc:dd:ee:ff"},
			Kernel: "https://example.com/vmlinuz",
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := internal.GetBootConfiguration(ctx, created.Metadata.UID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Spec.Kernel != "https://example.com/vmlinuz" {
		t.Fatalf("got kernel %q", got.Spec.Kernel)
	}

	listed, err := internal.GetBootConfigurations(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed %d configurations, want 1", len(listed))
	}

	if err := internal.DeleteBootConfiguration(ctx, created.Metadata.UID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := internal.GetBootConfiguration(ctx, created.Metadata.UID); err == nil {
		t.Fatal("expected an error for a deleted configuration")
	}
}

// Boot handlers make several internal calls from inside one request, each
// inheriting that request's context and its chi routing state.
func TestInternalAPIClientCallsFromInsideARequest(t *testing.T) {
	registerResourcePrefixesForTest(t)
	if err := storage.InitFileBackend(filepath.Join(t.TempDir(), "data")); err != nil {
		t.Fatal(err)
	}
	internal, err := newInternalAPIClient()
	if err != nil {
		t.Fatal(err)
	}

	router := chi.NewRouter()
	router.Get("/outer/{id}", func(w http.ResponseWriter, r *http.Request) {
		for attempt := 1; attempt <= 3; attempt++ {
			if _, err := internal.GetBootConfigurations(r.Context()); err != nil {
				t.Errorf("call %d: %v", attempt, err)
			}
		}
		if pattern := chi.RouteContext(r.Context()).RoutePattern(); pattern != "/outer/{id}" {
			t.Errorf("outer route pattern changed to %q", pattern)
		}
		if id := chi.URLParam(r, "id"); id != "x" {
			t.Errorf("outer URL parameter changed to %q", id)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/outer/x", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("got HTTP %d", response.Code)
	}
}

func TestHandlerTransport(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantBody   string
	}{
		{
			name:       "handler that writes nothing returns 200",
			handler:    func(http.ResponseWriter, *http.Request) {},
			wantStatus: http.StatusOK,
		},
		{
			name: "first status wins",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTeapot)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("short"))
			},
			wantStatus: http.StatusTeapot,
			wantBody:   "short",
		},
		{
			name: "handler always receives a body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Body == nil {
					http.Error(w, "nil body", http.StatusInternalServerError)
				}
			},
			wantStatus: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://internal/", nil)
			req.Body = nil
			response, err := handlerTransport{handler: tc.handler}.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close() //nolint:errcheck
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.wantStatus || string(body) != tc.wantBody {
				t.Fatalf("got %d %q, want %d %q", response.StatusCode, body, tc.wantStatus, tc.wantBody)
			}
		})
	}
}
