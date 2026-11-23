// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func declarativeRequest(router http.Handler, method, path, token, body, condition, value string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if condition != "" {
		req.Header.Set(condition, value)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/merge-patch+json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestDeclarativeHTTPNamedConditionalWrites(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	config := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-a"}}`
	if response := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", admin, config, "", ""); response.Code != 201 {
		t.Fatalf("create series: %d %s", response.Code, response.Body)
	}
	body := `{"metadata":{"name":"canary"},"spec":{"configRef":{"name":"os"}}}`
	path := "/bootassignments/by-name/canary"
	if response := declarativeRequest(router, "PUT", path, admin, body, "", ""); response.Code != 412 {
		t.Fatalf("missing create condition: %d", response.Code)
	}
	created := declarativeRequest(router, "PUT", path, admin, body, "If-None-Match", "*")
	if created.Code != 201 || created.Header().Get("ETag") == "" {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	etag := created.Header().Get("ETag")
	notModified := declarativeRequest(router, "GET", path, admin, "", "If-None-Match", etag)
	if notModified.Code != http.StatusNotModified {
		t.Fatalf("conditional read: %d", notModified.Code)
	}
	var resource struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &resource); err != nil || resource.Metadata.UID == "" {
		t.Fatalf("decode assignment: %v", err)
	}
	if response := declarativeRequest(router, "PUT", path, admin, body, "If-None-Match", "*"); response.Code != 412 {
		t.Fatalf("competing create: %d", response.Code)
	}
	replay := declarativeRequest(router, "PUT", path, admin, body, "If-Match", etag)
	if replay.Code != 200 || replay.Header().Get("ETag") != etag {
		t.Fatalf("replay: %d %q", replay.Code, replay.Header().Get("ETag"))
	}
	uidPath := "/bootassignments/" + resource.Metadata.UID
	rename := `{"metadata":{"name":"other"},"spec":{"configRef":{"name":"os"}}}`
	if response := declarativeRequest(router, "PUT", uidPath, admin, rename, "If-Match", etag); response.Code != 409 {
		t.Fatalf("rename: %d", response.Code)
	}
	changed := declarativeRequest(router, "PATCH", uidPath, admin, `{"priority":7}`, "If-Match", etag)
	if changed.Code != 200 || changed.Header().Get("ETag") == etag {
		t.Fatalf("patch: %d %s", changed.Code, changed.Body)
	}
	if response := declarativeRequest(router, "DELETE", path, admin, "", "If-Match", etag); response.Code != 412 {
		t.Fatalf("stale delete: %d", response.Code)
	}
	if response := declarativeRequest(router, "DELETE", path, admin, "", "If-Match", changed.Header().Get("ETag")); response.Code != 204 {
		t.Fatalf("delete: %d", response.Code)
	}
}

func TestDeclarativeHTTPRejectsUnknownFieldsAndLatestBypass(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	proposer := declarativeTestToken(t, issuer, "propose:boot")
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	config := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-a"}}`
	if response := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", proposer, config, "", ""); response.Code != 201 {
		t.Fatalf("series: %d %s", response.Code, response.Body)
	}
	invalid := `{"metadata":{"name":"bad"},"spec":{"configRef":{"name":"os"},"defaultRevisionUid":"fake"}}`
	if response := declarativeRequest(router, "POST", "/bootassignments", admin, invalid, "", ""); response.Code != 400 {
		t.Fatalf("unknown field: %d", response.Code)
	}
	missing := `{"metadata":{"name":"bad"},"spec":{"configRef":{"name":"missing"}}}`
	if response := declarativeRequest(router, "POST", "/bootassignments", admin, missing, "", ""); response.Code != 422 {
		t.Fatalf("missing reference: %d", response.Code)
	}
	assignment := `{"metadata":{"name":"latest"},"spec":{"configRef":{"name":"os","selector":"latest"}}}`
	if response := declarativeRequest(router, "POST", "/bootassignments", admin, assignment, "", ""); response.Code != 201 {
		t.Fatalf("latest assignment: %d %s", response.Code, response.Body)
	}
	newConfig := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-b"}}`
	for _, tc := range []struct{ method, path string }{{"PUT", "/bootconfigs/os/revisions/B"}, {"POST", "/bootconfigs/os/revisions"}} {
		response := declarativeRequest(router, tc.method, tc.path, proposer, newConfig, "", "")
		if response.Code != http.StatusForbidden {
			t.Errorf("%s %s: %d", tc.method, tc.path, response.Code)
		}
	}
}
