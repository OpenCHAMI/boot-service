// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"
	"sync"
	"testing"
)

func TestDeclarativeHTTPCompetingNamedCreates(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	config := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-a"}}`
	if response := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", admin, config, "", ""); response.Code != http.StatusCreated {
		t.Fatalf("series: %d", response.Code)
	}
	body := `{"metadata":{"name":"canary"},"spec":{"configRef":{"name":"os"}}}`
	start := make(chan struct{})
	results := make(chan int, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results <- declarativeRequest(router, "PUT", "/bootassignments/by-name/canary", admin, body, "If-None-Match", "*").Code
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[http.StatusCreated] != 1 || counts[http.StatusPreconditionFailed] != 1 {
		t.Fatalf("unexpected competing-create results: %v", counts)
	}
}

func TestDeclarativeHTTPCompetingETagUpdates(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	config := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-a"}}`
	if response := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", admin, config, "", ""); response.Code != http.StatusCreated {
		t.Fatalf("series: %d", response.Code)
	}
	body := `{"metadata":{"name":"canary"},"spec":{"configRef":{"name":"os"}}}`
	created := declarativeRequest(router, "PUT", "/bootassignments/by-name/canary", admin, body, "If-None-Match", "*")
	if created.Code != http.StatusCreated {
		t.Fatalf("assignment: %d", created.Code)
	}
	start := make(chan struct{})
	results := make(chan int, 2)
	var workers sync.WaitGroup
	for _, patch := range []string{`{"priority":1}`, `{"priority":2}`} {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results <- declarativeRequest(router, "PATCH", "/bootassignments/by-name/canary", admin, patch, "If-Match", created.Header().Get("ETag")).Code
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusPreconditionFailed] != 1 {
		t.Fatalf("unexpected competing-update results: %v", counts)
	}
}
