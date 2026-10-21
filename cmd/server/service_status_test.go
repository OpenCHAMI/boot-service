// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceStatusRoutes(t *testing.T) {
	// Exercise the generated route alongside the boot route registrations so
	// an accidentally reintroduced boot-owned status route cannot mask it.
	for _, legacy := range []bool{false, true} {
		name := "modern only"
		if legacy {
			name = "legacy enabled"
		}
		t.Run(name, func(t *testing.T) {
			router := newRouterWithLegacyModeForTest(t, legacy)
			for _, path := range []string{"/service/version", "/boot/v1/service/version"} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				want := http.StatusOK
				if !legacy && path == "/boot/v1/service/version" {
					want = http.StatusNotFound
				}
				if response.Code != want {
					t.Fatalf("%s returned HTTP %d, want %d", path, response.Code, want)
				}
			}

			original := serviceStatusProvider
			t.Cleanup(func() { serviceStatusProvider = original })
			for _, tc := range []struct {
				status ServiceStatus
				code   int
			}{
				{ServiceStatusOK, http.StatusOK},
				{ServiceStatusDegraded, http.StatusOK},
				{ServiceStatusDown, http.StatusServiceUnavailable},
				{ServiceStatusInitializing, http.StatusServiceUnavailable},
			} {
				t.Run(string(tc.status), func(t *testing.T) {
					serviceStatusProvider = func() ServiceStatus { return tc.status }
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/service/status", nil))
					if response.Code != tc.code {
						t.Fatalf("got HTTP %d, want %d: %s", response.Code, tc.code, response.Body.String())
					}
					var body map[string]any
					if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					if len(body) != 2 || body["service"] != "boot-service" || body["status"] != string(tc.status) {
						t.Fatalf("unexpected public status response: %v", body)
					}
					spec := GenerateOpenAPISpec()
					operation := spec.Paths.Value("/service/status").Get
					schema := operation.Responses.Status(tc.code).Value.Content["application/json"].Schema.Value
					if err := schema.VisitJSON(body); err != nil {
						t.Fatalf("response does not match OpenAPI: %v", err)
					}
				})
			}

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/boot/v1/service/status", nil))
			if !legacy {
				if response.Code != http.StatusNotFound {
					t.Fatalf("legacy status must be disabled, got HTTP %d", response.Code)
				}
				return
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || body["service_name"] != "boot-script-service" || body["service_status"] != "running" || body["service_version"] == nil {
				t.Fatalf("legacy status contract changed: HTTP %d, %v", response.Code, body)
			}
		})
	}
}
