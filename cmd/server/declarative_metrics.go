// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Append service-owned collectors without changing the generated registry or
// exposing per-node labels. Force Prometheus text so the two registries form one
// valid response (OpenMetrics has an EOF marker and cannot be concatenated).
func declarativeMetricsHandler(features *generatedFeatureSet, hits func() uint64) http.Handler {
	base := newMetricsHandler(features)
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "boot_service_declarative_cache_hits_total",
		Help: "Successful declarative render-cache lookups after current-state validation.",
	}, func() float64 { return float64(hits()) }))
	registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "boot_service_declarative_membership_cache_ttl_seconds",
		Help: "Declarative identity cache lifetime; zero means refresh on every request.",
	}, func() float64 { return 0 }))
	extra := promhttp.HandlerFor(registry, promhttp.HandlerOpts{DisableCompression: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := r.Clone(r.Context())
		req.Header = r.Header.Clone()
		req.Header.Set("Accept", "text/plain; version=0.0.4")
		req.Header.Del("Accept-Encoding")
		// Buffer each exposition so handlers cannot commit conflicting headers,
		// lengths or status codes to the same response.
		original := &recordedResponse{header: make(http.Header)}
		base.ServeHTTP(original, req)
		additional := &recordedResponse{header: make(http.Header)}
		if original.status == http.StatusOK {
			extra.ServeHTTP(additional, req)
		}
		for name, values := range original.header {
			w.Header()[name] = values
		}
		w.Header().Del("Content-Length")
		w.WriteHeader(original.status)
		_, _ = w.Write(original.body.Bytes())
		if additional.status == http.StatusOK {
			_, _ = w.Write(additional.body.Bytes())
		}
	})
}
