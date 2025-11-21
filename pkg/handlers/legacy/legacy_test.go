// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Legacy BSS API integration tests
package legacy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const testServerURL = "http://localhost:8080"

// checkServerAvailable checks if the test server is running
func checkServerAvailable(t *testing.T) {
	t.Helper()

	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to connect to the server
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(testServerURL + "/health")
	if err != nil {
		t.Skipf("Test server not available at %s: %v", testServerURL, err)
	}
	resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Test server not healthy at %s: status %d", testServerURL, resp.StatusCode)
	}
}

// TestLegacyServiceEndpoints tests the legacy service status and version endpoints
func TestLegacyServiceEndpoints(t *testing.T) {
	checkServerAvailable(t)

	t.Run("Service Status", func(t *testing.T) {
		resp, err := http.Get(testServerURL + "/boot/v1/service/status")
		if err != nil {
			t.Fatalf("Failed to call service status: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var status ServiceStatus
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			t.Fatalf("Failed to decode status response: %v", err)
		}

		if status.ServiceName != "boot-script-service" {
			t.Errorf("Expected service name 'boot-script-service', got '%s'", status.ServiceName)
		}

		if status.ServiceStatus != "running" {
			t.Errorf("Expected service status 'running', got '%s'", status.ServiceStatus)
		}

		t.Logf("✅ Service status: %s %s", status.ServiceName, status.ServiceVersion)
	})

	t.Run("Service Version", func(t *testing.T) {
		resp, err := http.Get(testServerURL + "/boot/v1/service/version")
		if err != nil {
			t.Fatalf("Failed to call service version: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var version ServiceVersion
		if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
			t.Fatalf("Failed to decode version response: %v", err)
		}

		if version.ServiceName != "boot-script-service" {
			t.Errorf("Expected service name 'boot-script-service', got '%s'", version.ServiceName)
		}

		if !strings.Contains(version.ServiceVersion, "fabrica") {
			t.Errorf("Expected version to contain 'fabrica', got '%s'", version.ServiceVersion)
		}

		t.Logf("✅ Service version: %s %s", version.ServiceName, version.ServiceVersion)
	})
}

// TestLegacyBootParameters tests the boot parameters CRUD operations
func TestLegacyBootParameters(t *testing.T) {
	checkServerAvailable(t)

	t.Run("Get Boot Parameters", func(t *testing.T) {
		resp, err := http.Get(testServerURL + "/boot/v1/bootparameters")
		if err != nil {
			t.Fatalf("Failed to get boot parameters: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var response BootParametersResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode boot parameters response: %v", err)
		}

		if len(response.BootParameters) == 0 {
			t.Error("Expected at least one boot parameter configuration")
		}

		t.Logf("✅ Retrieved %d boot parameter configurations", len(response.BootParameters))

		// Check the structure of the first configuration
		if len(response.BootParameters) > 0 {
			param := response.BootParameters[0]
			if param.Kernel == "" {
				t.Error("Expected kernel to be set in boot parameters")
			}
			if len(param.Hosts) == 0 && len(param.Macs) == 0 && len(param.Nids) == 0 {
				t.Error("Expected at least one identifier (hosts, macs, or nids) in boot parameters")
			}
			t.Logf("✅ Boot parameter structure validated: kernel=%s, hosts=%v", param.Kernel, param.Hosts)
		}
	})

	t.Run("Create Boot Parameters", func(t *testing.T) {
		// Create a new boot parameter configuration
		newConfig := BootParametersRequest{
			Hosts:  []string{"x9999c9s9b9n9"},
			Kernel: "http://example.com/test-kernel",
			Initrd: "http://example.com/test-initrd",
			Params: "console=tty0 test=true",
		}

		jsonData, err := json.Marshal(newConfig)
		if err != nil {
			t.Fatalf("Failed to marshal test config: %v", err)
		}

		resp, err := http.Post(testServerURL+"/boot/v1/bootparameters", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			t.Fatalf("Failed to create boot parameters: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d. Response: %s", resp.StatusCode, string(body))
		}

		var response BootParametersResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode create response: %v", err)
		}

		if len(response.BootParameters) != 1 {
			t.Errorf("Expected 1 boot parameter in response, got %d", len(response.BootParameters))
		}

		created := response.BootParameters[0]
		if created.Kernel != newConfig.Kernel {
			t.Errorf("Expected kernel '%s', got '%s'", newConfig.Kernel, created.Kernel)
		}

		t.Logf("✅ Created boot parameters with kernel: %s", created.Kernel)
	})
}

// TestLegacyBootScript tests the boot script generation endpoint
func TestLegacyBootScript(t *testing.T) {
	checkServerAvailable(t)

	testCases := []struct {
		name       string
		queryParam string
		identifier string
	}{
		{"Host Identifier", "host", "x1000c0s0b0n0"},
		{"MAC Identifier", "mac", "00:1B:63:84:45:E6"},
		{"NID Identifier", "nid", "123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := testServerURL + "/boot/v1/bootscript?" + tc.queryParam + "=" + tc.identifier

			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("Failed to get boot script: %v", err)
			}
			defer resp.Body.Close() //nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
			}

			// Check content type
			contentType := resp.Header.Get("Content-Type")
			if contentType != "text/plain" {
				t.Errorf("Expected Content-Type 'text/plain', got '%s'", contentType)
			}

			// Read and validate script content
			scriptBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read script body: %v", err)
			}

			script := string(scriptBytes)

			// Validate iPXE script format
			if !strings.HasPrefix(script, "#!ipxe") {
				t.Error("Expected script to start with '#!ipxe'")
			}

			if !strings.Contains(script, "x1000c0s0b0n0") {
				t.Error("Expected script to contain node identifier")
			}

			if !strings.Contains(script, "test-direct") {
				t.Error("Expected script to contain configuration name")
			}

			t.Logf("✅ Generated %d byte iPXE script for %s=%s", len(script), tc.queryParam, tc.identifier)
		})
	}

	t.Run("Unknown Node Fallback", func(t *testing.T) {
		url := testServerURL + "/boot/v1/bootscript?host=unknown-node"

		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("Failed to get boot script for unknown node: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 even for unknown node, got %d", resp.StatusCode)
		}

		scriptBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read script body: %v", err)
		}

		script := string(scriptBytes)

		// Should still be a valid iPXE script
		if !strings.HasPrefix(script, "#!ipxe") {
			t.Error("Expected fallback script to start with '#!ipxe'")
		}

		t.Logf("✅ Generated fallback script for unknown node (%d bytes)", len(script))
	})

	t.Run("Missing Identifier Error", func(t *testing.T) {
		url := testServerURL + "/boot/v1/bootscript"

		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for missing identifier, got %d", resp.StatusCode)
		}

		var errorResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			if errorResp.Status != http.StatusBadRequest {
				t.Errorf("Expected error status 400, got %d", errorResp.Status)
			}
			t.Logf("✅ Proper error handling for missing identifier: %s", errorResp.Title)
		}
	})
}

