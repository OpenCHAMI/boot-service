// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDeclarativeMetricsComposePlainExposition(t *testing.T) {
	metrics := NewMetrics("boot_service")
	features := &generatedFeatureSet{publicRoutes: []func(chi.Router){func(r chi.Router) { r.Handle("/metrics", metrics.Handler()) }}}
	handler := newMetricsHandler(features, func() uint64 { return 3 })
	for _, accept := range []string{"text/plain", "application/openmetrics-text"} {
		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("Accept", accept)
		req.Header.Set("Accept-Encoding", "gzip")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		text := response.Body.String()
		if response.Code != 200 || response.Header().Get("Content-Encoding") != "" || !strings.Contains(response.Header().Get("Content-Type"), "text/plain") || strings.Contains(text, "# EOF") {
			t.Fatalf("invalid exposition: %d %v", response.Code, response.Header())
		}
		for _, metric := range []string{"go_goroutines ", "boot_service_declarative_cache_hits_total 3\n", "boot_service_declarative_membership_cache_ttl_seconds 0\n"} {
			if !strings.Contains(text, metric) {
				t.Fatalf("missing %q", metric)
			}
		}
	}
}
