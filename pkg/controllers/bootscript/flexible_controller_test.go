// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Tests for flexible boot script controller with YAML and HSM providers
package bootscript

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/clients/hsm"
	"github.com/openchami/boot-service/pkg/clients/local"
)

// createTestYAMLFile creates a temporary YAML file for testing
func createTestYAMLFile(t *testing.T) string {
	content := `
version: "1.0"
nodes:
  - id: "x1000c0s0b0n0"
    xname: "x1000c0s0b0n0"
    type: "Node"
    role: "Compute"
    subrole: "Worker"
    state: "Ready"
    enabled: true
    nid: 123
    boot_mac: "00:1B:63:84:45:E6"
    ethernet_interfaces:
      - mac_address: "00:1B:63:84:45:E6"
        ip_address: "10.1.1.123"
        description: "Primary interface"
    metadata:
      cluster: "test-cluster"
      rack: "rack1"

  - id: "x2000c0s0b0n0" 
    xname: "x2000c0s0b0n0"
    type: "Node"
    role: "Compute"
    subrole: "Worker"
    state: "Ready"
    enabled: true
    nid: 456
    boot_mac: "00:1B:63:84:45:F0"
    ethernet_interfaces:
      - mac_address: "00:1B:63:84:45:F0"
        ip_address: "10.1.1.456"

  - id: "x1000c0r0e0"
    xname: "x1000c0r0e0"
    type: "RouterModule"
    role: "Service"
    state: "Ready"
    enabled: true
    metadata:
      location: "datacenter1"
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_nodes.yaml")

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}

	return filePath
}

// createMockBootService creates a mock boot service for testing
func createMockBootService(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock boot service always returns "not found" so fallback is tested
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "node not found"}`))
	}))
}

