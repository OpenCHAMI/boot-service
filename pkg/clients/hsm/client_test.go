// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// HSM client tests
package hsm

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestHSMClient_GetComponents tests retrieving components from HSM
func TestHSMClient_GetComponents(t *testing.T) {
	// Mock HSM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hsm/v2/State/Components" {
			t.Errorf("Expected path /hsm/v2/State/Components, got %s", r.URL.Path)
		}

		response := HSMResponse{
			Components: []HSMComponent{
				{
					ID:      "x1000c0s0b0n0",
					Type:    "Node",
					State:   "Ready",
					Enabled: true,
					Role:    "Compute",
					NID:     123,
				},
				{
					ID:      "x1000c0s0b0n1",
					Type:    "Node",
					State:   "Ready",
					Enabled: true,
					Role:    "Compute",
					NID:     124,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response) //nolint:errcheck
	}))
	defer server.Close()

	// Create client
	config := DefaultHSMConfig()
	config.BaseURL = server.URL
	config.CacheExpiry = 100 * time.Millisecond // Short cache for testing

	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	client := NewHSMClient(config, logger)

	// Test getting components
	ctx := context.Background()
	components, err := client.GetComponents(ctx)
	if err != nil {
		t.Fatalf("Failed to get components: %v", err)
	}

	if len(components) != 2 {
		t.Errorf("Expected 2 components, got %d", len(components))
	}

	// Verify first component
	if components[0].ID != "x1000c0s0b0n0" {
		t.Errorf("Expected ID x1000c0s0b0n0, got %s", components[0].ID)
	}

	if components[0].NID != 123 {
		t.Errorf("Expected NID 123, got %d", components[0].NID)
	}

	t.Logf("✅ Retrieved %d components from HSM", len(components))
}

// TestHSMClient_GetComponent tests retrieving a specific component
func TestHSMClient_GetComponent(t *testing.T) {
	// Mock HSM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/hsm/v2/State/Components/x1000c0s0b0n0"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		component := HSMComponent{
			ID:      "x1000c0s0b0n0",
			Type:    "Node",
			State:   "Ready",
			Enabled: true,
			Role:    "Compute",
			NID:     123,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(component) //nolint:errcheck
	}))
	defer server.Close()

	// Create client
	config := DefaultHSMConfig()
	config.BaseURL = server.URL

	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	client := NewHSMClient(config, logger)

	// Test getting specific component
	ctx := context.Background()
	component, err := client.GetComponent(ctx, "x1000c0s0b0n0")
	if err != nil {
		t.Fatalf("Failed to get component: %v", err)
	}

	if component.ID != "x1000c0s0b0n0" {
		t.Errorf("Expected ID x1000c0s0b0n0, got %s", component.ID)
	}

	if component.NID != 123 {
		t.Errorf("Expected NID 123, got %d", component.NID)
	}

	t.Logf("✅ Retrieved component %s from HSM", component.ID)
}

// TestHSMClient_GetEthernetInterfaces tests retrieving ethernet interfaces
func TestHSMClient_GetEthernetInterfaces(t *testing.T) {
	// Mock HSM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hsm/v2/Inventory/EthernetInterfaces" {
			t.Errorf("Expected path /hsm/v2/Inventory/EthernetInterfaces, got %s", r.URL.Path)
		}

		response := []HSMEthernetInterface{
			{
				MACAddress:  "00:1B:63:84:45:E6",
				ComponentID: "x1000c0s0b0n0",
				Type:        "Node",
			},
			{
				MACAddress:  "00:1B:63:84:45:E7",
				ComponentID: "x1000c0s0b0n1",
				Type:        "Node",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response) //nolint:errcheck
	}))
	defer server.Close()

	// Create client
	config := DefaultHSMConfig()
	config.BaseURL = server.URL

	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	client := NewHSMClient(config, logger)

	// Test getting ethernet interfaces
	ctx := context.Background()
	interfaces, err := client.GetEthernetInterfaces(ctx)
	if err != nil {
		t.Fatalf("Failed to get ethernet interfaces: %v", err)
	}

	if len(interfaces) != 2 {
		t.Errorf("Expected 2 interfaces, got %d", len(interfaces))
	}

	// Verify first interface
	if interfaces[0].MACAddress != "00:1B:63:84:45:E6" {
		t.Errorf("Expected MAC 00:1B:63:84:45:E6, got %s", interfaces[0].MACAddress)
	}

	if interfaces[0].ComponentID != "x1000c0s0b0n0" {
		t.Errorf("Expected ComponentID x1000c0s0b0n0, got %s", interfaces[0].ComponentID)
	}

	t.Logf("✅ Retrieved %d ethernet interfaces from HSM", len(interfaces))
}

