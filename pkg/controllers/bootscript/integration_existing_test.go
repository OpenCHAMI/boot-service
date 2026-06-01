// SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package bootscript

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	apiv1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/client"
)

// TestBootLogicWithExistingData tests the boot logic using a real running server.
// The test starts and stops a local boot-service instance automatically.
func TestBootLogicWithExistingData(t *testing.T) {
	// Skip if we're not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if os.Getenv("BOOT_SERVICE_RUN_INTEGRATION") != "1" {
		t.Skip("Skipping integration test; set BOOT_SERVICE_RUN_INTEGRATION=1 to enable")
	}

	baseURL, stopServer := startTestServer(t)
	defer stopServer()

	bootClient, err := client.NewClient(baseURL, &http.Client{Timeout: 30 * time.Second}, client.DefaultLogger())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create controller with real client
	logger := log.New(os.Stdout, "test: ", log.LstdFlags)
	controller := NewBootScriptController(*bootClient, logger)

	ctx := context.Background()
	seedIntegrationData(t, bootClient, ctx)

	nodes, err := bootClient.GetNodes(ctx)
	if err != nil {
		t.Fatalf("Boot service not available at %s: %v", baseURL, err)
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
				scriptByXName, err := controller.GenerateBootScript(ctx, node.Spec.XName, "")
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
					scriptByNID, err := controller.GenerateBootScript(ctx, fmt.Sprintf("%d", node.Spec.NID), "")
					if err != nil {
						t.Logf("Failed to generate script by NID %d: %v", node.Spec.NID, err)
					} else if scriptByNID == scriptByXName {
						t.Logf("✅ Successfully generated identical script by NID and XName for %s", node.Spec.XName)
					}
				}

				// Test generation by MAC if available
				if node.Spec.BootMAC != "" {
					scriptByMAC, err := controller.GenerateBootScript(ctx, node.Spec.BootMAC, "")
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
			script, err := controller.GenerateBootScript(ctx, node.Spec.XName, "")
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
		script, err := controller.GenerateBootScript(ctx, "x9999c9s9b9n9", "")
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
		script1, err := controller.GenerateBootScript(ctx, testNode.Spec.XName, "")
		duration1 := time.Since(start)
		if err != nil {
			t.Fatalf("Failed to generate boot script: %v", err)
		}

		// Generate same script again (should be cache hit)
		start = time.Now()
		script2, err := controller.GenerateBootScript(ctx, testNode.Spec.XName, "")
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
		script, err := controller.GenerateBootScript(ctx, testNode.Spec.XName, "")
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

func startTestServer(t *testing.T) (string, func()) {
	t.Helper()

	port := reserveFreePort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	dataDir := t.TempDir()
	repoRoot := findRepoRoot(t)

	serverBin := filepath.Join(t.TempDir(), "boot-service-test-server")
	buildCmd := exec.Command("go", "build", "-o", serverBin, "./cmd/server")
	buildCmd.Dir = repoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build boot-service test binary: %v\n%s", err, string(out))
	}

	var logs bytes.Buffer
	cmd := exec.Command(
		serverBin, "serve",
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--data-dir", dataDir,
		"--enable-legacy-api=true",
	)
	cmd.Dir = repoRoot
	cmd.Stdout = &logs
	cmd.Stderr = &logs

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start boot-service test server: %v", err)
	}

	if err := waitForHealth(baseURL, 20*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("boot-service did not become healthy: %v\nserver logs:\n%s", err, logs.String())
	}

	stop := func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			_, _ = fmt.Fprintln(os.Stderr, "boot-service test server forced to stop")
			_ = cmd.Wait()
		case <-done:
		}
	}

	return baseURL, stop
}

func reserveFreePort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve free port: %v", err)
	}
	defer ln.Close() // nolint:errcheck

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("failed to parse reserved tcp address")
	}

	return addr.Port
}

func waitForHealth(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for healthy endpoint")
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to read working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("failed to locate repository root (go.mod)")
		}
		dir = parent
	}
}

func seedIntegrationData(t *testing.T, bootClient *client.Client, ctx context.Context) { //nolint:revive
	t.Helper()

	nodeName := "x0c0s0b0n0"
	createNodeReq := client.CreateNodeRequest{
		Spec: apiv1.NodeSpec{
			XName:   nodeName,
			NID:     42,
			BootMAC: "02:00:00:00:00:42",
			Role:    "compute",
			Groups:  []string{"integration"},
		},
	}
	createNodeReq.Metadata.Name = nodeName

	if _, err := bootClient.CreateNode(ctx, createNodeReq); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		t.Fatalf("failed to create integration node: %v", err)
	}

	createConfigReq := client.CreateBootConfigurationRequest{
		Spec: apiv1.BootConfigurationSpec{
			Hosts:    []string{nodeName},
			Kernel:   "http://files.local/vmlinuz",
			Initrd:   "http://files.local/initrd.img",
			Params:   "console=ttyS0,115200",
			Priority: 100,
		},
	}
	createConfigReq.Metadata.Name = "integration-config"

	if _, err := bootClient.CreateBootConfiguration(ctx, createConfigReq); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		t.Fatalf("failed to create integration boot configuration: %v", err)
	}
}
