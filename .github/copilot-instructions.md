<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# OpenCHAMI Boot Service - AI Agent Instructions

## Architecture Overview

This is a **Fabrica-generated** REST API service for managing node boot configurations in OpenCHAMI HPC clusters. It provides both modern resource-based APIs and legacy BSS (Boot Script Service) compatibility.

### Key Components

1. **Fabrica Code Generation** - Resources define the API contract, handlers/storage/client are auto-generated
2. **BootScriptController** (`pkg/controllers/bootscript/`) - Core boot logic with iPXE template generation and intelligent config matching
3. **Legacy BSS API** (`pkg/handlers/legacy/`) - Compatibility layer for legacy systems
4. **Authentication** (`pkg/auth/`) - TokenSmith JWT integration with scope-based authorization
5. **Storage Backend** (`internal/storage/`) - File-based storage (database support planned)

### Data Flow

```
Client → Chi Router → [Auth Middleware] → Generated Handlers → Storage Backend
                                      ↓
                          BootScriptController → iPXE Script
```

## Critical Developer Workflows

### Code Generation (Fabrica)

**NEVER manually edit `*_generated.go` files.** All handlers, storage, and client code is generated from resource definitions.

```bash
# After modifying resources in pkg/resources/*/
fabrica generate --handlers --storage --openapi --client

# Or use the Makefile
make dev  # clean + generate + build
```

Resources are defined in `pkg/resources/{node,bootconfiguration,bmc}/` with `Spec` (desired state) and `Status` (observed state) structs.

### Building

```bash
# Standard build
go build -o bin/boot-service ./cmd/server

# With local fabrica changes (IMPORTANT)
GOPROXY=direct go build -o bin/boot-service ./cmd/server

# Or use Makefile
make build
```

**Note**: `go.mod` has `replace github.com/openchami/fabrica => ../fabrica` for local development.

### Running

```bash
# Copy and edit config first
cp config.example.yaml config.yaml

# Run with config file
./bin/boot-service serve

# Override with flags
./bin/boot-service serve --port 8082 --enable-auth --hsm-url http://localhost:27779
```

### Testing

```bash
# All tests
go test ./...

# Specific package
go test ./pkg/controllers/bootscript/... -v

# Integration tests (require running server at localhost:8080)
go test ./pkg/controllers/bootscript/ -run TestBootLogicWithExistingData -v
```

## Project-Specific Conventions

### Resource Structure Pattern

Every resource follows the Fabrica pattern:

```go
type Resource struct {
    resource.Resource  // Metadata (Name, UID, Labels, etc.)
    Spec   ResourceSpec
    Status ResourceStatus
}
```

- `Spec` = desired state (user input)
- `Status` = observed state (system managed)
- Implement `Validate(ctx context.Context) error` for custom validation

### Node Identification

The system supports **three identifier types** for node lookups:

1. **XName** (Cray format) - e.g., `x0c0s0b0n0`
2. **NID** (Numeric ID) - e.g., `42`
3. **MAC Address** - e.g., `a4:bf:01:00:00:01`

Use `pkg/controllers/bootscript` methods that accept any identifier type.

### Configuration Scoring Algorithm

Boot configurations are matched to nodes with a priority score:

- Exact MAC match: **100 points**
- NID match: **75 points**
- Host/XName pattern: **50 points**
- Group membership: **25 points per group**
- Default config: **1 point**

Higher scores + explicit `Priority` field determine selection. See `pkg/controllers/bootscript/controller.go`.

### iPXE Template System

Templates use Go `html/template` with these variables:

```go
// Node info
{{.XName}} {{.NID}} {{.BootMAC}} {{.Role}} {{.SubRole}} {{.Hostname}} {{.Groups}}

// Boot config
{{.Kernel}} {{.Initrd}} {{.Params}} {{.Priority}}

// Derived
{{.KernelFilename}} {{.InitrdFilename}}
```

Three templates exist: `DefaultIPXETemplate`, `MinimalIPXETemplate`, `ErrorIPXETemplate` in `pkg/controllers/bootscript/ipxe.go`.

## Authentication Patterns

### TokenSmith Integration

Authentication is **optional** and controlled via config. Three modes:

```go
// Development - auth disabled
config := auth.DevConfig()

// Non-enforcing - logs errors, doesn't block
config := auth.NonEnforcingConfig()

// Production - full enforcement
config := auth.DefaultConfig()
config.JWKSURL = "https://auth.openchami.org/.well-known/jwks.json"
config.RequiredScopes = []string{"boot:read"}
```

### Middleware Application

**IMPORTANT**: Apply middleware to router **before** registering routes:

