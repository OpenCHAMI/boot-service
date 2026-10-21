// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBootConfigurationConditionalRequests(t *testing.T) {
	if err := registerResourcePrefixes(); err != nil {
		t.Fatal(err)
	}
	router := newGeneratedRouterForTest(t)
	request := func(method, path, body, header, value string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if header != "" {
			req.Header.Set(header, value)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	checkStatus := func(response *httptest.ResponseRecorder, want int) {
		t.Helper()
		if response.Code != want {
			t.Fatalf("got HTTP %d, want %d: %s", response.Code, want, response.Body.String())
		}
	}

	created := request(http.MethodPost, "/bootconfigurations", `{"metadata":{"name":"compute"},"spec":{"kernel":"https://example.com/vmlinuz-v1"}}`, "", "")
	checkStatus(created, http.StatusCreated)
	var config BootConfigurationResponse
	if err := json.Unmarshal(created.Body.Bytes(), &config); err != nil {
		t.Fatal(err)
	}
	path := "/bootconfigurations/" + config.Metadata.UID
	initial := request(http.MethodGet, path, "", "", "")
	checkStatus(initial, http.StatusOK)
	etag := initial.Header().Get("ETag")
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Fatalf("expected a strong ETag, got %q", etag)
	}

	cached := request(http.MethodGet, path, "", "If-None-Match", etag)
	checkStatus(cached, http.StatusNotModified)
	if cached.Body.Len() != 0 {
		t.Fatal("conditional GET should have an empty response body")
	}

	update := `{"spec":{"kernel":"https://example.com/vmlinuz-v2"}}`
	checkStatus(request(http.MethodPut, path, update, "If-Match", etag), http.StatusOK)
	current := request(http.MethodGet, path, "", "", "")
	checkStatus(current, http.StatusOK)
	if current.Header().Get("ETag") == etag {
		t.Fatal("ETag must change after updating the kernel")
	}

	for _, tc := range []struct {
		method string
		body   string
	}{
		{http.MethodPut, `{"spec":{"kernel":"https://example.com/stale"}}`},
		{http.MethodPatch, `{"kernel":"https://example.com/stale"}`},
		{http.MethodDelete, ""},
	} {
		checkStatus(request(tc.method, path, tc.body, "If-Match", etag), http.StatusPreconditionFailed)
	}
	unchanged := request(http.MethodGet, path, "", "", "")
	checkStatus(unchanged, http.StatusOK)
	if unchanged.Body.String() != current.Body.String() {
		t.Fatal("requests with a stale ETag must leave the resource unchanged")
	}

	// Existing clients can still update without sending conditional headers.
	checkStatus(request(http.MethodPut, path, update, "", ""), http.StatusOK)
}
