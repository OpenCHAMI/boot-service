// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openchami/boot-service/pkg/handlers/boot"
)

func TestOpenAPIServiceStatusMatchesBootHandler(t *testing.T) {
	var handler boot.Handler
	response := httptest.NewRecorder()
	handler.GetServiceStatus(response, httptest.NewRequest(http.MethodGet, "/service/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	var body any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	spec := GenerateOpenAPISpec()
	operation := spec.Paths.Value("/service/status").Get
	schema := operation.Responses.Status(http.StatusOK).Value.Content["application/json"].Schema.Value
	if err := schema.VisitJSON(body); err != nil {
		t.Fatalf("service status response does not match OpenAPI: %v", err)
	}
}
