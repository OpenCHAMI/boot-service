<!--
SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Configuration Guide

This document describes the configuration keys the current server binary reads
from `cmd/server/main.go`.

If a key is not listed here, assume it is not currently consumed by the server
startup path.

`config.example.yaml` in the repository root is the annotated reference file: it
lists every supported key with comments and default values.

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

## Configuration Precedence Order

The server applies configuration in this precedence order:

1. Command-line flags
2. Environment variables
3. `config.yaml`
4. Built-in defaults

The standard server environment variable prefix is `BOOT_SERVICE_`. TokenSmith
bootstrap settings for HSM auth also support standardized `TOKENSMITH_*`
environment variables.

## Supported Runtime Keys

### Server and Storage

| Key | Example | Description |
| --- | --- | --- |
| `port` | `8080` | Main HTTP listen port for the API router. |
| `host` | `"0.0.0.0"` | Interface address bound by the main HTTP listener. |
| `read_timeout` | `30` | Request read timeout in seconds. |
| `write_timeout` | `30` | Response write timeout in seconds. |
| `idle_timeout` | `120` | Keep-alive timeout in seconds for idle connections. |
| `data_dir` | `"./data"` | Filesystem path used by the file-backed storage implementation. |

### Feature Flags

| Key | Example | Description |
| --- | --- | --- |
| `auth_enabled` | `false` | Requires a valid JWT on the resource and boot parameter APIs and enables HSM service-token exchange. |
| `jwks_url` | `""` | JWKS URL used to validate JWTs. `TOKENSMITH_JWKS_URL` takes precedence. |
| `enable_legacy_api` | `true` | Controls availability of legacy BSS-compatible endpoints at `/boot/v1/*`. When `false`, only modern endpoints at root paths are available. |
| `enable_metrics` | `false` | Starts the dedicated metrics listener. `/metrics` on the main listener does not depend on it. |
| `metrics_port` | `9090` | Port used for the dedicated metrics listener when `enable_metrics` is `true`. |

**Modern vs Legacy API Endpoints:**

When `enable_legacy_api` is `false` (default):

- Modern endpoints at root paths are available: `/bootscript`, `/bootparameters`, `/service/*`
- Legacy endpoints at `/boot/v1/*` return 404 Not Found

When `enable_legacy_api` is `true`:

- Both modern and legacy, BSS-compatible endpoints are available
- Not meant for production use! Use modern endpoints for production.
- Legacy endpoints provided for BSS compatibility only

### TokenSmith and HSM

| Key | Example | Description |
| --- | --- | --- |
| `tokensmith_url` | `"http://localhost:8080"` | Base URL for TokenSmith when startup validation or HSM token exchange is enabled. |
| `tokensmith_target_service` | `"hsm"` | Service name requested during TokenSmith service-token exchange. |
| `tokensmith_bootstrap_policy_scopes_hint` | `"hsm:read"` | Optional comma-separated scope hint used for diagnostics during bootstrap exchange. |
| `tokensmith_refresh_skew_sec` | `120` | Number of seconds before expiry that cached service tokens should be treated as stale. |
| `hsm_url` | `"http://localhost:27779"` | Enables HSM-backed node resolution when set. |
| `hsm_sync_enabled` | `true` | Turns the optional background HSM sync loop on or off. |
| `hsm_sync_interval` | `5` | Background HSM sync interval in minutes. |

Optional bootstrap token input:

```yaml
# tokensmith_bootstrap_token: "<bootstrap-jwt>"
```

Environment fallback:

```bash
export TOKENSMITH_BOOTSTRAP_TOKEN="<bootstrap-jwt>"
```

## Current Auth Behavior

With `auth_enabled: true`, the server:

- requires a valid JWT on the resource and boot parameter APIs
- refuses to start without `tokensmith_url` and a JWKS URL
- enables HSM service-token exchange, which needs a bootstrap token when `hsm_url` and `tokensmith_url` are both set

If `auth_enabled: false`, `tokensmith_url` is ignored for HSM integration.

See `docs/AUTHENTICATION.md` for which routes stay public and what is not yet
checked.

### Deprecated Settings

These settings still work so that existing deployments keep starting, and log a
warning:

| Deprecated | Replacement | Behavior |
| --- | --- | --- |
| `enable_auth`, `--enable-auth` | `auth_enabled`, `--auth-enabled` | Enables the same setting, so it now also enforces JWTs. `auth_enabled` wins if both are set. |
| `jwks_endpoint`, `--jwks-endpoint` | `jwks_url`, `--jwks-url` | Used if `jwks_url` and `TOKENSMITH_JWKS_URL` are unset. It previously had no effect. |
| `storage_type`, `--storage-type` | none | Ignored, as it always was in practice. `.fabrica.yaml` selects storage at build time. |

## Metrics Behavior

Metrics are always on in this build. The server installs Fabrica-generated HTTP
metrics middleware and serves Prometheus/OpenMetrics output at `GET /metrics` on
the main listener, without authentication. It emits request counters, latency
histograms, in-flight request gauges, and Go/process/build metrics.

To also serve `/metrics` on a dedicated listener at `host:metrics_port`, use
`enable_metrics` in `config.yaml` or the `--enable-metrics` flag:

```yaml
enable_metrics: true
metrics_port: 9090
```

```bash
./bin/server serve --enable-metrics
```

The generated metric names use the `boot_service` namespace, for example:

- `boot_service_http_request_total`
- `boot_service_http_request_duration_seconds`
- `boot_service_http_requests_in_flight`

Requests the service makes to its own resource API run in process and are not
counted.

`.fabrica.yaml` selects metrics at build time. Removing them there and
regenerating removes the `/metrics` route:

```yaml
features:
  metrics:
    enabled: true
    provider: prometheus
```

**Do not put that nested Fabrica feature block in `config.yaml`.** The
boot-service runtime config only reads `enable_metrics` and `metrics_port`, which
control the dedicated listener.

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
export BOOT_SERVICE_AUTH_ENABLED=true
export TOKENSMITH_URL=http://localhost:8080
export TOKENSMITH_JWKS_URL=http://localhost:8080/.well-known/jwks.json
export TOKENSMITH_BOOTSTRAP_TOKEN="<bootstrap-jwt>"
./bin/server serve --hsm-url http://localhost:27779
```

## Validation and Troubleshooting

The current startup validation fails when:

- `port` is outside the valid TCP range
- `auth_enabled: true` but `tokensmith_url` is empty
- `auth_enabled: true` but no JWKS URL is set
- `auth_enabled: true` with `jwt_issuer`, `jwt_audience`, `jwt_public_key`, or `auth_non_enforcing`, which the generated authentication does not read
- `tokensmith_refresh_skew_sec` is negative
- `auth_enabled: true`, `hsm_url` is set, `tokensmith_url` is set, and no bootstrap token is available

Common checks:

1. If the service will not start, run `./bin/server serve` directly and inspect the startup error.
2. Metrics are at `http://<host>:<port>/metrics`. If `http://<host>:<metrics_port>/metrics` does not respond, confirm `enable_metrics: true` or start with `--enable-metrics`.
3. If HSM integration fails while auth is enabled, confirm `TOKENSMITH_BOOTSTRAP_TOKEN` is set.

## See Also

- [API.md](API.md) for the current HTTP surface
- [AUTHENTICATION.md](AUTHENTICATION.md) for package-level auth behavior and current server auth notes
- [PROFILES.md](PROFILES.md) for boot profile behavior
- `config.example.yaml` for a sample config that matches the current runtime keys
