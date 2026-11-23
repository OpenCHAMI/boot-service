// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/fabrica/pkg/patch"
)

// statusTransform decodes a status-only replacement, or applies a patch to the
// current status inside the mutation gate. Resource/spec fields are rejected.
func statusTransform[T any](w http.ResponseWriter, r *http.Request) (func(T) (T, error), bool) {
	if r.Method == http.MethodPut {
		var status T
		if !readDeclarative(w, r, &status) {
			return nil, false
		}
		return func(T) (T, error) { return status, nil }, true
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid patch", http.StatusBadRequest)
		return nil, false
	}
	return func(current T) (T, error) {
		encoded, err := json.Marshal(current)
		if err != nil {
			return current, err
		}
		result, err := patch.ApplyPatchWithOptions(encoded, data, patch.DetectPatchType(r.Header.Get("Content-Type")), patch.PatchOptions{AllowAddFields: true, AllowRemoveFields: true})
		if err != nil {
			return current, declarativeboot.ErrInvalid
		}
		var updated T
		if err := decodeDeclarative(result.Updated, &updated); err != nil {
			return current, declarativeboot.ErrInvalid
		}
		return updated, nil
	}, true
}

func (h *declarativeHTTP) assignmentStatus(w http.ResponseWriter, r *http.Request) {
	transform, ok := statusTransform[v1.BootAssignmentStatus](w, r)
	if !ok {
		return
	}
	result, err := h.manager.TransformStatusByUID(r.Context(), declarativeboot.StatusTarget{
		UID: chi.URLParam(r, "uid"), Conditions: declarativeConditions(r),
	}, transform)
	if err != nil {
		declarativeError(w, err)
		return
	}
	declarativeJSON(w, http.StatusOK, result.Assignment.Status, result.ETag)
}

func (h *declarativeHTTP) seriesStatus(w http.ResponseWriter, r *http.Request) {
	transform, ok := statusTransform[v1.BootConfigStatus](w, r)
	if !ok {
		return
	}
	status, etag, err := h.revisions.TransformStatus(r.Context(), chi.URLParam(r, "name"), declarativeConditions(r), transform)
	if err != nil {
		declarativeError(w, err)
		return
	}
	declarativeJSON(w, http.StatusOK, status, etag)
}
