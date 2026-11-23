// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootadopt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestRunDryRunPartialFailureAndReplay(t *testing.T) {
	source := v1.BootConfiguration{Metadata: resource.Metadata{UID: "source"}, Spec: v1.BootConfigurationSpec{Kernel: "/kernel", Hosts: []string{"x0c0s0b0n0"}}}
	var record *revision.Record[*v1.BootConfig]
	var assignment *v1.BootAssignment
	writes, revisionWrites, failAssignment := 0, 0, true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing bearer")
		}
		write := func(value any) {
			t.Helper()
			if err := json.NewEncoder(w).Encode(value); err != nil {
				t.Error(err)
			}
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/bootconfigurations/source":
			write(source)
		case r.Method == "GET" && r.URL.Path == "/bootconfigs/production":
			if record == nil {
				http.NotFound(w, r)
				return
			}
			write(revision.SeriesStatus{Name: "production"})
		case r.Method == "POST" && r.URL.Path == "/bootconfigs":
			writes++
			revisionWrites++
			if r.URL.Query().Get("revisionName") != "legacy-source" {
				t.Error("missing deterministic name")
			}
			var config v1.BootConfig
			if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
				t.Error(err)
			}
			record = &revision.Record[*v1.BootConfig]{Metadata: revision.Metadata{UID: "revision", RevisionName: "legacy-source"}, Resource: &config}
			write(record)
		case r.Method == "PUT" && r.URL.Path == "/bootconfigs/production/revisions/legacy-source":
			write(record)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/bootassignments/"):
			if assignment == nil {
				http.NotFound(w, r)
				return
			}
			write(assignment)
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/bootassignments/"):
			if failAssignment {
				failAssignment = false
				http.Error(w, "interrupted", 503)
				return
			}
			if r.Header.Get("If-None-Match") != "*" {
				t.Error("unconditional assignment write")
			}
			writes++
			assignment = new(v1.BootAssignment)
			if err := json.NewDecoder(r.Body).Decode(assignment); err != nil {
				t.Error(err)
			}
			write(assignment)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	options := Options{Server: server.URL, Token: "test-token", SourceUID: "source", Series: "production", Assignment: "canary"}
	receipt, err := Run(context.Background(), options)
	if err != nil || writes != 0 || receipt.Requests.Started != 1 || receipt.Requests.Completed != 1 || receipt.Phase != "complete" {
		t.Fatalf("dry-run %#v: %v", receipt, err)
	}
	options.Execute = true
	receipt, err = Run(context.Background(), options)
	if err == nil || record == nil || assignment != nil || receipt.Phase != "response" || receipt.Requests.Started != receipt.Requests.Completed || receipt.Requests.Ambiguous {
		t.Fatalf("partial %#v: %v", receipt, err)
	}
	for range 2 {
		receipt, err = Run(context.Background(), options)
		if err != nil || receipt.Phase != "complete" || receipt.Requests.Started != receipt.Requests.Completed {
			t.Fatalf("retry %#v: %v", receipt, err)
		}
	}
	if writes != 2 || revisionWrites != 1 || assignment.Spec.ConfigRef.UID != "revision" {
		t.Fatalf("duplicate writes: %d revision writes %d", writes, revisionWrites)
	}
}

func TestRunTransportFailureIsAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = connection.Close()
	}))
	defer server.Close()
	receipt, err := Run(context.Background(), Options{Server: server.URL, SourceUID: "source", Series: "production", Assignment: "canary"})
	if err == nil || receipt.Phase != "transport" || !receipt.Requests.Ambiguous || receipt.Requests.Started != 1 || receipt.Requests.Completed != 0 {
		t.Fatalf("incorrect transport receipt %#v: %v", receipt, err)
	}
}