```go
r := chi.NewRouter()
r.Use(middleware.RequestID)  // Core middleware first
r.Use(authMiddleware)        // Auth middleware
RegisterGeneratedRoutes(r)   // THEN register routes
```

Do NOT add middleware inside `RegisterGeneratedRoutes()` - that's auto-generated.

### Scope-Based Authorization

Routes can require specific scopes:

```go
readScope := auth.CreateScopeMiddleware("boot:read")
writeScope := auth.CreateScopeMiddleware("boot:write")

r.Group(func(r chi.Router) {
    r.Use(authMiddleware, readScope)
    r.Get("/boot/list", listHandler)
})
```

Common scopes: `boot:read`, `boot:write`, `boot:admin`, `node:read`, `node:write`.

## Configuration Management

**Priority**: Flags > Environment Variables > Config File > Defaults

```yaml
# config.yaml structure
port: 8080
enable_auth: false
enable_metrics: true
enable_legacy_api: true
hsm_url: "http://localhost:27779"

auth:
  enabled: false
  jwks_url: "https://auth.example.com/.well-known/jwks.json"
  required_scopes: ["boot:read"]
```

Environment variables use prefix `BOOT_SERVICE_` (e.g., `BOOT_SERVICE_PORT=8082`).

## External Service Integration

### HSM (Hardware State Manager)

**Auto-enabled** when `--hsm-url` flag is provided or `hsm_url` is set in config.

**Current Status**: HSM client is initialized and validates connectivity, but not yet fully integrated into the boot script generation pipeline.

**Implementation**:
- HSM client: `pkg/clients/hsm/client.go` - HTTP client for HSM v2 API with caching
- Integration service: `pkg/clients/hsm/integration.go` - Wraps HSM client with node provider interface
- Flexible controller: `pkg/controllers/bootscript/flexible_controller.go` - Supports pluggable node providers

**Integration Options** (see TODOs in `cmd/server/main.go`):
1. **FlexibleBootScriptController**: Use `NewFlexibleBootScriptController` with HSM provider config
2. **Controller-level**: Add NodeProvider parameter to BootScriptController
3. **Storage-level**: Add HSM fallback in storage.GetNode() for transparent integration

**Node resolution with HSM** (when integrated):
- XName lookups: Direct HSM component query (`/hsm/v2/State/Components/{xname}`)
- MAC lookups: Queries HSM ethernet interfaces endpoint (`/hsm/v2/Inventory/EthernetInterfaces`)
- NID lookups: Falls back to retrieving all components and searching

**Caching**: HSM responses are cached (default: 5 minutes) to reduce load on HSM service.

**Current Limitation**: Legacy BSS API handlers use standard BootScriptController which queries local storage only. To enable HSM for boot scripts, modify handlers to accept controller interface and pass FlexibleBootScriptController instance.

### TokenSmith

Authentication service providing JWT tokens. Configure via `auth.jwks_url` or `auth.jwt_public_key`.

### BSS Compatibility

Legacy API at `/boot/v1/*` enabled via `enable_legacy_api: true`. Wraps modern API with BSS-compatible endpoints.

## Container Builds

Two Dockerfiles exist with different purposes:

1. **`Dockerfile`** - Runtime-only, expects prebuilt binary (used by GoReleaser)
2. **`Dockerfile.standalone`** - Multi-stage build, builds inside container (standalone use)

```bash
# GoReleaser workflow
goreleaser build --snapshot --clean

# Standalone Docker build
docker build -f Dockerfile.standalone -t boot-service:local .
```

GoReleaser config: `.goreleaser.yaml` (v2.4.4 compatible, no sboms).

## Common Pitfalls

1. **Editing generated files** - Run `fabrica generate` instead
2. **Build failures with 403** - Use `GOPROXY=direct` when fabrica is local
3. **Middleware order** - Apply middleware before route registration, not during
4. **Config precedence** - Flags override everything, config file is lowest priority
5. **HSM auto-enable** - Providing `--hsm-url` enables HSM, no explicit flag needed

## Key Files Reference

- `cmd/server/main.go` - Server entrypoint with Cobra CLI and config loading
- `pkg/resources/*/` - Resource definitions (edit these, not generated files)
- `pkg/controllers/bootscript/` - Boot logic, config matching, iPXE generation
- `pkg/handlers/legacy/` - BSS compatibility layer
- `pkg/auth/` - TokenSmith integration and testing utilities
- `config.example.yaml` - Comprehensive config documentation
- `docs/AUTHENTICATION.md` - JWT integration guide
- `docs/CONFIGURATION.md` - Configuration patterns and examples
- `Makefile` - Common development tasks
