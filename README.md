<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# boot-service

OpenCHAMI Boot Service is a Fabrica-generated REST API for managing node boot
configuration in HPC environments. It exposes modern resource APIs for
`BMC`, `BootConfiguration`, and `Node` objects, plus boot endpoints
(`/bootscript`, `/bootparameters`, `/service/status`). Legacy BSS-compatible
endpoints are available at `/boot/v1/*` when `enable_legacy_api: true`.

## What Is In This Repo

- Generated CRUD and status endpoints for `/bmcs`, `/bootconfigurations`, and `/nodes`
- Modern boot endpoints at `/bootscript`, `/bootparameters`, and `/service/*`
- Legacy BSS-compatible endpoints at `/boot/v1/*` (when enabled)
- Boot script generation with node matching by XName, NID, or MAC address
- A reusable TokenSmith auth package plus generated AuthZ classifier scaffolding
- Optional HSM-backed node resolution, including TokenSmith service-token exchange
- Optional Fabrica-generated Prometheus metrics at `/metrics`
- OpenAPI publishing at `/openapi.json` and `/docs`
- A generated CLI client with commands such as `./bin/client health` and `./bin/client version`

## Quick Start

### Prerequisites

- GNU Make
- A Go toolchain compatible with `go.mod` (this branch currently declares `go 1.26.5`)
- `pre-commit` if you want local CI-style checks

### Configure

```bash
cp config.example.yaml config.yaml
```

Configuration precedence is:

1. Command-line flags
2. Environment variables
3. `config.yaml`
4. Built-in defaults

The server reads `BOOT_SERVICE_*` environment variables. TokenSmith bootstrap
settings for HSM auth also support the standardized `TOKENSMITH_*` variables
documented in `config.example.yaml`.

### Build

```bash
make build
```

Local build artifacts:

- `bin/server`
- `bin/client`

### Run

```bash
# Run from source
go run ./cmd/server serve

# Run the built server
./bin/server serve

# Enable runtime exposure of Prometheus metrics at /metrics
./bin/server serve --enable-metrics

# Optional client smoke test
./bin/client --server http://localhost:8080 health

# Show client build and Fabrica generator version information
./bin/client version

# Show server build and Fabrica generator version information
./bin/server version
```

Example overrides:

```bash
./bin/server serve \
  --port 8082 \
  --enable-auth \
  --hsm-url http://localhost:27779 \
  --tokensmith_url http://localhost:8080
```

## Current API Behavior

### Health, Docs, and Metrics

- `GET /health` returns a small JSON health response
- `GET /openapi.json` serves the generated OpenAPI document
- `GET /docs` serves Swagger UI
- When `enable_metrics` or `--enable-metrics` is enabled, Fabrica-generated Prometheus metrics are exposed at `/metrics` on the main server listener and on the separate metrics listener configured by `metrics_port`

### Modern Resource APIs

The generated API supports the current resource set:

- `/bmcs`
- `/bootconfigurations`
- `/nodes`

The current generated surface includes `PATCH` support for these resources.
Routes are registered with trailing slashes and normalized by Chi middleware.

### Boot API Endpoints

The boot service provides modern boot API endpoints at root paths:

- `GET /bootscript` - Generate iPXE boot script for a node
- `GET /bootparameters` - List boot configurations
- `POST /bootparameters` - Create boot configuration
- `PUT /bootparameters` - Update boot configuration
- `DELETE /bootparameters` - Delete boot configuration
- `GET /service/status` - Service status information
- `GET /service/version` - Service version information

These endpoints accept node identifiers (`host`, `mac`, or `nid`) and support
intelligent boot configuration matching by score and priority.

### Legacy BSS Compatibility

When `enable_legacy_api: true`, legacy BSS-compatible routes are available at `/boot/v1/*`:

- `GET /boot/v1/bootscript`
- `GET /boot/v1/bootparameters`
- `POST /boot/v1/bootparameters`
- `PUT /boot/v1/bootparameters`
- `DELETE /boot/v1/bootparameters`
- `GET /boot/v1/service/status`
- `GET /boot/v1/service/version`

When legacy API is disabled, only the modern endpoints at root paths are available.

**Important:** Both modern and legacy endpoints use the same handler logic and support
the same features. The `profile` query parameter is ignored and the controller always
auto-resolves the best matching boot configuration by score and priority.

### Boot Profiles

Boot profiles are supported in the boot script controller and modern
`BootConfiguration` resources. When a requested profile is empty, the controller
selects the best matching configuration across profiles; when a requested
profile has no match, it falls back to the default profile.

See `docs/PROFILES.md` for the full model and examples.

### Authentication and HSM Integration

- The repository includes a reusable `pkg/auth` package for JWT, JWKS, scope, and service-token middleware patterns
- The current server binary does not attach `pkg/auth` request middleware in `cmd/server/main.go`
- `enable_auth: true` currently gates TokenSmith-dependent startup behavior and requires `tokensmith_url`
- Supplying `hsm_url` enables HSM-backed node lookups
- If both `enable_auth: true` and `tokensmith_url` are set, the server can exchange a bootstrap token for short-lived HSM service tokens

## Development Workflow

### Fabrica Generation

Resource definitions live under `apis/boot.openchami.io/v1/` and are wired by
`.fabrica.yaml` and `apis.yaml`.

Do not edit `*_generated.go` files manually.

Regenerate handlers, storage, client code, and OpenAPI after API changes:

```bash
make generate
make generate-check
```

`make generate-check` requires a clean git tree and fails if regeneration would
change tracked files.

If you are working against a local Fabrica checkout, point the Makefile at that
directory instead of using the old `FABRICA_LOCAL=1` pattern:

```bash
(cd ../fabrica && go build -o bin/fabrica ./cmd/fabrica)
make generate LOCAL_FABRICA=../fabrica
make generate-check LOCAL_FABRICA=../fabrica
```

### Test and Lint

```bash
make test
make test-integration
make lint
make pre-commit-run
```

`make test-integration` sets `BOOT_SERVICE_RUN_INTEGRATION=1` and runs
`TestBootLogicWithExistingData`.

Useful setup:

```bash
make setup-dev
```

## Release Notes

- `CHANGELOG.md` tracks release history and the next unreleased entry
- `.github/workflows/Release.yaml` publishes tagged releases with GoReleaser on `v*` tags
- `make release-snapshot` creates a local snapshot release for verification

## Docker

- `Dockerfile` expects a prebuilt binary and is used by the release flow
- `Dockerfile.standalone` performs a multi-stage container build
- The distroless runtime image does not include `curl` or `wget`; probe `/health` externally instead of using an in-container Docker `HEALTHCHECK`

## Troubleshooting

- If local Fabrica development hits Go proxy issues, try `GOPROXY=direct go build -o bin/server ./cmd/server`
- If you want to verify only generated-file drift, start from a clean tree and run `make generate-check`
- If an integration test seems to assume a running server, use `make test-integration` instead of `make test`

## Documentation

- `docs/PROFILES.md` for boot profile behavior and examples
- `docs/API.md` for the current HTTP endpoint surface
- `docs/CONFIGURATION.md` for configuration details
- `docs/AUTHENTICATION.md` for TokenSmith JWT integration
- `docs/AUTHENTICATION_TESTING.md` for auth test coverage and examples
- `CHANGELOG.md` for release history
