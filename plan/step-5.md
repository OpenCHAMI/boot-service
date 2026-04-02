<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Step 5 - Reconcile tests and add targeted compatibility coverage

## Goal

Reconcile test coverage with the migration state and add targeted compatibility tests for the safe-boundary integration repair introduced in Step 4.

## Completed

- Reviewed existing legacy compatibility tests in `pkg/handlers/legacy/legacy_test.go`.
  - Confirmed they are mostly external integration tests against `http://localhost:8080` and therefore useful for manual/integration validation, but not sufficient as direct regression coverage for router/client compatibility.
- Added focused in-process tests in `cmd/server/main_test.go` to validate the Step 4 compatibility behavior:
  - slashless collection requests (`/bmcs`, `/bootconfigurations`, `/nodes`) succeed against the generated route tree when `middleware.RedirectSlashes` is applied;
  - the generated client continues to work against those slashless collection paths.
- Kept coverage at the handwritten safe-edit boundary rather than editing or testing generated files directly.

## Validation target

- `go test ./cmd/server`
- `go test ./...` (expecting unrelated pre-existing HSM test failures if they persist)
