// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/fabrica/pkg/resource"
)

type assignmentInput struct {
	Metadata    declarativeMetadata    `json:"metadata"`
	Spec        *v1.BootAssignmentSpec `json:"spec"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Annotations map[string]string      `json:"annotations,omitempty"`
}

func (input assignmentInput) value() v1.BootAssignment {
	if input.Labels == nil {
		input.Labels = input.Metadata.Labels
	}
	if input.Annotations == nil {
		input.Annotations = input.Metadata.Annotations
	}
	return v1.BootAssignment{Metadata: resource.Metadata{Name: input.Metadata.Name, UID: input.Metadata.UID, Labels: input.Labels, Annotations: input.Annotations}, Spec: *input.Spec}
}

func assignmentResult(w http.ResponseWriter, result declarativeboot.MutationResult, err error) {
	if err != nil {
		declarativeError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	declarativeJSON(w, status, result.Assignment, result.ETag)
}

func (h *declarativeHTTP) createAssignment(w http.ResponseWriter, r *http.Request) {
	if err := declarativeConditions(r).Validate(); err != nil {
		declarativeError(w, err)
		return
	}
	var input assignmentInput
	if !readDeclarative(w, r, &input) {
		return
	}
	if input.Spec == nil {
		http.Error(w, "spec required", http.StatusBadRequest)
		return
	}
	result, err := h.manager.Create(r.Context(), input.value())
	assignmentResult(w, result, err)
}

func (h *declarativeHTTP) putAssignmentUID(w http.ResponseWriter, r *http.Request) {
	var input assignmentInput
	if !readDeclarative(w, r, &input) {
		return
	}
	if input.Spec == nil {
		http.Error(w, "spec required", http.StatusBadRequest)
		return
	}
	result, err := h.manager.PutByUID(r.Context(), chi.URLParam(r, "uid"), input.value(), declarativeConditions(r))
	assignmentResult(w, result, err)
}

func (h *declarativeHTTP) putAssignmentName(w http.ResponseWriter, r *http.Request) {
	var input assignmentInput
	if !readDeclarative(w, r, &input) {
		return
	}
	if input.Spec == nil {
		http.Error(w, "spec required", http.StatusBadRequest)
		return
	}
	result, err := h.manager.PutByName(r.Context(), chi.URLParam(r, "name"), input.value(), declarativeConditions(r))
	assignmentResult(w, result, err)
}

func (h *declarativeHTTP) deleteAssignmentUID(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.DeleteByUID(r.Context(), chi.URLParam(r, "uid"), declarativeConditions(r)); err != nil {
		declarativeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *declarativeHTTP) deleteAssignmentName(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.DeleteByName(r.Context(), chi.URLParam(r, "name"), declarativeConditions(r)); err != nil {
		declarativeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
