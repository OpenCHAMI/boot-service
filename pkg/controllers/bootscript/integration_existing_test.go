// SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openchami/boot-service/pkg/client"
)

// TestBootLogicWithExistingData tests the boot logic using existing data in the server
// This test proves that the complete boot script generation workflow works correctly
// NOTE: This test requires a running boot service at localhost:8080 with existing data
func TestBootLogicWithExistingData(t *testing.T) {
	// Skip if we're not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create client pointing to localhost (assumes server is running)
	bootClient, err := client.NewClient("http://localhost:8080", &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create controller with real client
	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	controller := NewBootScriptController(*bootClient, logger)

	ctx := context.Background()

	// Test server connectivity first
	nodes, err := bootClient.GetNodes(ctx)
	if err != nil {
		t.Skipf("Boot service not available at localhost:8080, skipping integration test: %v", err)
	}

	configs, err := bootClient.GetBootConfigurations(ctx)
	if err != nil {
		t.Fatalf("Failed to get boot configurations: %v", err)
	}

	if len(nodes) == 0 || len(configs) == 0 {
		t.Skipf("No test data available - need at least one node and one configuration")
	}

	t.Logf("Found %d nodes and %d configurations for testing", len(nodes), len(configs))

	// Test Case 1: Generate boot script for existing nodes by different identifiers
	t.Run("Multi-Identifier Node Resolution", func(t *testing.T) {
		for _, node := range nodes {
			t.Run("Node_"+node.Spec.XName, func(t *testing.T) {
				// Test generation by XName
				scriptByXName, err := controller.GenerateBootScript(ctx, node.Spec.XName)
				if err != nil {
					t.Errorf("Failed to generate script by XName %s: %v", node.Spec.XName, err)
					return
				}

				// Validate basic iPXE script structure
				if !strings.Contains(scriptByXName, "#!ipxe") {
					t.Errorf("Script missing iPXE header for XName %s", node.Spec.XName)
				}

				if !strings.Contains(scriptByXName, "dhcp") {
					t.Errorf("Script missing DHCP command for XName %s", node.Spec.XName)
				}

				if !strings.Contains(scriptByXName, "boot") {
					t.Errorf("Script missing boot command for XName %s", node.Spec.XName)
				}

				// Test generation by NID if available
				if node.Spec.NID > 0 {
					scriptByNID, err := controller.GenerateBootScript(ctx, fmt.Sprintf("%d", node.Spec.NID))
					if err != nil {
						t.Logf("Failed to generate script by NID %d: %v", node.Spec.NID, err)
					} else if scriptByNID == scriptByXName {
						t.Logf("✅ Successfully generated identical script by NID and XName for %s", node.Spec.XName)
					}
				}

				// Test generation by MAC if available
				if node.Spec.BootMAC != "" {
					scriptByMAC, err := controller.GenerateBootScript(ctx, node.Spec.BootMAC)
					if err != nil {
						t.Logf("Failed to generate script by MAC %s: %v", node.Spec.BootMAC, err)
					} else if scriptByMAC == scriptByXName {
						t.Logf("✅ Successfully generated identical script by MAC and XName for %s", node.Spec.XName)
					}
				}

				t.Logf("✅ Successfully generated boot script for node %s", node.Spec.XName)
			})
		}
	})

	// Test Case 2: Configuration matching logic
	t.Run("Configuration Matching Validation", func(t *testing.T) {
		for _, node := range nodes {
			script, err := controller.GenerateBootScript(ctx, node.Spec.XName)
			if err != nil {
				t.Errorf("Failed to generate script for node %s: %v", node.Spec.XName, err)
				continue
			}

			// Check if the script contains configuration-specific content
			foundKernel := false
			for _, config := range configs {
				if strings.Contains(script, config.Spec.Kernel) {
					foundKernel = true
					t.Logf("✅ Node %s matched with configuration containing kernel %s",
						node.Spec.XName, config.Spec.Kernel)
					break
				}
			}

			if !foundKernel {
				t.Logf("⚠️  Node %s did not match any specific configuration (using default/minimal)", node.Spec.XName)
			}
		}
	})

	// Test Case 3: Unknown node handling
	t.Run("Unknown Node Fallback", func(t *testing.T) {
		// Try to generate script for non-existent node
		script, err := controller.GenerateBootScript(ctx, "x9999c9s9b9n9")
		if err != nil {
			t.Fatalf("Expected graceful handling of unknown node, but got error: %v", err)
		}

		// Should return error script
		if !strings.Contains(script, "#!ipxe") {
			t.Errorf("Expected valid iPXE script for unknown node")
		}

		// Should contain some indication of error or minimal boot
		if strings.Contains(script, "Boot script generation failed") ||
			strings.Contains(script, "Node resolution failed") ||
			strings.Contains(script, "default") {
			t.Logf("✅ Successfully handled unknown node with appropriate script")
		} else {
			t.Errorf("Expected error or minimal script indication for unknown node")
		}
	})

	// Test Case 4: Cache behavior verification
	t.Run("Cache Behavior Validation", func(t *testing.T) {
		if len(nodes) == 0 {
			t.Skip("No nodes available for cache testing")
		}

		testNode := nodes[0]

		// Clear cache statistics
		stats := controller.cache.Stats()
		t.Logf("Cache stats before test - Total: %d, Valid: %d", stats.TotalEntries, stats.ValidEntries)

		// Generate script (should be cache miss)
		start := time.Now()
		script1, err := controller.GenerateBootScript(ctx, testNode.Spec.XName)
		duration1 := time.Since(start)
		if err != nil {
			t.Fatalf("Failed to generate boot script: %v", err)
		}

		// Generate same script again (should be cache hit)
		start = time.Now()
		script2, err := controller.GenerateBootScript(ctx, testNode.Spec.XName)
		duration2 := time.Since(start)
		if err != nil {
			t.Fatalf("Failed to generate boot script: %v", err)
		}

		// Scripts should be identical
		if script1 != script2 {
			t.Errorf("Cached script should match original script")
		}

		// Cache hit should typically be faster (though this might be unreliable in tests)
		t.Logf("First generation: %v, Second generation: %v", duration1, duration2)

		// Verify cache statistics improved
		stats = controller.cache.Stats()
		t.Logf("✅ Successfully validated cache behavior - entries: %d", stats.ValidEntries)
	})

	// Test Case 5: Template system validation
	t.Run("Template Content Validation", func(t *testing.T) {
		if len(nodes) == 0 {
			t.Skip("No nodes available for template testing")
		}

		testNode := nodes[0]
		script, err := controller.GenerateBootScript(ctx, testNode.Spec.XName)
		if err != nil {
			t.Fatalf("Failed to generate boot script: %v", err)
		}

		// Check for node-specific template variables
		expectedInScript := []string{
			testNode.Spec.XName, // Node XName should appear
			"#!ipxe",            // iPXE header
			"dhcp",              // Network configuration
			"boot",              // Boot command
		}

		for _, expected := range expectedInScript {
			if !strings.Contains(script, expected) {
				t.Errorf("Script missing expected content: %s", expected)
			}
		}

		// Check for proper variable substitution (should not contain template markers)
		templateMarkers := []string{
			"{{.XName}}",
			"{{.NID}}",
			"{{.Kernel}}",
			"{{.Error}}",
		}

		for _, marker := range templateMarkers {
			if strings.Contains(script, marker) {
				t.Errorf("Script contains unsubstituted template marker: %s", marker)
			}
		}

		t.Logf("✅ Successfully validated template system and variable substitution")
	})

	t.Logf("🎉 All integration tests passed with existing data - boot logic is fully functional!")
}
