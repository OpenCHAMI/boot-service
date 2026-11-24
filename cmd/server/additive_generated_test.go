// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestGeneratedAdditiveModelsOnHTTP(t *testing.T) {
	registerResourcePrefixesForTest(t)
	router := newGeneratedRouterForTest(t)
	request := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		return rec
	}
	config := request("/bootconfigs?revisionName=A", `{"metadata":{"name":"Config.A"},"spec":{"kernel":"https://images.example.invalid/kernel"}}`)
	if config.Code != http.StatusCreated {
		t.Fatalf("create series: HTTP %d: %s", config.Code, config.Body.String())
	}
	var record revision.Record[*v1.BootConfig]
	if err := json.Unmarshal(config.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Metadata.SeriesName != "Config.A" || record.Resource.Spec.Kernel != "https://images.example.invalid/kernel" {
		t.Fatalf("unexpected revision: %+v", record)
	}
	assignment := request("/bootassignments", `{"metadata":{"name":"Assignment.A"},"spec":{"selector":{"macs":["AA-BB-CC-DD-EE-FF"]},"configRef":{"name":"Config.A"}}}`)
	if assignment.Code != http.StatusCreated {
		t.Fatalf("create assignment: HTTP %d: %s", assignment.Code, assignment.Body.String())
	}
	var created v1.BootAssignment
	if err := json.Unmarshal(assignment.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Metadata.UID, "bootassignment_") || created.Spec.Selector == nil || created.Spec.Selector.MACs[0] != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("unexpected assignment: %+v", created)
	}
}

func TestGeneratedAdditiveModelsRejectInvalidHTTP(t *testing.T) {
	registerResourcePrefixesForTest(t)
	router := newGeneratedRouterForTest(t)
	tests := []struct{ path, body string }{
		{"/bootconfigs", `{"metadata":{"name":"bad/name"},"spec":{"kernel":"/boot/kernel"}}`},
		{"/bootconfigs", `{"metadata":{"name":"config"},"spec":{"kernel":"s3://bucket/kernel"}}`},
		{"/bootassignments", `{"metadata":{"name":"assignment"},"spec":{"priority":101,"configRef":{"name":"config"}}}`},
		{"/bootassignments", `{"metadata":{"name":"assignment"},"spec":{"configRef":{"name":"config","revisionName":"A","selector":"latest"}}}`},
	}
	for _, tc := range tests {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got HTTP %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