// createMockHSMService creates a mock HSM service for testing
func createMockHSMService(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hsm/v2/service/ready":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))

		case "/hsm/v2/State/Components":
			response := hsm.HSMResponse{
				Components: []hsm.HSMComponent{
					{
						ID:      "x3000c0s0b0n0",
						Type:    "Node",
						State:   "Ready",
						Enabled: true,
						Role:    "Compute",
						NID:     789,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case "/hsm/v2/State/Components/x3000c0s0b0n0":
			component := hsm.HSMComponent{
				ID:      "x3000c0s0b0n0",
				Type:    "Node",
				State:   "Ready",
				Enabled: true,
				Role:    "Compute",
				NID:     789,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(component)

		default:
			t.Logf("Unhandled HSM path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func TestFlexibleController_YAMLProvider(t *testing.T) {
	// Create test YAML file
	yamlFile := createTestYAMLFile(t)

	// Create mock boot service
	bootServer := createMockBootService(t)
	defer bootServer.Close()

	// Create boot client
	bootClient, err := client.NewClient(bootServer.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Failed to create boot client: %v", err)
	}

	// Create YAML config
	yamlConfig := local.IntegrationConfig{
		YAMLFile:     yamlFile,
		AutoReload:   true,
		SyncEnabled:  false,
		SyncInterval: 1 * time.Minute,
	}

	// Create flexible controller with YAML provider
	logger := log.New(os.Stdout, "yaml-test: ", log.LstdFlags)
	controller := NewYAMLController(*bootClient, yamlConfig, logger)
	if controller == nil {
		t.Fatal("Failed to create YAML controller")
	}

	ctx := context.Background()

	t.Run("Provider Type", func(t *testing.T) {
		if controller.GetProviderType() != "yaml" {
			t.Errorf("Expected provider type 'yaml', got '%s'", controller.GetProviderType())
		}
	})

	t.Run("Health Check", func(t *testing.T) {
		err := controller.HealthCheck(ctx)
		if err != nil {
			t.Errorf("YAML health check failed: %v", err)
		}
	})

	t.Run("Provider Stats", func(t *testing.T) {
		stats := controller.GetProviderStats(ctx)

		if stats["provider_type"].(string) != "yaml" {
			t.Error("Provider type not set correctly in stats")
		}

		if !stats["provider_configured"].(bool) {
			t.Error("Provider should be configured")
		}

		if stats["total_nodes"].(int) != 3 {
			t.Errorf("Expected 3 nodes, got %d", stats["total_nodes"].(int))
		}

		t.Logf("YAML provider stats: %+v", stats)
	})

	t.Run("Boot Script Generation with YAML Fallback", func(t *testing.T) {
		// Test with node that exists in YAML
		script, err := controller.GenerateBootScriptWithFallback(ctx, "x1000c0s0b0n0")
		if err != nil {
			t.Errorf("Failed to generate boot script with YAML fallback: %v", err)
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

		t.Logf("✅ Generated boot script with YAML fallback (%d bytes)", len(script))
	})

	t.Run("Boot Script Generation by MAC", func(t *testing.T) {
		// Test with MAC address from YAML
		script, err := controller.GenerateBootScriptWithFallback(ctx, "00:1B:63:84:45:F0")
		if err != nil {
			t.Errorf("Failed to generate boot script by MAC: %v", err)
			return
		}

		if len(script) == 0 {
			t.Error("Generated script is empty")
		}

		t.Logf("✅ Generated boot script by MAC address (%d bytes)", len(script))
	})
}

func TestFlexibleController_HSMProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HSM provider test in short mode")
	}

	// Create mock services
	bootServer := createMockBootService(t)
	defer bootServer.Close()

	hsmServer := createMockHSMService(t)
	defer hsmServer.Close()

	// Create boot client
	bootClient, err := client.NewClient(bootServer.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Failed to create boot client: %v", err)
	}

	// Create HSM config
	hsmConfig := hsm.DefaultIntegrationConfig()
	hsmConfig.HSMConfig.BaseURL = hsmServer.URL
	hsmConfig.SyncEnabled = false

	// Create flexible controller with HSM provider
	logger := log.New(os.Stdout, "hsm-test: ", log.LstdFlags)
	controller := NewHSMController(*bootClient, hsmConfig, logger)
	if controller == nil {
		t.Fatal("Failed to create HSM controller")
	}

	ctx := context.Background()

	t.Run("Provider Type", func(t *testing.T) {
		if controller.GetProviderType() != "hsm" {
			t.Errorf("Expected provider type 'hsm', got '%s'", controller.GetProviderType())
		}
	})

	t.Run("Health Check", func(t *testing.T) {
		err := controller.HealthCheck(ctx)
		if err != nil {
			t.Errorf("HSM health check failed: %v", err)
		}
	})

	t.Run("Provider Stats", func(t *testing.T) {
		stats := controller.GetProviderStats(ctx)

		if stats["provider_type"].(string) != "hsm" {
			t.Error("Provider type not set correctly in stats")
		}

		if !stats["provider_configured"].(bool) {
			t.Error("Provider should be configured")
		}

		if !stats["sync_supported"].(bool) {
			t.Error("HSM provider should support sync")
		}

		t.Logf("HSM provider stats: %+v", stats)
	})

	t.Run("Boot Script Generation with HSM Fallback", func(t *testing.T) {
		// Test with node that exists in HSM
		script, err := controller.GenerateBootScriptWithFallback(ctx, "x3000c0s0b0n0")
		if err != nil {
			t.Errorf("Failed to generate boot script with HSM fallback: %v", err)
			return
		}

		if len(script) == 0 {
			t.Error("Generated script is empty")
			return
		}

		t.Logf("✅ Generated boot script with HSM fallback (%d bytes)", len(script))
	})
}

func TestFlexibleController_ProviderComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping provider comparison test in short mode")
	}

	// Create test files and servers
	yamlFile := createTestYAMLFile(t)
	bootServer := createMockBootService(t)
	defer bootServer.Close()
	hsmServer := createMockHSMService(t)
	defer hsmServer.Close()

	// Create boot client
	bootClient, err := client.NewClient(bootServer.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Failed to create boot client: %v", err)
	}

	// Create both controllers
	yamlConfig := local.IntegrationConfig{
		YAMLFile:    yamlFile,
		AutoReload:  true,
		SyncEnabled: false,
	}

	hsmConfig := hsm.DefaultIntegrationConfig()
	hsmConfig.HSMConfig.BaseURL = hsmServer.URL
	hsmConfig.SyncEnabled = false

	logger := log.New(os.Stdout, "comparison-test: ", log.LstdFlags)

	yamlController := NewYAMLController(*bootClient, yamlConfig, logger)
	hsmController := NewHSMController(*bootClient, hsmConfig, logger)

	ctx := context.Background()

	t.Run("Provider Types", func(t *testing.T) {
		if yamlController.GetProviderType() != "yaml" {
			t.Error("YAML controller has wrong provider type")
		}
		if hsmController.GetProviderType() != "hsm" {
			t.Error("HSM controller has wrong provider type")
		}
	})

	t.Run("Performance Comparison", func(t *testing.T) {
		// Test YAML provider performance
		start := time.Now()
		for i := 0; i < 5; i++ {
			_, err := yamlController.GenerateBootScriptWithFallback(ctx, "x1000c0s0b0n0")
			if err != nil {
				t.Errorf("YAML script generation failed: %v", err)
			}
		}
		yamlDuration := time.Since(start)

		// Test HSM provider performance
		start = time.Now()
		for i := 0; i < 5; i++ {
			_, err := hsmController.GenerateBootScriptWithFallback(ctx, "x3000c0s0b0n0")
			if err != nil {
				t.Errorf("HSM script generation failed: %v", err)
			}
		}
		hsmDuration := time.Since(start)

		t.Logf("YAML provider: %v for 5 requests (avg: %v)", yamlDuration, yamlDuration/5)
		t.Logf("HSM provider: %v for 5 requests (avg: %v)", hsmDuration, hsmDuration/5)

		// YAML should generally be faster since it's local
		if yamlDuration > hsmDuration*2 {
			t.Logf("⚠️  YAML provider seems slower than expected compared to HSM")
		}
	})
}

// BenchmarkFlexibleController_YAML benchmarks the YAML provider
func BenchmarkFlexibleController_YAML(b *testing.B) {
	yamlFile := createTestYAMLFile(&testing.T{})
	bootServer := createMockBootService(&testing.T{})
	defer bootServer.Close()

	bootClient, _ := client.NewClient(bootServer.URL, &http.Client{Timeout: 5 * time.Second})
	yamlConfig := local.IntegrationConfig{
		YAMLFile:    yamlFile,
		AutoReload:  false, // Disable auto-reload for benchmarking
		SyncEnabled: false,
	}

	logger := log.New(os.Stdout, "bench: ", log.LstdFlags)
	controller := NewYAMLController(*bootClient, yamlConfig, logger)

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := controller.GenerateBootScriptWithFallback(ctx, "x1000c0s0b0n0")
		if err != nil {
			b.Fatalf("Script generation failed: %v", err)
		}
	}
}
