// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/bootadopt"
	"github.com/openchami/fabrica/pkg/resource"
)

func TestBootAdoptionUsesRealManagedWireContractAndReplays(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	source := v1.BootConfiguration{Metadata: resource.Metadata{UID: "source"}, Spec: v1.BootConfigurationSpec{Kernel: "/kernel", Hosts: []string{"x0c0s0b0n0"}}}
	interrupted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/bootconfigurations/source" {
			if err := json.NewEncoder(w).Encode(source); err != nil {
				t.Error(err)
			}
			return
		}
		result := httptest.NewRecorder()
		router.ServeHTTP(result, r)
		if !interrupted && r.Method == http.MethodPost && r.URL.Path == "/bootconfigs" && result.Code == http.StatusCreated {
			interrupted = true
			http.Error(w, "injected post-commit failure", http.StatusBadGateway)
			return
		}
		for name, values := range result.Header() {
			w.Header()[name] = values
		}
		w.WriteHeader(result.Code)
		_, _ = io.Copy(w, result.Body)
	}))
	t.Cleanup(server.Close)
	options := bootadopt.Options{Server: server.URL, Token: declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot"), SourceUID: "source", Series: "production", Assignment: "canary", Execute: true}
	receipt, err := bootadopt.Run(context.Background(), options)
	if err == nil || !interrupted || receipt.Phase != "response" || receipt.Error != "POST returned HTTP 502" || receipt.Requests.Started != receipt.Requests.Completed {
		t.Fatalf("first run: %#v, %v", receipt, err)
	}
	for range 2 {
		receipt, err = bootadopt.Run(context.Background(), options)
		if err != nil || receipt.Phase != "complete" || receipt.Requests.Ambiguous {
			t.Fatalf("retry: %#v, %v", receipt, err)
		}
	}
	response := declarativeRequest(router, "GET", "/bootconfigs/production/revisions", options.Token, "", "", "")
	var records []json.RawMessage
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &records) != nil || len(records) != 1 {
		t.Fatalf("duplicate adoption revisions: HTTP %d %s", response.Code, response.Body.String())
	}
}
