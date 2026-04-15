// SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/openchami/boot-service/internal/storage"
	bootclient "github.com/openchami/boot-service/pkg/client"
)

func newGeneratedRouterForTest(t *testing.T) http.Handler {
	t.Helper()

	dataDir := filepath.Join(t.TempDir(), "data")
	if err := storage.InitFileBackend(dataDir); err != nil {
		t.Fatalf("failed to initialize file backend: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RedirectSlashes)
	RegisterGeneratedRoutes(r)

	return r
}

func TestGeneratedRoutesSupportSlashlessCollectionPaths(t *testing.T) {
	server := httptest.NewServer(newGeneratedRouterForTest(t))
	defer server.Close()

	tests := []struct {
		name string
		path string
	}{
		{name: "BMCs", path: "/bmcs"},
		{name: "BootConfigurations", path: "/bootconfigurations"},
		{name: "Nodes", path: "/nodes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s failed: %v", tc.path, err)
			}
			defer resp.Body.Close() //nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s returned status %d, want %d", tc.path, resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestGeneratedClientWorksAgainstSlashlessCollectionPaths(t *testing.T) {
	server := httptest.NewServer(newGeneratedRouterForTest(t))
	defer server.Close()

	client, err := bootclient.NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	if _, err := client.GetBMCs(ctx); err != nil {
		t.Fatalf("GetBMCs failed against slashless path: %v", err)
	}

	if _, err := client.GetBootConfigurations(ctx); err != nil {
		t.Fatalf("GetBootConfigurations failed against slashless path: %v", err)
	}

	if _, err := client.GetNodes(ctx); err != nil {
		t.Fatalf("GetNodes failed against slashless path: %v", err)
	}
}
