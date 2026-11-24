// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"
	"testing"
)

func TestDeclarativeHTTPUnavailableWhenAuthenticationDisabled(t *testing.T) {
	router := newServerRouterForTest(t, false, "")
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/bootassignments"},
		{http.MethodPost, "/bootassignments/"},
		{http.MethodPost, "/bootconfigs"},
		{http.MethodPut, "/bootconfigs/config/revisions/A"},
		{http.MethodDelete, "/bootconfigs/config/revisions/by-uid/rev_x"},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := requestForTest(router, tc.method, tc.path, "not-json", "")
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("got HTTP %d, want 503", response.Code)
			}
		})
	}
}
