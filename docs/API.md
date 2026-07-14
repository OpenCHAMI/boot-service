<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# API Reference

This document summarizes the HTTP API surface currently exposed by the boot
service server.

## Public Endpoints

These routes are registered directly in the server entrypoint:

- `GET /health`
- `GET /openapi.json`
- `GET /docs`

When metrics are enabled, Prometheus metrics are also exposed at:

- `GET /metrics`

and on the separate metrics listener configured by `metrics_port`.

## Modern Resource APIs

The generated REST API exposes three resource types:

- `BMC` at `/bmcs`
- `BootConfiguration` at `/bootconfigurations`
- `Node` at `/nodes`

For each resource type, the generated routes currently support:

- `GET /resource`
- `POST /resource`
- `GET /resource/{uid}`
- `PUT /resource/{uid}`
- `PATCH /resource/{uid}`
- `DELETE /resource/{uid}`
- `PUT /resource/{uid}/status`
- `PATCH /resource/{uid}/status`

The generated router registers trailing-slash routes and the server applies Chi
slash normalization so both slashless and slashful collection paths work.

## Boot API

The boot service exposes boot management endpoints at root paths that are
always available.

### Boot Script Generation

- `GET /bootscript` - Generate iPXE boot script for a node

Query parameters:

- `host` - Node XName (e.g., x0c0s0b0n0)
- `mac` - MAC address (e.g., aa:bb:cc:dd:ee:ff)
- `nid` - Node ID (e.g., 42)
- `profile` - Profile name (currently ignored; auto-selects best match)

Example:

```bash
curl "http://localhost:8080/bootscript?mac=aa:bb:cc:dd:ee:ff"
```

### Boot Parameters Management

- `GET /bootparameters` - List boot configurations
- `POST /bootparameters` - Create boot configuration
- `PUT /bootparameters` - Update boot configuration
- `DELETE /bootparameters` - Delete boot configuration

### Service Information

- `GET /service/status` - Service status information
- `GET /service/version` - Service version information

## Legacy BSS Compatibility API

When `enable_legacy_api: true`, legacy BSS-compatible endpoints are available at `/boot/v1/*`:

- `GET /boot/v1/bootscript`
- `GET /boot/v1/bootparameters`
- `POST /boot/v1/bootparameters`
- `PUT /boot/v1/bootparameters`
- `DELETE /boot/v1/bootparameters`
- `GET /boot/v1/service/status`
- `GET /boot/v1/service/version`

When legacy API is disabled (`enable_legacy_api` is `false`), these `/boot/v1/*` endpoints
return 404 Not Found. Only the modern endpoints at root paths are available.

Example with legacy API enabled:
```bash
curl "http://localhost:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff"
```

**Note:** Both modern and legacy endpoints use the same handler logic. The `profile`
query parameter is currently ignored; the controller auto-selects the best matching
configuration across profiles based on score and priority.

## Generated Client

`make build` produces a generated CLI client at `bin/client`.

Current top-level commands include:

- `client health`
- `client bmc ...`
- `client bootconfiguration ...`
- `client node ...`

Use `./bin/client --help` for the full generated command tree.
