// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeclarativeHTTPWireDriver(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	steps := []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/bootconfigs?revisionName=A", `{"metadata":{"name":"wire"},"spec":{"kernel":"/wire-kernel"}}`, 201},
		{"POST", "/bootassignments", `{"metadata":{"name":"wire-node"},"spec":{"configRef":{"name":"wire"}}}`, 201},
		{"GET", "/bootassignments/by-name/wire-node", "", 200},
	}
	for _, step := range steps {
		req, err := http.NewRequest(step.method, server.URL+step.path, strings.NewReader(step.body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+admin)
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("response read=%v close=%v", readErr, closeErr)
		}
		if response.StatusCode != step.status || response.Header.Get("ETag") == "" {
			t.Fatalf("%s %s: HTTP %d etag=%q body=%s", step.method, step.path, response.StatusCode, response.Header.Get("ETag"), body)
		}
	}
}
