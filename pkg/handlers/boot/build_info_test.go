// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package boot

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openchami/boot-service/pkg/client"
)

func TestServiceEndpointsUseBuildInfo(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	constructors := []struct {
		name string
		new  func(...HandlerOption) *Handler
	}{
		{"standard", func(options ...HandlerOption) *Handler {
			return NewHandler(client.Client{}, logger, options...)
		}},
		{"custom controller", func(options ...HandlerOption) *Handler {
			return NewHandlerWithController(client.Client{}, nil, logger, options...)
		}},
	}
	for _, constructor := range constructors {
		for _, tc := range []struct {
			name    string
			options []HandlerOption
			want    ServiceVersion
		}{
			{
				name: "release",
				options: []HandlerOption{WithBuildInfo(BuildInfo{
					Version: "v0.3.4", Commit: "abc1234", Date: "2026-10-21T12:00:00Z",
				})},
				want: ServiceVersion{
					ServiceName: "boot-script-service", ServiceVersion: "v0.3.4",
					GitCommit: "abc1234", BuildDate: "2026-10-21T12:00:00Z",
				},
			},
			{
				name: "no metadata",
				want: ServiceVersion{ServiceName: "boot-script-service", ServiceVersion: "dev"},
			},
			{
				name:    "unset build variables",
				options: []HandlerOption{WithBuildInfo(BuildInfo{Commit: "none", Date: "unknown"})},
				want:    ServiceVersion{ServiceName: "boot-script-service", ServiceVersion: "dev"},
			},
		} {
			t.Run(constructor.name+"/"+tc.name, func(t *testing.T) {
				handler := constructor.new(tc.options...)
				router := chi.NewRouter()
				handler.RegisterModernRoutes(router)
				handler.RegisterLegacyRoutes(router)
				for _, path := range []string{"/service/version", "/boot/v1/service/version"} {
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
					if response.Code != http.StatusOK {
						t.Fatalf("%s returned HTTP %d", path, response.Code)
					}
					var version ServiceVersion
					if err := json.Unmarshal(response.Body.Bytes(), &version); err != nil {
						t.Fatal(err)
					}
					if version != tc.want {
						t.Fatalf("%s returned %+v, want %+v", path, version, tc.want)
					}
					var fields map[string]any
					if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
						t.Fatal(err)
					}
					if tc.want.GitCommit == "" && (fields["git_commit"] != nil || fields["build_date"] != nil) {
						t.Fatalf("unavailable metadata should be omitted: %v", fields)
					}
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/boot/v1/service/status", nil))
				var status ServiceStatus
				if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusOK || status.ServiceVersion != tc.want.ServiceVersion || status.ServiceStatus != "running" {
					t.Fatalf("legacy status does not identify the same build: %+v", status)
				}
			})
		}
	}
}
