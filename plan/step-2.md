# Step 2 - Update module definitions and generation sources

## Goal

Introduce the modern Fabrica source inputs and align module metadata for the target generator version without regenerating artifacts yet.

## Completed

- Bumped direct `github.com/openchami/fabrica` module requirement to `v0.4.0`.
- Added `.fabrica.yaml` as generator project configuration.
- Added `apis.yaml` as API source-of-truth.
- Added handwritten versioned API type sources under `apis/boot.openchami.io/v1/` for:
  - `BMC`
  - `BootConfiguration`
  - `Node`
- Recorded execution progress in `plan/marvin.md`.

## Deferred to later steps

- Run `fabrica generate`
- Reconcile generated fallout with existing handwritten code and legacy compatibility paths
- Run full validation and doc updates tied to regenerated behavior
