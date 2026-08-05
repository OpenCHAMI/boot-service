// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package hsm

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	bootclient "github.com/openchami/boot-service/pkg/client"
)

func TestIntegrationService_SyncNodesFromHSMUsesBulkMemberships(t *testing.T) {
	var bulkMembershipCalls int32
	var singularMembershipCalls int32
	var createdNodesMu sync.Mutex
	createdNodes := make([]bootclient.CreateNodeRequest, 0)

	hsmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hsm/v2/State/Components":
			response := HSMResponse{Components: []HSMComponent{
				{ID: "x1000c0s0b0n0", Type: "Node", Role: "Compute", NID: 100},
				{ID: "x1000c0s0b0n1", Type: "Node", Role: "Application", NID: 101},
				{ID: "x1000c0s0b0", Type: "NodeBMC", Role: "Management"},
			}}
			writeTestJSON(t, w, response)
		case "/hsm/v2/Inventory/EthernetInterfaces":
			response := []HSMEthernetInterface{
				{MACAddress: "00:1B:63:84:45:E6", ComponentID: "x1000c0s0b0n0", Type: "Node"},
				{MACAddress: "00:1B:63:84:45:E7", ComponentID: "x1000c0s0b0n1", Type: "Node"},
			}
			writeTestJSON(t, w, response)
		case "/hsm/v2/memberships":
			atomic.AddInt32(&bulkMembershipCalls, 1)
			if got := r.URL.Query().Get("type"); got != "node" {
				t.Errorf("expected type=node query, got %q", got)
			}
			response := []HSMMembership{
				{ID: "x1000c0s0b0n0", GroupLabels: []string{"compute", "batch"}},
				{ID: "x1000c0s0b0n1", GroupLabels: []string{"application"}},
			}
			writeTestJSON(t, w, response)
		default:
			if len(r.URL.Path) > len("/hsm/v2/memberships/") && r.URL.Path[:len("/hsm/v2/memberships/")] == "/hsm/v2/memberships/" {
				atomic.AddInt32(&singularMembershipCalls, 1)
			}
			http.NotFound(w, r)
		}
	}))
	defer hsmServer.Close()

	bootServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/nodes":
			writeTestJSON(t, w, []v1.Node{})
		case r.Method == http.MethodPost && r.URL.Path == "/nodes":
			var req bootclient.CreateNodeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode create node request: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			createdNodesMu.Lock()
			createdNodes = append(createdNodes, req)
			createdNodesMu.Unlock()
			writeTestJSON(t, w, v1.Node{Spec: req.Spec})
		default:
			http.NotFound(w, r)
		}
	}))
	defer bootServer.Close()

	config := DefaultHSMConfig()
	config.BaseURL = hsmServer.URL
	config.CacheExpiry = time.Minute

	hsmClient, err := NewHSMClient(config, log.New(os.Stdout, "test: ", log.LstdFlags))
	if err != nil {
		t.Fatalf("failed to create HSM client: %v", err)
	}
	bootClient, err := bootclient.NewClient(bootServer.URL, bootServer.Client(), bootclient.DefaultLogger())
	if err != nil {
		t.Fatalf("failed to create boot client: %v", err)
	}

	service, err := NewIntegrationServiceWithClient(hsmClient, DefaultIntegrationConfig(), *bootClient, log.New(os.Stdout, "test: ", log.LstdFlags))
	if err != nil {
		t.Fatalf("failed to create integration service: %v", err)
	}

	if err := service.SyncNodesFromHSM(context.Background()); err != nil {
		t.Fatalf("SyncNodesFromHSM failed: %v", err)
	}

	if atomic.LoadInt32(&bulkMembershipCalls) != 1 {
		t.Fatalf("expected 1 bulk membership request, got %d", atomic.LoadInt32(&bulkMembershipCalls))
	}
	if atomic.LoadInt32(&singularMembershipCalls) != 0 {
		t.Fatalf("expected 0 singular membership requests, got %d", atomic.LoadInt32(&singularMembershipCalls))
	}
	createdNodesMu.Lock()
	createdNodesCopy := append([]bootclient.CreateNodeRequest(nil), createdNodes...)
	createdNodesMu.Unlock()

	if len(createdNodesCopy) != 2 {
		t.Fatalf("expected 2 created nodes, got %d", len(createdNodesCopy))
	}
	if !stringSlicesEqual(createdNodesCopy[0].Spec.Groups, []string{"compute", "batch"}) {
		t.Fatalf("expected first node groups [compute batch], got %v", createdNodesCopy[0].Spec.Groups)
	}
	if !stringSlicesEqual(createdNodesCopy[1].Spec.Groups, []string{"application"}) {
		t.Fatalf("expected second node groups [application], got %v", createdNodesCopy[1].Spec.Groups)
	}
}

