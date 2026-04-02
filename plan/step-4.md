<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Step 4 - Repair integration points and preserve legacy compatibility behavior

## Goal

Repair integration mismatches introduced by the migration inputs while preserving legacy compatibility behavior and avoiding direct edits to generated files.

## Completed

- Inspected the safe-edit boundary in `cmd/server/main.go`, legacy handlers, generated route registration, generated client behavior, and resource/storage wiring.
- Confirmed a concrete compatibility mismatch:
  - generated chi routes are registered with trailing slashes (`/bootconfigurations/`, `/nodes/`, `/bmcs/`)
  - the generated client used by the legacy compatibility handler issues slashless requests (`/bootconfigurations`, `/nodes`, `/bmcs`)
- Added `middleware.RedirectSlashes` in `cmd/server/main.go` so slashless modern API requests continue to work without hand-editing generated route files.
- Repaired a malformed note in `plan/marvin.md` so plan tracking remains readable and truthful.

## Why this change belongs in Step 4

This is an integration repair at the safe-edit boundary between generated route registration and handwritten legacy compatibility code. It preserves existing behavior for both direct clients and the internal client used by legacy BSS handlers.

## Not completed in this environment

- Direct Fabrica regeneration is still blocked by the lack of a generic command runner in this session.
- No generated files were edited by hand.

## Validation target

- `go build ./...`
- `go test ./...` (noting existing unrelated HSM test failures if they persist)
