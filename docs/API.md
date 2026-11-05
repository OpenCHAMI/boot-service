<!--
SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC

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
- `GET /service/status`
- `GET`, `HEAD /.well-known/api-catalog` - RFC 9727 API catalog

Base URLs in the API catalog and in the OpenAPI `servers` list are fixed at
generation time by `generation.openapi_server_url` in `.fabrica.yaml`, not taken
from the running deployment. They currently use Fabrica's default,
`http://localhost:8080`.

When `enable_metrics` or `--enable-metrics` is enabled, Fabrica-generated
Prometheus metrics are also exposed at:

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

### Resource UIDs

New resources get TypeID UIDs: the lowercase kind, an underscore, and 26 base32
characters, such as `node_01h455vb4pex5vsknk084sn02q`. Resources created by
earlier releases keep their `prefix-hex` UIDs, such as `node-1a2b3c4d`, and both
formats work everywhere a UID is accepted. Treat UIDs as opaque strings: do not
split them or assume a length.

### Create Requests

`POST /resource` requires a `spec`. Omitting it or sending `null` returns HTTP
400 for every resource type, including `BMC`, which previously accepted a
missing `spec`. Send `"spec": {}` to create a `BMC` with an empty spec. `metadata`
remains optional.

### Conditional Writes

`PUT`, `PATCH`, and `DELETE` evaluate `If-Match` and write as one atomic step, so
concurrent writers cannot both pass the same ETag check. A write that keeps
losing to concurrent changes returns HTTP 409 and can be retried. Request bodies
are now parsed before the resource is loaded, so a malformed body returns HTTP
400 even when the UID does not exist.

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

`GET /service/version` reports the running boot-service build using the existing
`service_name`, `service_version`, `git_commit`, and `build_date` fields. Version,
commit, and build date come from the same build variables as `server version`.
Builds without release metadata report `dev`; unavailable commit and build date
fields are omitted. The service name remains `boot-script-service` for
compatibility. This identifies boot-service, independently of Fabrica's generator
version.

With legacy API enabled, `/boot/v1/service/version` returns the same build
information, and `/boot/v1/service/status` reports the same `service_version`.

`GET /service/status` is public and uses Fabrica's generated response:

```json
{"service":"boot-service","status":"ok"}
```

`ok` and `degraded` map to HTTP 200; `down` and `initializing` map to HTTP 503.
The current provider always reports `ok` and does not probe dependencies.
The public response excludes build metadata. This replaces the previous
`service_name`, `service_version`, and `service_status` fields at this path.
The legacy response remains available at `/boot/v1/service/status` when enabled.

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
- `client version`
- `client bmc ...`
- `client bootconfiguration ...`
- `client node ...`

Use `./bin/client --help` for the full generated command tree.