func TestIntegrationService_SyncNodesFromHSMRejectsBadMembershipRefresh(t *testing.T) {
	tests := []struct {
		name           string
		membershipCode int
		membershipBody string
		seedCache      bool
		wantErr        string
	}{
		{
			name:           "failed refresh with expired cache",
			membershipCode: http.StatusServiceUnavailable,
			membershipBody: `{"error":"smd unavailable"}`,
			seedCache:      true,
			wantErr:        "failed to refresh HSM memberships cache",
		},
		{
			name:           "empty bulk response",
			membershipCode: http.StatusOK,
			membershipBody: `[]`,
			wantErr:        "missing HSM membership data for compute node x1000c0s0b0n0",
		},
		{
			name:           "missing one compute node membership",
			membershipCode: http.StatusOK,
			membershipBody: `[{"id":"x1000c0s0b0n0","groupLabels":["compute"]}]`,
			wantErr:        "missing HSM membership data for compute node x1000c0s0b0n1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bootCalls int32
			var membershipCalls int32
			var singularMembershipCalls int32

			hsmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/hsm/v2/State/Components":
					response := HSMResponse{Components: []HSMComponent{
						{ID: "x1000c0s0b0n0", Type: "Node", Role: "Compute", NID: 100},
						{ID: "x1000c0s0b0n1", Type: "Node", Role: "Application", NID: 101},
					}}
					writeTestJSON(t, w, response)
				case "/hsm/v2/Inventory/EthernetInterfaces":
					response := []HSMEthernetInterface{
						{MACAddress: "00:1B:63:84:45:E6", ComponentID: "x1000c0s0b0n0", Type: "Node"},
						{MACAddress: "00:1B:63:84:45:E7", ComponentID: "x1000c0s0b0n1", Type: "Node"},
					}
					writeTestJSON(t, w, response)
				case "/hsm/v2/memberships":
					call := atomic.AddInt32(&membershipCalls, 1)
					if got := r.URL.Query().Get("type"); got != "node" {
						t.Errorf("expected type=node query, got %q", got)
					}
					if tt.seedCache && call == 1 {
						writeTestJSON(t, w, []HSMMembership{
							{ID: "x1000c0s0b0n0", GroupLabels: []string{"compute"}},
							{ID: "x1000c0s0b0n1", GroupLabels: []string{"application"}},
						})
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.membershipCode)
					_, _ = w.Write([]byte(tt.membershipBody))
				default:
					if strings.HasPrefix(r.URL.Path, "/hsm/v2/memberships/") {
						atomic.AddInt32(&singularMembershipCalls, 1)
					}
					http.NotFound(w, r)
				}
			}))
			defer hsmServer.Close()

			bootServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&bootCalls, 1)
				t.Errorf("boot API should not be called after bad membership refresh: %s %s", r.Method, r.URL.Path)
				http.Error(w, "unexpected boot API call", http.StatusInternalServerError)
			}))
			defer bootServer.Close()

			config := DefaultHSMConfig()
			config.BaseURL = hsmServer.URL
			config.CacheExpiry = time.Nanosecond

			hsmClient, err := NewHSMClient(config, log.New(os.Stdout, "test: ", log.LstdFlags))
			if err != nil {
				t.Fatalf("failed to create HSM client: %v", err)
			}
			if tt.seedCache {
				if _, err := hsmClient.GetMemberships(context.Background()); err != nil {
					t.Fatalf("failed to seed membership cache: %v", err)
				}
				time.Sleep(time.Millisecond)
			}

			bootClient, err := bootclient.NewClient(bootServer.URL, bootServer.Client(), bootclient.DefaultLogger())
			if err != nil {
				t.Fatalf("failed to create boot client: %v", err)
			}
			service, err := NewIntegrationServiceWithClient(hsmClient, DefaultIntegrationConfig(), *bootClient, log.New(os.Stdout, "test: ", log.LstdFlags))
			if err != nil {
				t.Fatalf("failed to create integration service: %v", err)
			}

			err = service.SyncNodesFromHSM(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
			if got := atomic.LoadInt32(&bootCalls); got != 0 {
				t.Fatalf("expected no boot API calls, got %d", got)
			}
			if got := atomic.LoadInt32(&singularMembershipCalls); got != 0 {
				t.Fatalf("expected no singular membership calls, got %d", got)
			}
		})
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("failed to encode JSON response: %v", err)
	}
}