// TestHSMClient_GetComponentByMAC tests finding a component by MAC address
func TestHSMClient_GetComponentByMAC(t *testing.T) {
	// Mock HSM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hsm/v2/Inventory/EthernetInterfaces":
			response := []HSMEthernetInterface{
				{
					MACAddress:  "00:1B:63:84:45:E6",
					ComponentID: "x1000c0s0b0n0",
					Type:        "Node",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response) //nolint:errcheck

		case "/hsm/v2/State/Components/x1000c0s0b0n0":
			component := HSMComponent{
				ID:      "x1000c0s0b0n0",
				Type:    "Node",
				State:   "Ready",
				Enabled: true,
				Role:    "Compute",
				NID:     123,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(component) //nolint:errcheck

		default:
			t.Errorf("Unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client
	config := DefaultHSMConfig()
	config.BaseURL = server.URL

	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	client := NewHSMClient(config, logger)

	// Test getting component by MAC
	ctx := context.Background()
	component, err := client.GetComponentByMAC(ctx, "00:1B:63:84:45:E6")
	if err != nil {
		t.Fatalf("Failed to get component by MAC: %v", err)
	}

	if component.ID != "x1000c0s0b0n0" {
		t.Errorf("Expected ID x1000c0s0b0n0, got %s", component.ID)
	}

	if component.NID != 123 {
		t.Errorf("Expected NID 123, got %d", component.NID)
	}

	t.Logf("✅ Found component %s by MAC address", component.ID)
}

// TestHSMClient_Cache tests caching functionality
func TestHSMClient_Cache(t *testing.T) {
	callCount := 0

	// Mock HSM server that tracks calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:revive
		callCount++

		response := HSMResponse{
			Components: []HSMComponent{
				{
					ID:      "x1000c0s0b0n0",
					Type:    "Node",
					State:   "Ready",
					Enabled: true,
					Role:    "Compute",
					NID:     123,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response) //nolint:errcheck
	}))
	defer server.Close()

	// Create client with short cache expiry
	config := DefaultHSMConfig()
	config.BaseURL = server.URL
	config.CacheExpiry = 200 * time.Millisecond

	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	client := NewHSMClient(config, logger)

	ctx := context.Background()

	// First call should hit the server
	_, err := client.GetComponents(ctx)
	if err != nil {
		t.Fatalf("Failed to get components: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 server call, got %d", callCount)
	}

	// Second call should use cache
	_, err = client.GetComponents(ctx)
	if err != nil {
		t.Fatalf("Failed to get components: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 server call (cached), got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(250 * time.Millisecond)

	// Third call should hit server again
	_, err = client.GetComponents(ctx)
	if err != nil {
		t.Fatalf("Failed to get components: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 server calls (cache expired), got %d", callCount)
	}

	t.Logf("✅ Cache working correctly: %d server calls for 3 requests", callCount)
}

// TestHSMClient_Health tests health check functionality
func TestHSMClient_Health(t *testing.T) {
	// Mock HSM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hsm/v2/service/ready" {
			t.Errorf("Expected path /hsm/v2/service/ready, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK")) //nolint:errcheck
	}))
	defer server.Close()

	// Create client
	config := DefaultHSMConfig()
	config.BaseURL = server.URL

	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	client := NewHSMClient(config, logger)

	// Test health check
	ctx := context.Background()
	err := client.Health(ctx)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	t.Logf("✅ HSM health check passed")
}

// TestHSMClient_ErrorHandling tests error scenarios
func TestHSMClient_ErrorHandling(t *testing.T) {
	// Mock HSM server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hsm/v2/State/Components/nonexistent" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create client
	config := DefaultHSMConfig()
	config.BaseURL = server.URL

	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	client := NewHSMClient(config, logger)

	ctx := context.Background()

	// Test 404 error
	_, err := client.GetComponent(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent component")
	}

	// Test 500 error
	_, err = client.GetComponents(ctx)
	if err == nil {
		t.Error("Expected error for server error")
	}

	t.Logf("✅ Error handling working correctly")
}

func TestHSMClient_AuthTokenProvider(t *testing.T) {
	var receivedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")

		response := HSMResponse{
			Components: []HSMComponent{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response) //nolint:errcheck
	}))
	defer server.Close()

	config := DefaultHSMConfig()
	config.BaseURL = server.URL
	config.AuthTokenProvider = func(context.Context) (string, error) {
		return "dynamic-token", nil
	}

	client := NewHSMClient(config, log.New(os.Stdout, "test: ", log.LstdFlags))

	if _, err := client.GetComponents(context.Background()); err != nil {
		t.Fatalf("GetComponents failed: %v", err)
	}

	if receivedAuthHeader != "Bearer dynamic-token" {
		t.Fatalf("expected Authorization header %q, got %q", "Bearer dynamic-token", receivedAuthHeader)
	}
}

