// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package hsm

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDeclarativeProviderFreshSMDIdentity(t *testing.T) {
	var groups atomic.Value
	groups.Store(`["canary"]`)
	var membershipAvailable atomic.Bool
	membershipAvailable.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/hsm/v2/State/Components":
			_, _ = io.WriteString(w, `{"Components":[{"ID":"x0c0s0b0n0","Type":"Node","NID":42}]}`)
		case "/hsm/v2/Inventory/EthernetInterfaces":
			_, _ = io.WriteString(w, `[{"ComponentID":"x0c0s0b0n0","MACAddress":"aa:bb:cc:dd:ee:ff"}]`)
		case "/hsm/v2/memberships":
			if r.URL.Query().Get("type") != "Node" {
				http.Error(w, "bad type", http.StatusBadRequest)
				return
			}
			if !membershipAvailable.Load() {
				_, _ = io.WriteString(w, `[]`)
				return
			}
			_, _ = io.WriteString(w, `[{"id":"x0c0s0b0n0","groupLabels":`+groups.Load().(string)+`}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	config := DefaultHSMConfig()
	config.BaseURL, config.AuthToken = server.URL, "test-token"
	client, err := NewHSMClient(config, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	provider := NewDeclarativeNodeProvider(client)
	first, err := provider.ResolveNodeByIdentifier(context.Background(), "AA-BB-CC-DD-EE-FF")
	if err != nil || first.Spec.XName != "x0c0s0b0n0" || first.Spec.NID != 42 || len(first.Spec.Groups) != 1 || first.Spec.Groups[0] != "canary" {
		t.Fatalf("first: %#v %v", first, err)
	}
	groups.Store(`["production"]`)
	second, err := provider.ResolveNodeByIdentifier(context.Background(), "42")
	if err != nil || len(second.Spec.Groups) != 1 || second.Spec.Groups[0] != "production" {
		t.Fatalf("second: %#v %v", second, err)
	}
	membershipAvailable.Store(false)
	if _, err := provider.ResolveNodeByIdentifier(context.Background(), "42"); err == nil {
		t.Fatal("missing membership must fail closed")
	}
}

func TestDeclarativeProviderPropagatesCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	config := DefaultHSMConfig()
	config.BaseURL = server.URL
	client, err := NewHSMClient(config, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := NewDeclarativeNodeProvider(client).ResolveNodeByIdentifier(ctx, "x0c0s0b0n0")
		result <- err
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
