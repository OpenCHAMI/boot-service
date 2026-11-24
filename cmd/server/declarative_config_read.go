// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openchami/fabrica/pkg/revision"
)

func revisionReference(r *http.Request) revision.Reference {
	return revision.Reference{Name: chi.URLParam(r, "name"), UID: chi.URLParam(r, "revisionUID")}
}

func (h *declarativeHTTP) getSeries(w http.ResponseWriter, r *http.Request) {
	value, etag, err := h.revisions.GetSeries(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		declarativeError(w, err)
		return
	}
	declarativeRead(w, r, value, etag)
}

func revisionJSON(w http.ResponseWriter, r *http.Request, value any, err error) {
	if err != nil {
		declarativeError(w, err)
		return
	}
	etag, err := revision.ValueETag(value)
	if err != nil {
		declarativeError(w, err)
		return
	}
	declarativeRead(w, r, value, etag)
}

func (h *declarativeHTTP) listRevisions(w http.ResponseWriter, r *http.Request) {
	value, err := h.revisions.ListRevisions(r.Context(), chi.URLParam(r, "name"))
	revisionJSON(w, r, value, err)
}

func (h *declarativeHTTP) getRevision(w http.ResponseWriter, r *http.Request) {
	value, err := h.revisions.GetRevision(r.Context(), revisionReference(r))
	revisionJSON(w, r, value, err)
}

func (h *declarativeHTTP) resolveRevision(w http.ResponseWriter, r *http.Request) {
	ref := revision.Reference{Name: chi.URLParam(r, "name"), UID: r.URL.Query().Get("uid"), RevisionName: r.URL.Query().Get("revisionName"), Selector: revision.Selector(r.URL.Query().Get("selector"))}
	value, err := h.revisions.ResolveRevision(r.Context(), ref)
	if err != nil {
		declarativeError(w, err)
		return
	}
	revisionJSON(w, r, RevisionResolveResponse{SeriesName: ref.Name, Selector: ref.SelectorOrDefault(), Revision: value.Metadata}, nil)
}
