<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Changes remain under `Unreleased` until they ship in the next tagged release.

## [Unreleased]

### Added

- Added Fabrica v0.4.9 Prometheus metrics instrumentation generated from
  `.fabrica.yaml`.
- Added generated server and client `version` commands that report the Fabrica
  generator version.
- Added generated client simple create/update helpers and multi-resource delete
  support from Fabrica v0.4.9.

### Changed

- Regenerated with Fabrica v0.4.9 while keeping boot-service runtime metrics
  configuration on `enable_metrics` / `--enable-metrics`.

## [v0.2.0] - 2026-07-17

### Added

- Added modern boot API endpoints at root paths
  - `GET /bootscript`
  - `GET/POST/PUT/DELETE /bootparameters`
  - `GET /service/status`
  - `GET /service/version`

### Changed

- **BREAKING:** Moved boot endpoints from `/boot/v1/*` to root paths.
  - `/boot/v1/*` prefix now exclusively for legacy BSS compatibility when `enable_legacy_api` is `true`.
- **BREAKING:** Renamed `pkg/handlers/legacy` package to `pkg/handlers/boot` with unified handler supporting both modern and legacy routing.
- Changed `enable_legacy_api` behavior
  - When `false`, only modern endpoints are available; when `true`, both modern and legacy endpoints are available.
- Updated all documentation to reflect modern endpoint paths and legacy API behavior.
- Refactored server route registration to clearly separate modern and legacy endpoint registration.

## [0.1.7] - 2026-06-17

### Added

- Added YAML struct tags to allow marshalling/unmarshalling YAML data for resources

### Changed

- Regenerated server and client using Fabrica v0.4.8

## [0.1.6] - 2026-06-01

### Added

- Added `--log-level`/`-l` flag and debug messages containing HTTP request/response details
- Added client unit tests

### Changed

- Regenerated server and client using Fabrica v0.4.7

## [0.1.5] - 2026-05-13

### Added

- Added `GET /health` and a generated `client health` command for quick service checks.
- Added OpenAPI publication endpoints at `GET /openapi.json` and `GET /docs`.
- Added `PATCH` operations for `BMC`, `BootConfiguration`, and `Node` resources.
- Added custom validation hooks for `BMC`, `BootConfiguration`, and `Node` handlers.

### Changed

- Regenerated server, client, storage, and OpenAPI surfaces against Fabrica `v0.4.5`.
- Updated generated file headers to include Fabrica version metadata.
- Updated the Docker release build to pass dynamic build arguments into image builds.
- Tightened code generation drift checks around the current Fabrica workflow.
- Documented the generated service endpoints added in this release, including `/health`, `/openapi.json`, and `/docs`.

## [0.1.4] - 2026-05-06

### Added

- Added HSM group membership lookups and response caching to improve node resolution.

### Changed

- Added missing configuration aliases used by HSM-related settings.

### Fixed

- Cleaned up HSM client handling and a small lint-related response body close issue.

## [0.1.3] - 2026-05-05

### Added

- Added the legacy boot script endpoint behind the `enable_legacy_api` feature flag.
- Added explicit code generation drift checks via `make generate-check`.

### Changed

- Clarified boot profile behavior and validation in the docs.
- Changed empty-profile boot script selection to auto-resolve the best matching configuration across profiles.
- Updated the local Fabrica workflow in the Makefile and regenerated outputs for the newer generator.
- Refactored server integration setup for clearer handler registration.

## [0.1.2] - 2026-04-26

### Fixed

- Added the missing OpenAPI API routes.

## [0.1.1] - 2026-04-15

### Changed

- Added Docker Buildx setup with a custom build image in the release pipeline.

## [0.1.0] - 2026-04-15

### Added

- Initial tagged release of the Fabrica-generated boot-service API.
- File-backed `BMC`, `BootConfiguration`, and `Node` resource APIs.
- Legacy BSS-compatible boot endpoints and generated Go client support.

[Unreleased]: https://github.com/OpenCHAMI/boot-service/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/OpenCHAMI/boot-service/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/OpenCHAMI/boot-service/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/OpenCHAMI/boot-service/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/OpenCHAMI/boot-service/compare/v0.1.0...v0.1.1
