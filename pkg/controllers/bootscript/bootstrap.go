// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

/*
Package bootscript provides iPXE boot script generation and node boot configuration management
for the OpenCHAMI boot service.

# Overview

This package implements the core boot logic for OpenCHAMI clusters, handling iPXE script
generation, boot configuration matching, and node identification. It supports multiple
data sources through pluggable node providers and implements intelligent configuration
selection with scoring algorithms.

# Architecture

The package consists of several key components:

  - BootScriptController: Core controller for iPXE script generation
  - FlexibleBootScriptController: Extended controller with pluggable node providers
  - ScriptCache: Performance optimization through boot script caching
  - NodeProvider: Interface for different node data backends (HSM, YAML, etc.)

# Boot Script Generation

Boot scripts are generated through a multi-step process:

 1. Node Identification: Resolve node identity from XName, NID, or MAC address
 2. Configuration Matching: Find the best-matching boot configuration using scoring
 3. Template Rendering: Generate iPXE script from templates with node/config data
 4. Caching: Store generated scripts for improved performance

Example usage:

	// Create controller with standard client backend
	controller := bootscript.NewBootScriptController(client, logger)

	// Generate boot script for a node
	script, err := controller.GenerateBootScript(ctx, "x0c0s0b0n0")
	if err != nil {
		log.Fatal(err)
	}

	// Use flexible controller with HSM integration
	config := ProviderConfig{
		Type: "hsm",
		HSMConfig: &hsm.IntegrationConfig{
			BaseURL: "http://localhost:27779",
			Timeout: 30 * time.Second,
		},
	}
	flexController, err := NewFlexibleBootScriptController(client, config, logger)

# Node Identification

Nodes can be identified using three methods:

  - XName: Cray hardware naming (e.g., "x0c0s0b0n0")
  - NID: Numeric node ID (e.g., "42")
  - MAC: MAC address (e.g., "a4:bf:01:00:00:01")

The controller automatically detects the identifier type and resolves the node accordingly.

# Configuration Matching Algorithm

Boot configurations are matched to nodes using a priority-based scoring system:

  - Exact MAC match: 100 points
  - NID match: 75 points
  - Host/XName pattern: 50 points
  - Group membership: 25 points per group
  - Default config: 1 point

The configuration with the highest score is selected. If multiple configurations have
the same score, the explicit Priority field is used as a tiebreaker.

Example configuration scoring:

	Config A: MAC match (100) + Priority(10) = 110 points
	Config B: XName match (50) + 2 groups (50) + Priority(5) = 105 points
	Config C: Default (1) + Priority(0) = 1 point

	Result: Config A is selected

# iPXE Templates

Boot scripts are generated from Go templates with access to node and configuration data.
Three built-in templates are provided:

  - DefaultIPXETemplate: Standard boot sequence with kernel, initrd, and parameters
  - MinimalIPXETemplate: Bare-minimum boot script for unconfigured nodes
  - ErrorIPXETemplate: Error handling script for troubleshooting

Template variables include:

	Node Data:
	  {{.XName}}     - Hardware location (x0c0s0b0n0)
	  {{.NID}}       - Numeric node ID
	  {{.BootMAC}}   - Boot interface MAC address
	  {{.Role}}      - Node role (Compute, Management, etc.)
	  {{.SubRole}}   - Optional sub-role classification
	  {{.Hostname}}  - Fully qualified hostname
	  {{.Groups}}    - Comma-separated group memberships

	Boot Configuration:
	  {{.Kernel}}    - Kernel URL
	  {{.Initrd}}    - Initrd URL
	  {{.Params}}    - Kernel parameters
	  {{.Priority}}  - Configuration priority

	Derived:
	  {{.KernelFilename}} - Extracted kernel filename
	  {{.InitrdFilename}} - Extracted initrd filename

# Node Providers

The FlexibleBootScriptController supports pluggable node providers through the
NodeProvider interface. Built-in providers include:

HSM Provider: Integrates with Hardware State Manager for production deployments
  - Real-time node data from cluster management
  - Automatic component discovery
  - MAC address to XName resolution

YAML Provider: File-based configuration for development and testing
  - Simple YAML file format
  - Automatic reload on file changes
  - Useful for offline development

Example YAML provider setup:

	config := ProviderConfig{
		Type: "yaml",
		YAMLConfig: &local.IntegrationConfig{
			FilePath:   "/etc/openchami/nodes.yaml",
			AutoReload: true,
		},
	}

# Caching

The ScriptCache provides automatic caching of generated boot scripts to reduce
computational overhead and improve response times. Cache behavior:

  - Default TTL: 5 minutes
  - Automatic expiration and cleanup
  - Thread-safe concurrent access
  - Invalidation on configuration changes

Cache keys are generated from node identifier and configuration data to ensure
correct invalidation when either changes.

# Performance Considerations

Boot script generation performance is critical for large-scale clusters. Optimizations include:

  - Script caching with automatic expiration
  - Efficient node identifier resolution
  - Minimized backend queries through provider caching
  - Template pre-parsing and reuse

For clusters with thousands of nodes, the cache hit rate typically exceeds 95%
during steady-state operations.

# Error Handling

The package provides comprehensive error handling:

  - Node not found: Returns minimal boot script to allow PXE retry
  - Configuration missing: Uses default configuration if available
  - Template errors: Returns error script with diagnostic information
  - Backend failures: Logs warnings and attempts fallback strategies

Errors are logged but generally don't prevent boot script generation, allowing
nodes to boot even during partial service degradation.

# Integration Points

This package integrates with several other boot service components:

  - pkg/client: REST API client for boot configuration management
  - pkg/resources/node: Node resource definitions
  - pkg/resources/bootconfiguration: Boot configuration resources
  - pkg/clients/hsm: Hardware State Manager integration
  - pkg/clients/local: Local file-based node provider
  - pkg/handlers/legacy: Legacy BSS API compatibility

# Thread Safety

All controllers and caches are thread-safe and can be safely used from multiple
goroutines. Read operations use RWMutex for concurrent access, while write
operations are properly synchronized.

# Best Practices

When using this package:

 1. Reuse controller instances - they maintain internal caches
 2. Use context with appropriate timeouts for external requests
 3. Configure cache TTL based on configuration change frequency
 4. Monitor cache hit rates for performance tuning
 5. Use HSM provider for production, YAML for development
 6. Implement proper logging for troubleshooting boot issues

# Testing

The package includes comprehensive test coverage:

  - controller_test.go: Core controller functionality
  - flexible_controller_test.go: Provider integration tests
  - enhanced_controller_test.go: Advanced feature tests
  - integration_existing_test.go: End-to-end integration tests

Run tests with:

	go test ./pkg/controllers/bootscript/... -v

# Future Enhancements

Planned improvements include:

  - Dynamic template loading from configuration
  - A/B testing support for boot configurations
  - Advanced metrics and observability
  - Additional node provider backends (database, etcd)
  - Boot failure tracking and automatic remediation
  - Configuration versioning and rollback

For more information, see the OpenCHAMI boot service documentation at:
https://github.com/openchami/boot-service
*/
package bootscript
