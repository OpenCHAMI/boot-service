// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http/httptest"
	"testing"
)

func TestGeneratedDeclarativeRoutesAreContained(t *testing.T) {
	issuer := newTestIssuer(t)
	for _, enabled := range []bool{true, false} {
		router := newServerRouterForTest(t, enabled, issuer.jwksURL)
		for _, path := range []string{"/bootconfigs", "/bootconfigs/os/status", "/bootassignments", "/bootassignments/by-name/canary"} {
			for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
				if response.Code != 503 {
					t.Fatalf("%s %s: %d", method, path, response.Code)
				}
			}
		}
	}
}
