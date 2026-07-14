<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# OpenCHAMI Boot Service - AI Agent Instructions

## Architecture Overview

This is a **Fabrica-generated** REST API service for managing node boot configurations in OpenCHAMI HPC clusters. It provides both modern resource-based APIs and boot endpoints at root paths, plus optional legacy BSS (Boot Script Service) compatibility.

### Key Components

1. **Fabrica Code Generation** - Resources define the API contract, handlers/storage/client are auto-generated
2. **BootScriptController** (`pkg/controllers/bootscript/`) - Core boot logic with iPXE template generation and intelligent config matching
3. **Boot API Handlers** (`pkg/handlers/boot/`) - Modern and legacy (BSS-compatible) boot endpoints
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
# After modifying resources in apis/boot.openchami.io/v1/
make generate

# Validate generated files are committed
make generate-check

# Or use the Makefile
make dev  # generate + build
```

Resources are defined in `apis/boot.openchami.io/v1/` with `Spec` (desired state) and `Status` (observed state) structs.
The `pkg/resources/*` tree is deprecated and should not be used for new code.

### Building

```bash
# Standard build
go build -o bin/boot-service ./cmd/server

# With local fabrica changes (IMPORTANT)
GOPROXY=direct go build -o bin/boot-service ./cmd/server

# Or use Makefile
make build
```

### Running

```bash
# Copy and edit config first
cp config.example.yaml config.yaml

# Run with config file
./bin/server serve

# Override with flags
./bin/server serve --port 8082 --enable-auth --hsm-url http://localhost:27779
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

The repository contains a reusable `pkg/auth` package with three common modes:

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

**Important current runtime note**: the standalone server in `cmd/server/main.go`
does not currently attach `pkg/auth.CreateMiddleware(...)` to its route tree.
`enable_auth` currently affects startup validation and TokenSmith-backed HSM
service-token exchange, not documented request-time route enforcement.

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
enable_metrics: false
enable_legacy_api: true
# metrics_port is configured separately because it becomes active as soon as
# metrics are enabled, even though metrics default to off.
metrics_port: 9090
hsm_url: "http://localhost:27779"
tokensmith_url: "http://localhost:8080"
```

Environment variables use prefix `BOOT_SERVICE_` for standard server settings,
plus `TOKENSMITH_*` for bootstrap-token exchange settings.

## External Service Integration

### HSM (Hardware State Manager)

**Auto-enabled** when `--hsm-url` flag is provided or `hsm_url` is set in config.

**Current Status**: HSM-backed node resolution is wired into the server through
`FlexibleBootScriptController` in `cmd/server/server_extensions.go` when
`hsm_url` is configured.

**Implementation**:
- HSM client: `pkg/clients/hsm/client.go` - HTTP client for HSM v2 API with caching
- Integration service: `pkg/clients/hsm/integration.go` - Wraps HSM client with node provider interface
- Flexible controller: `pkg/controllers/bootscript/flexible_controller.go` - Supports pluggable node providers

**Current Integration Path**:
1. Build an HSM client in `cmd/server/main.go`
2. Create `FlexibleBootScriptController` in `cmd/server/server_extensions.go`
3. Register boot routes with `boot.NewHandlerWithController(...)`
4. Start optional HSM background sync when enabled

**Node resolution with HSM** (when integrated):
- XName lookups: Direct HSM component query (`/hsm/v2/State/Components/{xname}`)
- MAC lookups: Queries HSM ethernet interfaces endpoint (`/hsm/v2/Inventory/EthernetInterfaces`)
- NID lookups: Falls back to retrieving all components and searching

**Caching**: HSM responses are cached (default: 5 minutes) to reduce load on HSM service.

**Current Limitation**: The boot HTTP endpoints (`/bootscript` and `/boot/v1/bootscript`)
ignore the `profile` query parameter and always ask the controller to auto-resolve the
best configuration across profiles.

### TokenSmith

Authentication service providing JWT tokens. Configure via `auth.jwks_url` or `auth.jwt_public_key`.

### BSS Compatibility

Modern boot endpoints are always available at root paths (`/bootscript`, `/bootparameters`, etc.).
Legacy BSS-compatible endpoints at `/boot/v1/*` are available when `enable_legacy_api: true`.
Both modern and legacy endpoints use the same handler logic from `pkg/handlers/boot/`.

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
- `apis/boot.openchami.io/v1/` - Resource definitions (edit these, not generated files)
- `pkg/controllers/bootscript/` - Boot logic, config matching, iPXE generation
- `pkg/handlers/boot/` - Boot API handlers, both modern and legacy BSS- compatible
- `pkg/auth/` - TokenSmith integration and testing utilities
- `config.example.yaml` - Comprehensive config documentation
- `docs/AUTHENTICATION.md` - JWT integration guide
- `docs/CONFIGURATION.md` - Configuration patterns and examples
- `Makefile` - Common development tasks