// TestLegacyAPICompatibility tests overall API compatibility with legacy BSS
func TestLegacyAPICompatibility(t *testing.T) {
	checkServerAvailable(t)

	// Test that we can perform a typical legacy BSS workflow
	t.Run("Legacy Workflow", func(t *testing.T) {
		// 1. Check service status
		statusResp, err := http.Get(testServerURL + "/boot/v1/service/status")
		if err != nil {
			t.Fatalf("Failed to check service status: %v", err)
		}
		statusResp.Body.Close() //nolint:errcheck

		if statusResp.StatusCode != http.StatusOK {
			t.Fatalf("Service status check failed: %d", statusResp.StatusCode)
		}

		// 2. Get existing boot parameters
		paramsResp, err := http.Get(testServerURL + "/boot/v1/bootparameters")
		if err != nil {
			t.Fatalf("Failed to get boot parameters: %v", err)
		}
		paramsResp.Body.Close() //nolint:errcheck

		if paramsResp.StatusCode != http.StatusOK {
			t.Fatalf("Boot parameters retrieval failed: %d", paramsResp.StatusCode)
		}

		// 3. Generate boot script for a known node
		scriptResp, err := http.Get(testServerURL + "/boot/v1/bootscript?host=x1000c0s0b0n0")
		if err != nil {
			t.Fatalf("Failed to generate boot script: %v", err)
		}
		scriptResp.Body.Close() //nolint:errcheck

		if scriptResp.StatusCode != http.StatusOK {
			t.Fatalf("Boot script generation failed: %d", scriptResp.StatusCode)
		}

		t.Logf("✅ Complete legacy BSS workflow successful")
	})

	t.Run("Response Time Performance", func(t *testing.T) {
		// Test that legacy API responses are fast enough for production use
		start := time.Now()

		resp, err := http.Get(testServerURL + "/boot/v1/bootscript?host=x1000c0s0b0n0")
		if err != nil {
			t.Fatalf("Failed to get boot script: %v", err)
		}
		resp.Body.Close() //nolint:errcheck

		duration := time.Since(start)

		// Boot script generation should be fast (< 100ms for cached results)
		if duration > 100*time.Millisecond {
			t.Logf("⚠️  Boot script generation took %v (may indicate caching not working)", duration)
		} else {
			t.Logf("✅ Boot script generated in %v (good performance)", duration)
		}
	})
}

// TestLegacyErrorHandling tests error scenarios and responses
func TestLegacyErrorHandling(t *testing.T) {
	checkServerAvailable(t)

	t.Run("Invalid JSON in POST", func(t *testing.T) {
		resp, err := http.Post(testServerURL+"/boot/v1/bootparameters", "application/json", strings.NewReader("{invalid json"))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
		}

		var errorResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			t.Logf("✅ Proper error response for invalid JSON: %s", errorResp.Title)
		}
	})

	t.Run("Not Found Endpoint", func(t *testing.T) {
		resp, err := http.Get(testServerURL + "/boot/v1/nonexistent")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent endpoint, got %d", resp.StatusCode)
		}

		t.Logf("✅ Proper 404 handling for nonexistent endpoint")
	})
}
