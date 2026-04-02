# Step 3 - Regenerate artifacts and normalize generated outputs

## Goal

Run Fabrica regeneration against the new versioned API source inputs and normalize any generated fallout required before integration repair.

## Completed

- Inspected the Fabrica v0.4.0 generator entrypoint and confirmed versioned discovery expects flattened resource envelope fields (`APIVersion`, `Kind`, `Metadata`).
- Normalized the versioned API source files under `apis/boot.openchami.io/v1/` from embedded `resource.Resource` to the flattened envelope shape expected by the target generator.
- Ran `gofmt` on the modified API source files.
- Ran `go mod tidy` to stabilize module metadata so generation/build steps can proceed without pending module updates.
- Verified `go build ./...` succeeds in `boot-service` after input normalization.

## Blocked / not completed in this environment

- Could not run `fabrica generate` directly because the available execution tools in this session do not include a generic command runner for invoking the CLI.
- Therefore no `*_generated.go` files were regenerated in this step within the session.

## Observations

- A `go test ./...` run after module normalization succeeds for many packages but still reports pre-existing failures in `pkg/clients/hsm`:
  - `TestHSMClient_GetEthernetInterfaces`
  - `TestHSMClient_GetComponentByMAC`
- Those failures are unrelated to Fabrica regeneration and were not changed in this step.

## Next command to run outside this tool limitation

```bash
go run github.com/openchami/fabrica/cmd/fabrica generate
```

Then inspect and reconcile the generated fallout in Step 4.
