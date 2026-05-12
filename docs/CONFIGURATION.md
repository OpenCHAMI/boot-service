<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Configuration Guide

This document describes the configuration keys the current server binary reads
from `cmd/server/main.go`.

If a key is not listed here, assume it is not currently consumed by the server
startup path.

## Quick Start

1. Copy the example configuration:

   ```bash
   cp config.example.yaml config.yaml
   ```

2. Start the service:

   ```bash
   ./bin/server serve
   ```

3. Override settings with flags or environment variables when needed.

## Configuration Precedence

The server resolves configuration in this order:

1. Command-line flags
2. Environment variables
3. `config.yaml`
4. Built-in defaults

The standard server environment variable prefix is `BOOT_SERVICE_`. TokenSmith
bootstrap settings for HSM auth also support standardized `TOKENSMITH_*`
environment variables.

## Supported Runtime Keys

### Server and Storage

```yaml
port: 8080
host: "0.0.0.0"
read_timeout: 30
write_timeout: 30
idle_timeout: 120

data_dir: "./data"
storage_type: "file"
```

### Feature Flags

```yaml
enable_auth: false
enable_metrics: false
enable_legacy_api: true
metrics_port: 9090
```

`enable_legacy_api: false` still leaves `GET /boot/v1/bootscript` available for
node boot flow. It only disables the rest of the legacy BSS compatibility API.

### TokenSmith and HSM

```yaml
tokensmith_url: ""
tokensmith_target_service: "hsm"
tokensmith_bootstrap_policy_scopes_hint: ""
tokensmith_refresh_skew_sec: 120

hsm_url: ""
hsm_sync_enabled: true
hsm_sync_interval: 5
```

Optional bootstrap token input:

```yaml
# tokensmith_bootstrap_token: "<bootstrap-jwt>"
```

Environment fallback:

```bash
export TOKENSMITH_BOOTSTRAP_TOKEN="<bootstrap-jwt>"
```

Deprecated compatibility input still accepted:

```yaml
# tokensmith_scopes: "hsm:read"
```

## Current Auth Behavior

`enable_auth` does **not** currently attach the `pkg/auth` request middleware to
the server routes in `cmd/server/main.go`.

Today, `enable_auth` affects the server in these ways:

- startup validation requires `tokensmith_url` when `enable_auth: true`
- HSM service-token exchange is enabled only when `enable_auth: true`
- if `hsm_url` and `tokensmith_url` are both set while auth is enabled, a bootstrap token is required

If `enable_auth: false`, `tokensmith_url` is ignored for HSM integration.

For package-level JWT and JWKS middleware usage, see `docs/AUTHENTICATION.md`.

## Metrics Behavior

Metrics are disabled by default. When enabled, the server:

- serves `/metrics` on the main router
- starts a separate metrics listener on `host:metrics_port`

## Boot Profiles and HTTP Behavior

Boot profiles are stored on `BootConfiguration.spec.profile`, but the legacy
HTTP bootscript endpoint currently ignores the `profile` query parameter and
auto-selects the best configuration across profiles.

See `docs/PROFILES.md` for the exact behavior split between controller logic and
the legacy HTTP endpoint.

## Unsupported Older Examples

Older docs and examples may still mention nested sections such as:

- `auth:`
- `tokensmith:`
- `logging:`
- `health:`
- `limits:`
- `development:`
- `bss:`

Those sections are not currently unmarshaled by the server config struct in
`cmd/server/main.go`.

## Example Environment Overrides

```bash
export BOOT_SERVICE_PORT=8082
export BOOT_SERVICE_ENABLE_METRICS=true
export BOOT_SERVICE_HSM_URL=http://localhost:27779
./bin/server serve
```

For HSM service-token exchange:

```bash
export BOOT_SERVICE_ENABLE_AUTH=true
export TOKENSMITH_URL=http://localhost:8080
export TOKENSMITH_BOOTSTRAP_TOKEN="<bootstrap-jwt>"
./bin/server serve --hsm-url http://localhost:27779
```

## Validation and Troubleshooting

The current startup validation fails when:

- `port` is outside the valid TCP range
- `enable_auth: true` but `tokensmith_url` is empty
- `tokensmith_refresh_skew_sec` is negative
- `enable_auth: true`, `hsm_url` is set, `tokensmith_url` is set, and no bootstrap token is available

Common checks:

1. If the service will not start, run `./bin/server serve` directly and inspect the startup error.
2. If metrics do not appear, confirm `enable_metrics: true` and check port `9090` unless you changed `metrics_port`.
3. If HSM integration fails while auth is enabled, confirm `TOKENSMITH_BOOTSTRAP_TOKEN` is set.

## See Also

- [API.md](API.md) for the current HTTP surface
- [AUTHENTICATION.md](AUTHENTICATION.md) for package-level auth behavior and current server auth notes
- [PROFILES.md](PROFILES.md) for boot profile behavior
- `config.example.yaml` for a sample config that matches the current runtime keys
