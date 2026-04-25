<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# boot-service

OpenCHAMI Boot Service is a Fabrica-generated REST API for managing node boot
configuration in HPC environments, with legacy BSS-compatible endpoints.

## Quick Start

### Prerequisites

- Go 1.24+
- GNU Make
- `pre-commit` (optional, for local CI-style checks)

### Configure

```bash
cp config.example.yaml config.yaml
```

Configuration precedence (highest to lowest):

1. Command-line flags
2. Environment variables (prefix `BOOT_SERVICE_`)
3. `config.yaml`
4. Built-in defaults

### Build

```bash
make build
```

Build artifacts:

- `bin/server`
- `bin/client`

### Run

```bash
# Run from source
go run ./cmd/server serve

# Run built binary
./bin/server serve

# Example overrides
./bin/server serve --port 8082 --enable-auth --hsm-url http://localhost:27779
```

## API Surface

### Health and Docs

- `GET /health`
- `GET /openapi.json`
- `GET /docs`

### Resource APIs

Generated CRUD/status endpoints for:

- `/bmcs`
- `/bootconfigurations`
- `/nodes`

Routes are generated with trailing slashes and normalized by Chi middleware.

### Legacy BSS Compatibility

When `enable_legacy_api: true`, legacy routes are available under `/boot/v1/`.

## Development Workflow

### Fabrica Generation

Resource definitions live in:

- `.fabrica.yaml`
- `apis.yaml`
- `apis/boot.openchami.io/v1/*_types.go`

Regenerate handlers/storage/client/openapi after API changes:

```bash
# Default: use released Fabrica from go modules
make generate

# CI-style drift check (requires a clean working tree)
make generate-check

# Optional: use local Fabrica checkout (sibling ../fabrica)
(cd ../fabrica && go build -o bin/fabrica ./cmd/fabrica)
make generate FABRICA_LOCAL=1
make generate-check FABRICA_LOCAL=1
```

Do not edit `*_generated.go` files manually.

### Tests

```bash
# Main unit/integration-safe suite (examples excluded)
make test

# Bootscript integration test (opt-in)
make test-integration

# Override test timeout
make test TEST_TIMEOUT=4m
```

`make test-integration` sets `BOOT_SERVICE_RUN_INTEGRATION=1` and runs:

- `TestBootLogicWithExistingData`

### Lint and Local CI

```bash
make lint
make pre-commit-run
```

Useful setup:

```bash
make setup-dev
```

## Configuration Notes

Key settings are documented in `config.example.yaml` and in
`docs/CONFIGURATION.md`.

Common environment variable examples:

```bash
export BOOT_SERVICE_PORT=8082
export BOOT_SERVICE_ENABLE_AUTH=true
export BOOT_SERVICE_HSM_URL=http://localhost:27779
./bin/server serve
```

## Docker

- `Dockerfile`: runtime image expecting a prebuilt binary
- `Dockerfile.standalone`: multi-stage standalone build

## Troubleshooting

- If building with local Fabrica replacements and hitting module proxy issues,
  try: `GOPROXY=direct go build -o bin/server ./cmd/server`
- If a legacy test appears to require an externally running server, use
  `make test-integration` instead of `make test`.

## Documentation

- `docs/PROFILES.md` - Boot profiles for configuration management
- `docs/CONFIGURATION.md` - Service configuration guide
- `docs/AUTHENTICATION.md` - JWT authentication with TokenSmith
- `docs/AUTHENTICATION_TESTING.md` - Testing authentication flows
