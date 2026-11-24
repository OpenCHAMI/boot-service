// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Never mount generated mutations for resources owned by the shared manager.
type legacyGeneratedRouter struct{ chi.Router }

func (r legacyGeneratedRouter) Route(pattern string, fn func(chi.Router)) chi.Router {
	if pattern == "/bootconfigs" || pattern == "/bootassignments" {
		return chi.NewRouter()
	}
	return r.Router.Route(pattern, fn)
}

// The API/storage layer is deliberately unavailable until its guarded service
// handlers are installed by the next integration layer.
func containDeclarativeRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range []string{"/bootconfigs", "/bootassignments"} {
			if r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/") {
				http.Error(w, "management unavailable", 503)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
