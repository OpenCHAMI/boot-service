// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/fabrica/pkg/resource"
)

type declarativeMetadata struct {
	Name        string            `json:"name"`
	UID         string            `json:"uid,omitempty"`
	CreatedAt   time.Time         `json:"createdAt,omitempty"`
	UpdatedAt   time.Time         `json:"updatedAt,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type configInput struct {
	Metadata    declarativeMetadata `json:"metadata"`
	Spec        *v1.BootConfigSpec  `json:"spec"`
	Labels      map[string]string   `json:"labels,omitempty"`
	Annotations map[string]string   `json:"annotations,omitempty"`
}

func (h *declarativeHTTP) propose(w http.ResponseWriter, r *http.Request, operation string) {
	var input configInput
	if !readDeclarative(w, r, &input) {
		return
	}
	if input.Spec == nil || input.Metadata.UID != "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	name := chi.URLParam(r, "name")
	if operation == "series" {
		name = input.Metadata.Name
	}
	if input.Labels == nil {
		input.Labels = input.Metadata.Labels
	}
	if input.Annotations == nil {
		input.Annotations = input.Metadata.Annotations
	}
	request := declarativeboot.ProposalRequest{
		Name: name, Capabilities: declarativeCapabilities(r),
		Resource: v1.BootConfig{Metadata: resource.Metadata{Name: input.Metadata.Name, Labels: input.Labels, Annotations: input.Annotations}, Spec: *input.Spec},
	}
	var result declarativeboot.ProposalResult
	var err error
	switch operation {
	case "series":
		request.RevisionName = r.URL.Query().Get("revisionName")
		result, err = h.revisions.CreateSeries(r.Context(), request)
	case "ensure":
		request.RevisionName = chi.URLParam(r, "revisionName")
		result, err = h.revisions.EnsureRevision(r.Context(), request)
	case "create":
		result, err = h.revisions.CreateRevision(r.Context(), request)
	}
	if err != nil {
		declarativeError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	declarativeJSON(w, status, result.Record, result.ETag)
}

func (h *declarativeHTTP) createSeries(w http.ResponseWriter, r *http.Request) {
	h.propose(w, r, "series")
}
func (h *declarativeHTTP) createRevision(w http.ResponseWriter, r *http.Request) {
	h.propose(w, r, "create")
}
func (h *declarativeHTTP) ensureRevision(w http.ResponseWriter, r *http.Request) {
	h.propose(w, r, "ensure")
}

func (h *declarativeHTTP) promoteDefault(w http.ResponseWriter, r *http.Request) {
	if err := declarativeConditions(r).Validate(); err != nil {
		declarativeError(w, err)
		return
	}
	var input struct {
		RevisionUID string `json:"revisionUid"`
	}
	if !readDeclarative(w, r, &input) {
		return
	}
	result, err := h.revisions.PromoteDefault(r.Context(), declarativeboot.PromotionRequest{Name: chi.URLParam(r, "name"), UID: input.RevisionUID, IfMatch: r.Header.Get("If-Match"), Capabilities: declarativeCapabilities(r)})
	if err != nil {
		declarativeError(w, err)
		return
	}
	declarativeJSON(w, http.StatusOK, result.Status, result.ETag)
}

func (h *declarativeHTTP) retireRevision(w http.ResponseWriter, r *http.Request) {
	declarativeError(w, h.revisions.RetireRevision(r.Context(), revisionReference(r)))
}
