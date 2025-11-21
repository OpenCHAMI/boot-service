// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Integration tests for HSM-enhanced boot script controller
package bootscript

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/clients/hsm"
)

// TestEnhancedController_HSMIntegration tests HSM integration functionality
func TestEnhancedController_HSMIntegration(t *testing.T) {
	// Skip if no real boot service available
	if testing.Short() {
		t.Skip("Skipping HSM integration test in short mode")
	}

	// Check if boot service is available
	httpClient := &http.Client{Timeout: 2 * time.Second}
	resp, err := httpClient.Get("http://localhost:8080/health")
	if err != nil {
		t.Skipf("Boot service not available at localhost:8080: %v", err)
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Skipf("Boot service not healthy: status %d", resp.StatusCode)
	}

	// Mock HSM server
	hsmServer := createMockHSMServer(t)
	defer hsmServer.Close()

	// Create HSM config pointing to mock server
	hsmConfig := hsm.DefaultIntegrationConfig()
	hsmConfig.HSMConfig.BaseURL = hsmServer.URL
	hsmConfig.HSMConfig.CacheExpiry = 100 * time.Millisecond
	hsmConfig.SyncEnabled = false // Disable auto-sync for testing

	// Create boot service client (assuming live server)
	bootClient, err := client.NewClient("http://localhost:8080", &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Failed to create boot client: %v", err)
	}

	// Create enhanced controller
	logger := log.New(os.Stdout, "hsm-test: ", log.LstdFlags)
	controller := NewEnhancedBootScriptController(*bootClient, hsmConfig, logger)

	ctx := context.Background()

	t.Run("HSM Health Check", func(t *testing.T) {
		err := controller.HealthCheck(ctx)
		if err != nil {
			t.Errorf("HSM health check failed: %v", err)
		} else {
			t.Logf("✅ HSM health check passed")
		}
	})

	t.Run("HSM Stats", func(t *testing.T) {
		stats, err := controller.GetHSMStats(ctx)
		if err != nil {
			t.Errorf("Failed to get HSM stats: %v", err)
			return
		}

		// Check for the actual keys returned by GetHSMStats
		if enabled, ok := stats["hsm_integration_enabled"].(bool); !ok || !enabled {
			t.Error("HSM integration not reported as enabled")
		}

		if _, ok := stats["sync_enabled"].(bool); !ok {
			t.Error("sync_enabled key missing from stats")
		}

		if _, ok := stats["hsm_client_stats"]; !ok {
			t.Error("hsm_client_stats key missing from stats")
		}

		t.Logf("✅ HSM stats: %+v", stats)
	})

	t.Run("Manual HSM Sync", func(t *testing.T) {
		err := controller.SyncFromHSM(ctx)
		if err != nil {
			t.Errorf("HSM sync failed: %v", err)
		} else {
			t.Logf("✅ HSM sync completed successfully")
		}
	})

	t.Run("Boot Script with HSM Fallback", func(t *testing.T) {
		// Test with a node that might only exist in HSM
		script, err := controller.GenerateBootScriptWithHSM(ctx, "x2000c0s0b0n0")
		if err != nil {
			t.Errorf("Failed to generate boot script with HSM fallback: %v", err)
			return
		}

		if len(script) == 0 {
			t.Error("Generated script is empty")
			return
		}

		// Should be iPXE format
		if script[:6] != "#!ipxe" {
			t.Errorf("Expected iPXE script, got: %s", script[:20])
		}

		t.Logf("✅ Generated boot script with HSM fallback (%d bytes)", len(script))
	})

	t.Run("Performance with HSM", func(t *testing.T) {
		// Test performance impact of HSM integration
		start := time.Now()

		for i := 0; i < 5; i++ {
			_, err := controller.GenerateBootScriptWithHSM(ctx, "x1000c0s0b0n0")
			if err != nil {
				t.Errorf("Script generation failed on iteration %d: %v", i, err)
			}
		}

		duration := time.Since(start)
		avgTime := duration / 5

		// Should still be fast with caching
		if avgTime > 100*time.Millisecond {
			t.Logf("⚠️  Average time per script generation: %v (may indicate caching issues)", avgTime)
		} else {
			t.Logf("✅ Good performance: average %v per script generation", avgTime)
		}
	})
}

