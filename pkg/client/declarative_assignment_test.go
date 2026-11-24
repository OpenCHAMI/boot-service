// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNamedAssignmentApplySendsCondition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/bootassignments/by-name/canary" || r.Header.Get("If-None-Match") != "*" {
			t.Errorf("unexpected request: %s %s condition=%q", r.Method, r.URL.Path, r.Header.Get("If-None-Match"))
		}
		w.Header().Set("ETag", `"new"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"metadata":{"name":"canary"},"spec":{"configRef":{"name":"os"}}}`))
	}))
	defer server.Close()
	c, err := NewClient(server.URL, server.Client(), DefaultLogger())
	if err != nil {
		t.Fatal(err)
	}
	result, etag, err := c.ApplyBootAssignmentByName(context.Background(), "canary", CreateBootAssignmentRequest{}, NamedAssignmentCondition{IfNoneMatch: "*"})
	if err != nil || etag != `"new"` || result.Metadata.Name != "canary" {
		t.Fatalf("result=%v etag=%q err=%v", result, etag, err)
	}
}
