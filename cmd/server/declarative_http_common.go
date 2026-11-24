// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	conditional "github.com/openchami/boot-service/internal/middleware"
	"github.com/openchami/boot-service/pkg/auth"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/fabrica/pkg/revision"
)

func decodeDeclarative(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(json.RawMessage)) != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func readDeclarative(w http.ResponseWriter, r *http.Request, target any) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil || decodeDeclarative(data, target) != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func declarativeConditions(r *http.Request) declarativeboot.Conditions {
	return declarativeboot.Conditions{IfMatch: r.Header.Values("If-Match"), IfNoneMatch: r.Header.Values("If-None-Match")}
}

func declarativeCapabilities(r *http.Request) declarativeboot.Capabilities {
	p, _ := auth.DeclarativePrincipalFromContext(r.Context())
	return declarativeboot.Capabilities{Propose: p.HasScope(auth.ScopeProposeBoot), Promote: p.HasScope(auth.ScopePromoteBoot)}
}

func declarativeError(w http.ResponseWriter, err error) {
	status := http.StatusServiceUnavailable
	code := "unavailable"
	var precondition *revision.PreconditionError
	switch {
	case errors.Is(err, declarativeboot.ErrStoreUncertain):
	case errors.Is(err, declarativeboot.ErrRetirementDisabled):
		status, code = http.StatusConflict, "boot_config_retirement_disabled"
	case errors.As(err, &precondition):
		status, code = http.StatusPreconditionFailed, "precondition_failed"
		if precondition.CurrentETag != "" {
			w.Header().Set("ETag", precondition.CurrentETag)
		}
	case errors.Is(err, declarativeboot.ErrRevisionCorrupt):
	case errors.Is(err, declarativeboot.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, declarativeboot.ErrPrecondition), errors.Is(err, revision.ErrPreconditionFailed):
		status, code = http.StatusPreconditionFailed, "precondition_failed"
	case errors.Is(err, declarativeboot.ErrConflict), errors.Is(err, revision.ErrSeriesExists), errors.Is(err, revision.ErrRevisionNameConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, declarativeboot.ErrBadCondition):
		status, code = http.StatusBadRequest, "invalid_condition"
	case errors.Is(err, declarativeboot.ErrInvalid), errors.Is(err, revision.ErrInvalidReference), errors.Is(err, revision.ErrUnsupportedSelector):
		status, code = http.StatusUnprocessableEntity, "invalid"
	case errors.Is(err, revision.ErrRevisionRetired):
		status, code = http.StatusGone, "retired"
	case errors.Is(err, declarativeboot.ErrNotFound), errors.Is(err, revision.ErrSeriesNotFound), errors.Is(err, revision.ErrRevisionNotFound):
		status, code = http.StatusNotFound, "not_found"
	}
	respondJSON(w, status, struct {
		Code string `json:"code"`
	}{code})
}

func declarativeJSON(w http.ResponseWriter, status int, value any, etag string) {
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	respondJSON(w, status, value)
}

func declarativeRead(w http.ResponseWriter, r *http.Request, value any, etag string) {
	w.Header().Set("ETag", etag)
	if conditional.CheckIfNoneMatch(w, r, etag) {
		declarativeJSON(w, http.StatusOK, value, etag)
	}
}
