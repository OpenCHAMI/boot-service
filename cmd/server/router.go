// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openchami/boot-service/pkg/auth"
	"github.com/openchami/fabrica/pkg/versioning"
)

func declarativePath(path string) bool {
	return path == "/bootconfigs" || strings.HasPrefix(path, "/bootconfigs/") || path == "/bootassignments" || strings.HasPrefix(path, "/bootassignments/")
}

func slashPolicy(next http.Handler) http.Handler {
	legacy := middleware.RedirectSlashes(next)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !declarativePath(req.URL.Path) {
			legacy.ServeHTTP(w, req)
			return
		}
		clone := req.Clone(req.Context())
		clone.URL.Path = strings.TrimRight(req.URL.Path, "/")
		clone.URL.RawPath = ""
		next.ServeHTTP(w, clone)
	})
}

func newRouter(config Config, features *generatedFeatureSet, boot *bootRoutes) chi.Router {
	r := chi.NewRouter()
	verifier := auth.NewDeclarativeVerifier(auth.DeclarativeConfig{Enabled: config.AuthEnabled, Issuer: config.TokenSmithURL, JWKSURL: os.Getenv(jwksURLEnv)})
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, slashPolicy)
	r.Use(middleware.Timeout(time.Duration(config.ReadTimeout) * time.Second))
	features.Use(r)
	r.Use(versioning.VersionNegotiationMiddleware(versioning.GlobalVersionRegistry, nil))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodOptions && declarativePath(req.URL.Path) {
				verifier.Require(auth.ScopeReadBoot)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusMethodNotAllowed)
				})).ServeHTTP(w, req)
				return
			}
			next.ServeHTTP(w, req)
		})
	})
	r.Group(func(public chi.Router) {
		public.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","service":"boot-service"}`))
		})
		RegisterGeneratedPublicRoutes(public)
		public.Get("/openapi.json", ServeOpenAPISpec)
		public.Get("/docs", ServeSwaggerUI)
		features.RegisterPublicRoutes(public)

		boot.registerPublic(public)
	})
	r.Group(func(protected chi.Router) {
		features.UseProtected(protected)
		RegisterGeneratedRoutes(legacyGeneratedRouter{Router: protected})
		boot.registerProtected(protected)
	})
	registerDeclarativeRoutes(r, boot.manager, verifier)
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		if !declarativePath(req.URL.Path) {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		scope := auth.ScopePromoteBoot
		if req.Method == http.MethodGet || req.Method == http.MethodHead {
			scope = auth.ScopeReadBoot
		}
		verifier.Require(scope)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusMethodNotAllowed)
		})).ServeHTTP(w, req)
	})
	return r
}

func newMetricsHandler(features *generatedFeatureSet) http.Handler {
	r := chi.NewRouter()
	features.RegisterPublicRoutes(r)
	return r
}
