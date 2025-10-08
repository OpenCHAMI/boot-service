# Boot Service Controllers

This package contains the core controller implementations for the OpenCHAMI Boot Service Phase 2.

## BootScriptController

The `BootScriptController` is the primary component responsible for generating iPXE boot scripts for nodes. It implements the core boot logic defined in Phase 2 of the BSS reimplementation.

### Key Features

- **Multi-identifier Node Resolution**: Supports node identification by XName, NID, or MAC address
- **Intelligent Configuration Matching**: Uses a scoring algorithm to select the best boot configuration
- **Template-based Script Generation**: Generates iPXE scripts using customizable templates
- **Performance Caching**: Caches generated scripts to improve response times
- **Error Handling**: Gracefully handles missing nodes/configurations with fallback scripts

### Usage

```go
// Create a new controller
client := &boot_service_client.Client{...}
logger := log.New(os.Stdout, "bootscript: ", log.LstdFlags)
controller := bootscript.NewBootScriptController(client, logger)

// Generate a boot script
script, err := controller.GenerateBootScript(ctx, "x0c0s0b0n0")
if err != nil {
    log.Fatal(err)
}

fmt.Println(script)
```

### Node Resolution

The controller can resolve nodes using multiple identifier types:

- **XName**: Cray-style node names (e.g., `x0c0s0b0n0`)
- **NID**: Numeric node IDs (e.g., `42`)
- **MAC Address**: Boot MAC addresses (e.g., `a4:bf:01:00:00:01`)

### Configuration Matching

Boot configurations are matched to nodes using a scoring algorithm:

- **Exact MAC match**: 100 points (highest priority)
- **NID match**: 75 points
- **Host/XName pattern match**: 50 points
- **Group membership**: 25 points per matching group
- **Default configuration**: 1 point (fallback)

Configurations with higher scores and priorities are selected first.

### Caching

The controller implements intelligent caching:

- **Script Cache**: Caches complete generated scripts for performance
- **TTL-based Expiration**: Configurable cache lifetime (default: 5 minutes)
- **Selective Invalidation**: Can invalidate cache by node ID or configuration ID
- **Automatic Cleanup**: Removes expired entries automatically

### Templates

iPXE scripts are generated using Go templates:

- **Default Template**: Standard iPXE boot script with DHCP and kernel loading
- **Minimal Template**: Used for nodes without specific configurations
- **Error Template**: Used when script generation fails

### Testing

The package includes comprehensive tests:

```bash
go test ./pkg/controllers/bootscript/... -v
```

Tests cover:
- Cache functionality and expiration
- iPXE template generation
- Template variable preparation
- Filename extraction utilities
- Cache key generation

## Integration

The BootScriptController integrates with:

- **Boot Service Client**: For accessing nodes and configurations
- **HSM (Hardware State Manager)**: For hardware state information
- **Inventory Service**: For group management and node metadata

## Phase 2 Implementation

This controller implements the Phase 2 requirements:

✅ **Core Boot Logic**
- Node resolution by multiple identifiers
- Configuration selection with scoring
- iPXE script generation

✅ **Template System**
- Configurable iPXE templates
- Variable substitution
- Error handling templates

✅ **Performance Optimization**
- Script caching with TTL
- Efficient node/config lookup
- Minimal memory footprint

✅ **Testing & Validation**
- Comprehensive unit tests
- Mock client for testing
- Cache behavior validation

## Next Steps

Phase 3 will add:
- Legacy BSS API compatibility layer
- Enhanced template customization
- Advanced caching strategies
- Metrics and monitoring integration

See `/CLAUDE/IMPLEMENTATION_PLAN.md` for the complete implementation roadmap.