// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

func (h *declarativeHTTP) listAssignments(w http.ResponseWriter, r *http.Request) {
	snapshots, err := h.manager.List(r.Context())
	if err != nil {
		declarativeError(w, err)
		return
	}
	values := make([]v1.BootAssignment, 0, len(snapshots))
	for _, snapshot := range snapshots {
		values = append(values, snapshot.Assignment)
	}
	etag, err := revision.ValueETag(values)
	if err != nil {
		declarativeError(w, err)
		return
	}
	declarativeRead(w, r, values, etag)
}

func (h *declarativeHTTP) getAssignmentUID(w http.ResponseWriter, r *http.Request) {
	value, err := h.manager.GetByUID(r.Context(), chi.URLParam(r, "uid"))
	if err != nil {
		declarativeError(w, err)
		return
	}
	declarativeRead(w, r, value.Assignment, value.ETag)
}

func (h *declarativeHTTP) getAssignmentName(w http.ResponseWriter, r *http.Request) {
	value, err := h.manager.GetByName(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		declarativeError(w, err)
		return
	}
	declarativeRead(w, r, value.Assignment, value.ETag)
}
