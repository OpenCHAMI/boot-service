// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDeclarativeHTTPStatusPreservesSpecAndConditions(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	reader := declarativeTestToken(t, issuer, "read:boot")
	config := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel"}}`
	if response := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", admin, config, "", ""); response.Code != 201 {
		t.Fatalf("series: %d", response.Code)
	}
	assignment := `{"metadata":{"name":"canary"},"spec":{"priority":3,"configRef":{"name":"os"}}}`
	created := declarativeRequest(router, "POST", "/bootassignments", admin, assignment, "", "")
	if created.Code != 201 {
		t.Fatalf("assignment: %d", created.Code)
	}
	var value struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &value); err != nil || value.Metadata.UID == "" {
		t.Fatalf("decode: %v", err)
	}
	path := "/bootassignments/" + value.Metadata.UID + "/status"
	old := created.Header().Get("ETag")
	if response := declarativeRequest(router, "PUT", path, reader, `{"phase":"ready"}`, "If-Match", old); response.Code != 403 {
		t.Fatalf("reader status: %d", response.Code)
	}
	if response := declarativeRequest(router, "PUT", path, admin, `{"phase":"ready"}`, "", ""); response.Code != 412 {
		t.Fatalf("missing condition: %d", response.Code)
	}
	updated := declarativeRequest(router, "PUT", path, admin, `{"phase":"ready"}`, "If-Match", old)
	if updated.Code != 200 || updated.Header().Get("ETag") == old {
		t.Fatalf("update: %d %s", updated.Code, updated.Body)
	}
	if response := declarativeRequest(router, "PATCH", path, admin, `{"error":"offline"}`, "If-Match", old); response.Code != 412 {
		t.Fatalf("stale patch: %d", response.Code)
	}
	patched := declarativeRequest(router, "PATCH", path, admin, `{"error":"offline"}`, "If-Match", updated.Header().Get("ETag"))
	if patched.Code != 200 {
		t.Fatalf("patch: %d %s", patched.Code, patched.Body)
	}
	if response := declarativeRequest(router, "PUT", path, admin, `{"spec":{"configRef":{"name":"missing"}}}`, "If-Match", patched.Header().Get("ETag")); response.Code != 400 {
		t.Fatalf("status smuggling: %d", response.Code)
	}
	current := declarativeRequest(router, "GET", "/bootassignments/"+value.Metadata.UID, admin, "", "", "")
	var resource struct {
		Spec struct {
			Priority int `json:"priority"`
		} `json:"spec"`
		Status struct {
			Phase string `json:"phase"`
			Error string `json:"error"`
		} `json:"status"`
	}
	if err := json.Unmarshal(current.Body.Bytes(), &resource); err != nil || resource.Spec.Priority != 3 || resource.Status.Phase != "ready" || resource.Status.Error != "offline" {
		t.Fatalf("status/spec: %#v error=%v", resource, err)
	}
}

func TestDeclarativeHTTPSeriesStatusDoesNotMovePointers(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	created := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", admin, `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel"}}`, "", "")
	if created.Code != http.StatusCreated {
		t.Fatalf("series: %d", created.Code)
	}
	before := declarativeRequest(router, "GET", "/bootconfigs/os", admin, "", "", "")
	response := declarativeRequest(router, "PUT", "/bootconfigs/os/status", admin, `{"phase":"ready"}`, "If-Match", before.Header().Get("ETag"))
	if response.Code != http.StatusOK {
		t.Fatalf("status: %d %s", response.Code, response.Body)
	}
	after := declarativeRequest(router, "GET", "/bootconfigs/os", admin, "", "", "")
	var prior, current struct {
		Default string `json:"defaultRevisionUid"`
		Latest  string `json:"latestRevisionUid"`
	}
	if json.Unmarshal(before.Body.Bytes(), &prior) != nil || json.Unmarshal(after.Body.Bytes(), &current) != nil || prior != current || prior.Default == "" {
		t.Fatalf("pointers changed: %#v %#v", prior, current)
	}
}