// createMockHSMServer creates a mock HSM server for testing
func createMockHSMServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hsm/v2/service/ready":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK")) //nolint:errcheck

		case "/hsm/v2/State/Components":
			response := hsm.HSMResponse{
				Components: []hsm.HSMComponent{
					{
						ID:      "x1000c0s0b0n0",
						Type:    "Node",
						State:   "Ready",
						Enabled: true,
						Role:    "Compute",
						NID:     123,
					},
					{
						ID:      "x2000c0s0b0n0",
						Type:    "Node",
						State:   "Ready",
						Enabled: true,
						Role:    "Compute",
						NID:     456,
					},
					{
						ID:      "x1000c0r0e0",
						Type:    "RouterModule",
						State:   "Ready",
						Enabled: true,
						Role:    "Service",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response) //nolint:errcheck

		case "/hsm/v2/State/Components/x2000c0s0b0n0":
			component := hsm.HSMComponent{
				ID:      "x2000c0s0b0n0",
				Type:    "Node",
				State:   "Ready",
				Enabled: true,
				Role:    "Compute",
				NID:     456,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(component) //nolint:errcheck

		case "/hsm/v2/Inventory/EthernetInterfaces":
			response := hsm.HSMEthernetResponse{
				EthernetInterfaces: []hsm.HSMEthernetInterface{
					{
						MACAddress:  "00:1B:63:84:45:E6",
						ComponentID: "x1000c0s0b0n0",
						Type:        "Node",
					},
					{
						MACAddress:  "00:1B:63:84:45:F0",
						ComponentID: "x2000c0s0b0n0",
						Type:        "Node",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response) //nolint:errcheck

		default:
			t.Logf("Unhandled HSM path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

// TestEnhancedController_HSMSyncWorker tests the HSM sync background worker
func TestEnhancedController_HSMSyncWorker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HSM sync worker test in short mode")
	}

	// Check if boot service is available
	httpClient := &http.Client{Timeout: 2 * time.Second}
	resp, err := httpClient.Get("http://localhost:8080/health")
	if err != nil {
		t.Skipf("Boot service not available at localhost:8080: %v", err)
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Skipf("Boot service not healthy: status %d", resp.StatusCode)
	}

	// Mock HSM server
	hsmServer := createMockHSMServer(t)
	defer hsmServer.Close()

	// Create HSM config with fast sync for testing
	hsmConfig := hsm.DefaultIntegrationConfig()
	hsmConfig.HSMConfig.BaseURL = hsmServer.URL
	hsmConfig.SyncEnabled = true
	hsmConfig.SyncInterval = 200 * time.Millisecond

	// Create boot service client
	bootClient, err := client.NewClient("http://localhost:8080", &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Failed to create boot client: %v", err)
	}

	// Create enhanced controller
	logger := log.New(os.Stdout, "sync-test: ", log.LstdFlags)
	controller := NewEnhancedBootScriptController(*bootClient, hsmConfig, logger)

	// Start sync worker with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start sync worker in background
	go controller.StartHSMSync(ctx)

	// Wait for a couple sync cycles
	time.Sleep(500 * time.Millisecond)

	// Check that stats show HSM is working
	stats, err := controller.GetHSMStats(context.Background())
	if err != nil {
		t.Errorf("Failed to get HSM stats: %v", err)
		return
	}

	if enabled, ok := stats["hsm_integration_enabled"].(bool); !ok || !enabled {
		t.Error("HSM integration not enabled after sync worker start")
	}

	t.Logf("✅ HSM sync worker test completed")
}

// BenchmarkEnhancedController_WithHSM benchmarks the enhanced controller
func BenchmarkEnhancedController_WithHSM(b *testing.B) {
	// Mock HSM server
	hsmServer := createMockHSMServer(&testing.T{})
	defer hsmServer.Close()

	// Create HSM config
	hsmConfig := hsm.DefaultIntegrationConfig()
	hsmConfig.HSMConfig.BaseURL = hsmServer.URL
	hsmConfig.SyncEnabled = false

	// Create boot service client
	bootClient, err := client.NewClient("http://localhost:8080", &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		b.Fatalf("Failed to create boot client: %v", err)
	}

	// Create enhanced controller
	logger := log.New(os.Stdout, "bench: ", log.LstdFlags)
	controller := NewEnhancedBootScriptController(*bootClient, hsmConfig, logger)

	ctx := context.Background()

	b.ResetTimer()

	// Benchmark script generation with HSM fallback
	for i := 0; i < b.N; i++ {
		_, err := controller.GenerateBootScriptWithHSM(ctx, "x1000c0s0b0n0")
		if err != nil {
			b.Fatalf("Script generation failed: %v", err)
		}
	}
}