func TestServiceTokenManager_GetTokenAndRefresh(t *testing.T) {
	var callCount int32
	var firstForm url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Fatalf("expected /oauth/token, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form body: %v", err)
		}

		if atomic.LoadInt32(&callCount) == 0 {
			firstForm = r.PostForm
		}

		count := atomic.AddInt32(&callCount, 1)
		resp := map[string]interface{}{
			"access_token":       "token-" + time.Now().Format("150405") + "-" + strings.Repeat("x", int(count)),
			"token_type":         "Bearer",
			"expires_in":         1,
			"refresh_token":      "refresh-" + time.Now().Format("150405") + "-" + strings.Repeat("r", int(count)),
			"refresh_expires_in": 600,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	cfg := DefaultTokenExchangeConfig()
	cfg.TokenSmithURL = server.URL
	cfg.BootstrapToken = "bootstrap-jwt"
	cfg.TargetService = "hsm"
	cfg.Scopes = []string{"hsm:read"}
	cfg.RefreshBefore = 900 * time.Millisecond

	manager := NewServiceTokenManager(cfg, log.New(os.Stdout, "test: ", log.LstdFlags))

	token1, err := manager.GetToken(context.Background())
	if err != nil {
		t.Fatalf("first GetToken failed: %v", err)
	}
	if token1 == "" {
		t.Fatal("expected non-empty token")
	}

	if firstForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:token-exchange" {
		t.Fatalf("expected token-exchange grant type, got %q", firstForm.Get("grant_type"))
	}
	if firstForm.Get("subject_token") != "bootstrap-jwt" {
		t.Fatalf("expected bootstrap token in subject_token, got %q", firstForm.Get("subject_token"))
	}
	if firstForm.Get("subject_token_type") != "urn:openchami:params:oauth:token-type:bootstrap-token" {
		t.Fatalf("expected bootstrap token type, got %q", firstForm.Get("subject_token_type"))
	}

	time.Sleep(450 * time.Millisecond)
	token2, err := manager.GetToken(context.Background())
	if err != nil {
		t.Fatalf("second GetToken failed: %v", err)
	}

	if token2 == token1 {
		t.Fatal("expected refreshed token to differ from first token")
	}

	if atomic.LoadInt32(&callCount) < 2 {
		t.Fatalf("expected at least 2 token exchange calls, got %d", atomic.LoadInt32(&callCount))
	}

	stats := manager.Stats()
	if stats["refresh_success_count"].(uint64) < 2 {
		t.Fatalf("expected refresh_success_count >= 2, got %v", stats["refresh_success_count"])
	}
	if stats["refresh_failure_count"].(uint64) != 0 {
		t.Fatalf("expected refresh_failure_count == 0, got %v", stats["refresh_failure_count"])
	}
}

func TestServiceTokenManager_InitializeRetriesThenSucceeds(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}

		resp := map[string]interface{}{
			"access_token":       "token-after-retry",
			"token_type":         "Bearer",
			"expires_in":         600,
			"refresh_token":      "refresh-after-retry",
			"refresh_expires_in": 86400,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	cfg := DefaultTokenExchangeConfig()
	cfg.TokenSmithURL = server.URL
	cfg.BootstrapToken = "bootstrap-jwt"
	cfg.BootstrapMaxAttempts = 5
	cfg.BootstrapInitialBackoff = 10 * time.Millisecond
	cfg.BootstrapMaxBackoff = 20 * time.Millisecond

	manager := NewServiceTokenManager(cfg, log.New(os.Stdout, "test: ", log.LstdFlags))

	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if atomic.LoadInt32(&callCount) != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", atomic.LoadInt32(&callCount))
	}

	stats := manager.Stats()
	if stats["refresh_failure_count"].(uint64) < 2 {
		t.Fatalf("expected refresh_failure_count >= 2, got %v", stats["refresh_failure_count"])
	}
}

func TestHSMClient_GetStatsIncludesAuthTokenStats(t *testing.T) {
	config := DefaultHSMConfig()
	config.AuthTokenProvider = func(context.Context) (string, error) { return "t", nil }
	config.AuthTokenStatsProvider = func() map[string]interface{} {
		return map[string]interface{}{"refresh_success_count": uint64(7)}
	}

	client := NewHSMClient(config, log.New(os.Stdout, "test: ", log.LstdFlags))
	stats := client.GetStats(context.Background())

	authStats, ok := stats["auth_token_stats"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected auth_token_stats map in stats, got %#v", stats["auth_token_stats"])
	}

	if authStats["refresh_success_count"].(uint64) != 7 {
		t.Fatalf("expected refresh_success_count 7, got %v", authStats["refresh_success_count"])
	}
}

func TestServiceTokenManager_ErrorIncludesEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := DefaultTokenExchangeConfig()
	cfg.TokenSmithURL = server.URL
	cfg.BootstrapToken = "bootstrap-jwt"
	cfg.BootstrapMaxAttempts = 1

	manager := NewServiceTokenManager(cfg, log.New(os.Stdout, "test: ", log.LstdFlags))
	err := manager.Initialize(context.Background())
	if err == nil {
		t.Fatal("expected initialize to fail")
	}

	if !strings.Contains(err.Error(), server.URL+"/oauth/token") {
		t.Fatalf("expected error to include endpoint, got: %v", err)
	}
}
