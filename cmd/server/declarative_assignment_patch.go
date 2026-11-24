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

func (h *declarativeHTTP) patchAssignment(w http.ResponseWriter, r *http.Request, named bool) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid patch", http.StatusBadRequest)
		return
	}
	transform := func(spec v1.BootAssignmentSpec) (v1.BootAssignmentSpec, error) {
		current, err := json.Marshal(spec)
		if err != nil {
			return spec, err
		}
		result, err := patch.ApplyPatchWithOptions(current, data, patch.DetectPatchType(r.Header.Get("Content-Type")), patch.PatchOptions{AllowAddFields: true, AllowRemoveFields: true})
		if err != nil {
			return spec, declarativeboot.ErrInvalid
		}
		var updated v1.BootAssignmentSpec
		if err := decodeDeclarative(result.Updated, &updated); err != nil {
			return spec, declarativeboot.ErrInvalid
		}
		return updated, nil
	}
	var result declarativeboot.MutationResult
	if named {
		result, err = h.manager.TransformByName(r.Context(), chi.URLParam(r, "name"), declarativeConditions(r), transform)
	} else {
		result, err = h.manager.TransformByUID(r.Context(), chi.URLParam(r, "uid"), declarativeConditions(r), transform)
	}
	assignmentResult(w, result, err)
}

func (h *declarativeHTTP) patchAssignmentUID(w http.ResponseWriter, r *http.Request) {
	h.patchAssignment(w, r, false)
}

func (h *declarativeHTTP) patchAssignmentName(w http.ResponseWriter, r *http.Request) {
	h.patchAssignment(w, r, true)
}
