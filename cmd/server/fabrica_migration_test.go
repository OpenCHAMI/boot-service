// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openchami/fabrica/pkg/uid"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
)

func serveForTest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/merge-patch+json")
	} else if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestCreateRequiresSpecEnvelope(t *testing.T) {
	registerResourcePrefixesForTest(t)
	router := newGeneratedRouterForTest(t)

	for _, path := range []string{"/bmcs", "/bootconfigurations", "/nodes"} {
		for _, body := range []string{`{"metadata":{"name":"x"}}`, `{"metadata":{"name":"x"},"spec":null}`} {
			if response := serveForTest(router, http.MethodPost, path, body); response.Code != http.StatusBadRequest {
				t.Errorf("POST %s %s returned HTTP %d, want 400: %s", path, body, response.Code, response.Body)
			}
		}
	}

	// An explicitly empty spec is not an absent one, and metadata stays optional.
	if response := serveForTest(router, http.MethodPost, "/bmcs", `{"spec":{}}`); response.Code != http.StatusCreated {
		t.Errorf("POST /bmcs with empty spec returned HTTP %d, want 201: %s", response.Code, response.Body)
	}
}

func TestNewResourcesUseTypeIDs(t *testing.T) {
	registerResourcePrefixesForTest(t)
	router := newGeneratedRouterForTest(t)

	response := serveForTest(router, http.MethodPost, "/nodes", `{"spec":{"xname":"x0c0s0b0n0"}}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("POST /nodes returned HTTP %d: %s", response.Code, response.Body)
	}
	var node NodeResponse
	if err := json.Unmarshal(response.Body.Bytes(), &node); err != nil {
		t.Fatal(err)
	}
	parsed, err := (uid.Codec{}).Parse(node.Metadata.UID)
	if err != nil {
		t.Fatalf("parse %q: %v", node.Metadata.UID, err)
	}
	if parsed.Format != uid.FormatTypeID || parsed.Prefix != "node" {
		t.Fatalf("got format %q prefix %q, want a node TypeID", parsed.Format, parsed.Prefix)
	}
}

// Stores written by earlier releases hold prefix-hex UIDs, which are never
// rewritten and must stay fully usable.
func TestLegacyUIDsRemainUsable(t *testing.T) {
	registerResourcePrefixesForTest(t)
	router := newGeneratedRouterForTest(t)

	const legacyUID = "node-1a2b3c4d"
	legacy := &v1.Node{APIVersion: "boot.openchami.io/v1", Kind: "Node", Spec: v1.NodeSpec{XName: "x0c0s0b0n0"}}
	legacy.Metadata.UID = legacyUID
	if err := storage.SaveNode(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}

	path := "/nodes/" + legacyUID
	if response := serveForTest(router, http.MethodGet, path, ""); response.Code != http.StatusOK {
		t.Fatalf("GET returned HTTP %d: %s", response.Code, response.Body)
	}
	response := serveForTest(router, http.MethodPatch, path, `{"role":"Compute"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("PATCH returned HTTP %d: %s", response.Code, response.Body)
	}
	var patched NodeResponse
	if err := json.Unmarshal(response.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Metadata.UID != legacyUID || patched.Spec.Role != "Compute" {
		t.Fatalf("PATCH returned UID %q role %q", patched.Metadata.UID, patched.Spec.Role)
	}
	if response := serveForTest(router, http.MethodDelete, path, ""); response.Code != http.StatusOK {
		t.Fatalf("DELETE returned HTTP %d: %s", response.Code, response.Body)
	}
	if response := serveForTest(router, http.MethodGet, path, ""); response.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE returned HTTP %d, want 404", response.Code)
	}
}

func TestAPICatalogListsResources(t *testing.T) {
	router := chi.NewRouter()
	RegisterGeneratedPublicRoutes(router)

	response := serveForTest(router, http.MethodGet, "/.well-known/api-catalog", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET returned HTTP %d: %s", response.Code, response.Body)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/linkset+json") {
		t.Fatalf("unexpected Content-Type %q", contentType)
	}
	for _, path := range []string{"/service/status", "/bmcs", "/bootconfigurations", "/nodes"} {
		if !strings.Contains(response.Body.String(), path+`"`) {
			t.Errorf("catalog does not list %s: %s", path, response.Body)
		}
	}

	head := serveForTest(router, http.MethodHead, "/.well-known/api-catalog", "")
	if head.Code != http.StatusOK || head.Header().Get("Link") == "" || head.Body.Len() != 0 {
		t.Fatalf("HEAD returned HTTP %d, Link %q, %d body bytes", head.Code, head.Header().Get("Link"), head.Body.Len())
	}
}
