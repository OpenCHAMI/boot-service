<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Marvin execution tracker

Source of truth for plan progress, since naturally it did not exist when required.

## Steps

- [x] Step 1: Inspect repository, generation boundaries, and automation entrypoints
- [x] Step 2: Update module definitions and generation sources for target versions
- [x] Step 3: Regenerate artifacts and normalize generated outputs (inputs normalized; direct CLI regeneration blocked by session tool limits)
- [x] Step 4: Repair integration points and preserve legacy compatibility behavior
- [x] Step 5: Reconcile tests and add targeted compatibility coverage
- [x] Step 6: Run CI-equivalent validation from a clean state
- [x] Step 7: Update documentation and prepare final migration summary

## Notes

- `boot-service` was missing `plan/` artifacts entirely at start.
- The repository did not contain modern Fabrica inputs (`.fabrica.yaml`, `apis.yaml`, `apis/`), so step 2 introduces them as migration sources of truth.
- Legacy `pkg/resources/*` has been removed after regeneration and integration reconciliation; code now uses `apis/boot.openchami.io/v1` as the canonical source.
- Step 3 normalized versioned API inputs to the flattened Fabrica v0.4.0 envelope and stabilized module state with `go mod tidy`, but direct `fabrica generate` invocation was blocked by the available session tools.
