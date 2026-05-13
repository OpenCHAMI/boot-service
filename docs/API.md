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

## Legacy Compatibility API

The server always exposes:

- `GET /boot/v1/bootscript`

When `enable_legacy_api: true`, it also exposes the rest of the BSS-compatible
surface:

- `GET /boot/v1/bootparameters`
- `POST /boot/v1/bootparameters`
- `PUT /boot/v1/bootparameters`
- `DELETE /boot/v1/bootparameters`
- `GET /boot/v1/service/status`
- `GET /boot/v1/service/version`

Current behavior note: the legacy `bootscript` route accepts `host`, `mac`, and
`nid` identifiers, but ignores the `profile` query parameter and auto-selects
the best matching configuration across profiles.

## Generated Client

`make build` produces a generated CLI client at `bin/client`.

Current top-level commands include:

- `client health`
- `client bmc ...`
- `client bootconfiguration ...`
- `client node ...`

Use `./bin/client --help` for the full generated command tree.
