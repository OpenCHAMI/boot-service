package bootscript

import (
	"strings"
	"testing"
	"time"

	"github.com/openchami/boot-service/pkg/resources/bootconfiguration"
	"github.com/openchami/boot-service/pkg/resources/node"
)

// TestScriptCache tests the caching functionality
func TestScriptCache(t *testing.T) {
	cache := NewScriptCache(100 * time.Millisecond)

	// Test Set and Get
	cache.Set("test-key", "test-script", "node1", "config1")

	script, found := cache.Get("test-key")
	if !found {
		t.Errorf("Expected to find cached script")
	}
	if script != "test-script" {
		t.Errorf("Expected 'test-script', got '%s'", script)
	}

	// Test expiration
	time.Sleep(150 * time.Millisecond)
	_, found = cache.Get("test-key")
	if found {
		t.Errorf("Expected cache entry to be expired")
	}

	// Test invalidation
	cache.Set("test-key2", "test-script2", "node2", "config2")
	cache.InvalidateByNodeID("node2")

	_, found = cache.Get("test-key2")
	if found {
		t.Errorf("Expected cache entry to be invalidated")
	}
}

// TestIPXETemplates tests the iPXE script generation templates
func TestIPXETemplates(t *testing.T) {
	controller := createTestController(t)

	// Create test configuration and node
	config := &bootconfiguration.BootConfiguration{
		Spec: bootconfiguration.BootConfigurationSpec{
			Kernel:   "http://files.example.com/vmlinuz",
			Initrd:   "http://files.example.com/initramfs",
			Params:   "console=tty0 console=ttyS0,115200",
			Priority: 50,
		},
	}

	testNode := &node.Node{
		Spec: node.NodeSpec{
			XName:    "x0c0s0b0n0",
			NID:      1,
			BootMAC:  "a4:bf:01:00:00:01",
			Role:     "Compute",
			SubRole:  "Worker",
			Hostname: "compute-001",
			Groups:   []string{"compute", "batch"},
		},
	}

	script, err := controller.buildIPXEScript(config, testNode)
	if err != nil {
		t.Errorf("Unexpected error building iPXE script: %v", err)
		return
	}

	// Verify script content
	expectedContents := []string{
		"#!ipxe",
		"x0c0s0b0n0",
		"vmlinuz",
		"initramfs",
		"console=tty0 console=ttyS0,115200",
		"dhcp",
		"boot",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(script, expected) {
			t.Errorf("Script missing expected content: %s", expected)
		}
	}
}

// TestCacheKeyGeneration tests cache key generation
func TestCacheKeyGeneration(t *testing.T) {
	controller := createTestController(t)

	tests := []struct {
		identifier string
		config     string
		expected   string
	}{
		{"x0c0s0b0n0", "compute-config", "x0c0s0b0n0:compute-config"},
		{"node1", "", "node1:default"},
		{"", "config1", ":config1"},
	}

	for _, tt := range tests {
		result := controller.generateCacheKey(tt.identifier, tt.config)
		if result != tt.expected {
			t.Errorf("Expected cache key %s, got %s", tt.expected, result)
		}
	}
}

// TestTemplateVariablePreparation tests the template variable preparation
func TestTemplateVariablePreparation(t *testing.T) {
	controller := createTestController(t)

	config := &bootconfiguration.BootConfiguration{
		Spec: bootconfiguration.BootConfigurationSpec{
			Kernel:   "http://files.example.com/vmlinuz-5.4.0",
			Initrd:   "http://files.example.com/initramfs-5.4.0",
			Params:   "console=ttyS0,115200",
			Priority: 75,
		},
	}

	testNode := &node.Node{
		Spec: node.NodeSpec{
			XName:    "x0c0s1b0n0",
			NID:      42,
			BootMAC:  "aa:bb:cc:dd:ee:ff",
			Role:     "Management",
			SubRole:  "Login",
			Hostname: "login-001",
			Groups:   []string{"login", "management"},
		},
	}

	vars := controller.prepareTemplateVars(config, testNode)

	// Test node variables
	if vars["XName"] != "x0c0s1b0n0" {
		t.Errorf("Expected XName x0c0s1b0n0, got %v", vars["XName"])
	}
	if vars["NID"] != "42" {
		t.Errorf("Expected NID 42, got %v", vars["NID"])
	}
	if vars["BootMAC"] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("Expected BootMAC aa:bb:cc:dd:ee:ff, got %v", vars["BootMAC"])
	}
	if vars["Groups"] != "login,management" {
		t.Errorf("Expected Groups login,management, got %v", vars["Groups"])
	}

	// Test config variables
	if vars["Kernel"] != "http://files.example.com/vmlinuz-5.4.0" {
		t.Errorf("Expected Kernel URL, got %v", vars["Kernel"])
	}

	// Test derived variables
	if vars["KernelFilename"] != "vmlinuz-5.4.0" {
		t.Errorf("Expected KernelFilename vmlinuz-5.4.0, got %v", vars["KernelFilename"])
	}
	if vars["InitrdFilename"] != "initramfs-5.4.0" {
		t.Errorf("Expected InitrdFilename initramfs-5.4.0, got %v", vars["InitrdFilename"])
	}
}

// createTestController creates a minimal controller for testing
func createTestController(t *testing.T) *BootScriptController {
	return &BootScriptController{
		cache: NewScriptCache(5 * time.Minute),
	}
}

// TestFilenameExtraction tests the filename extraction utility
func TestFilenameExtraction(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://example.com/path/file.txt", "file.txt"},
		{"/local/path/kernel", "kernel"},
		{"simple-filename", "simple-filename"},
		{"", ""},
		{"http://example.com/", ""},
		{"path/with/multiple/levels/file.ext", "file.ext"},
	}

	for _, tt := range tests {
		result := extractFilename(tt.input)
		if result != tt.expected {
			t.Errorf("extractFilename(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}
