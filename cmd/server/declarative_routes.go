// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openchami/boot-service/pkg/auth"
	"github.com/openchami/boot-service/pkg/declarativeboot"
)

type declarativeHTTP struct {
	manager   *declarativeboot.Manager
	revisions *declarativeboot.RevisionService
}

func registerDeclarativeRoutes(r chi.Router, manager *declarativeboot.Manager, verifier *auth.DeclarativeVerifier) {
	h := &declarativeHTTP{manager: manager}
	if manager != nil {
		h.revisions = declarativeboot.NewRevisionService(manager)
	}
	add := func(method, path, scope string, handler http.HandlerFunc) {
		r.Method(method, path, verifier.Require(scope)(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if h.manager == nil {
				http.Error(w, "management unavailable", http.StatusServiceUnavailable)
				return
			}
			handler(w, req)
		})))
	}
	add("POST", "/bootconfigs", auth.ScopeProposeBoot, h.createSeries)
	add("GET", "/bootconfigs/{name}", auth.ScopeReadBoot, h.getSeries)
	add("POST", "/bootconfigs/{name}/revisions", auth.ScopeProposeBoot, h.createRevision)
	add("GET", "/bootconfigs/{name}/revisions", auth.ScopeReadBoot, h.listRevisions)
	add("PUT", "/bootconfigs/{name}/revisions/{revisionName}", auth.ScopeProposeBoot, h.ensureRevision)
	add("GET", "/bootconfigs/{name}/revisions/by-uid/{revisionUID}", auth.ScopeReadBoot, h.getRevision)
	add("DELETE", "/bootconfigs/{name}/revisions/by-uid/{revisionUID}", auth.ScopePromoteBoot, h.retireRevision)
	add("GET", "/bootconfigs/{name}/resolve", auth.ScopeReadBoot, h.resolveRevision)
	add("PUT", "/bootconfigs/{name}/default", auth.ScopePromoteBoot, h.promoteDefault)
	add("PUT", "/bootconfigs/{name}/status", auth.ScopePromoteBoot, h.seriesStatus)
	add("PATCH", "/bootconfigs/{name}/status", auth.ScopePromoteBoot, h.seriesStatus)
	add("GET", "/bootassignments", auth.ScopeReadBoot, h.listAssignments)
	add("POST", "/bootassignments", auth.ScopePromoteBoot, h.createAssignment)
	add("GET", "/bootassignments/{uid}", auth.ScopeReadBoot, h.getAssignmentUID)
	add("PUT", "/bootassignments/{uid}", auth.ScopePromoteBoot, h.putAssignmentUID)
	add("PATCH", "/bootassignments/{uid}", auth.ScopePromoteBoot, h.patchAssignmentUID)
	add("DELETE", "/bootassignments/{uid}", auth.ScopePromoteBoot, h.deleteAssignmentUID)
	add("GET", "/bootassignments/by-name/{name}", auth.ScopeReadBoot, h.getAssignmentName)
	add("PUT", "/bootassignments/by-name/{name}", auth.ScopePromoteBoot, h.putAssignmentName)
	add("PATCH", "/bootassignments/by-name/{name}", auth.ScopePromoteBoot, h.patchAssignmentName)
	add("DELETE", "/bootassignments/by-name/{name}", auth.ScopePromoteBoot, h.deleteAssignmentName)
	add("PUT", "/bootassignments/{uid}/status", auth.ScopePromoteBoot, h.assignmentStatus)
	add("PATCH", "/bootassignments/{uid}/status", auth.ScopePromoteBoot, h.assignmentStatus)
	for _, prefix := range []string{"/bootconfigs", "/bootassignments"} {
		add("OPTIONS", prefix, auth.ScopeReadBoot, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusMethodNotAllowed)
		})
		r.Handle(prefix+"/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			scope := auth.ScopePromoteBoot
			if req.Method == http.MethodGet || req.Method == http.MethodHead {
				scope = auth.ScopeReadBoot
			}
			verifier.Require(scope)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			})).ServeHTTP(w, req)
		}))
	}
}
