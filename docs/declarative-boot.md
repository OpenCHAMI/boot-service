<!-- SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC -->
<!-- SPDX-License-Identifier: MIT -->

# Declarative boot configuration

This additive API separates immutable boot content (`BootConfig`) from mutable
node targeting (`BootAssignment`). Existing `BootConfiguration` records, routes,
and selection remain available. Nothing converts legacy records at startup.
The feature implements boot-service #50 and its #68–#73 subissues.

## Authentication and deployment

New management routes require a signed JWT with expiry, issuer equal to the
configured TokenSmith URL, and audience `boot-service`. Configure the JWKS URL
through the existing `jwks_url`/`TOKENSMITH_JWKS_URL` settings. These routes always
enforce scopes: legacy non-enforcing settings do not relax them. Disabled or
unusable verification returns 503; missing/invalid identity returns 401, and a
verified token without the required scope returns 403. Bootscript delivery
continues to work without an operator JWT.

| Activity | Scope |
| --- | --- |
| Read configurations, revisions, and assignments | `read:boot` |
| Create a series or propose/ensure a revision | `propose:boot` |
| Write assignments, promote default, or change health status | `promote:boot` |
| Advance latest while an assignment follows latest | Both `propose:boot` and `promote:boot` |

Use one service writer process per file-storage data directory. The shared
manager serializes assignment, revision, status, and delivery snapshots within
that process. It is not an inter-process lock or a filesystem transaction.
Run one matching service/Fabrica version against a store, and back up the whole
data directory. Revision-state filenames use the bounded canonical format
documented in Fabrica's resource-revisions guide; do not downgrade after writing
that format. Legacy series are read compatibly, but old binaries cannot follow
new canonical state. Sudden storage failure may require restoring a backup.

## Propose, select, promote, and roll back

Names are case-sensitive DNS-subdomain-style names, at most 253 bytes, with
labels at most 63 bytes. They are never implicitly lowercased. Use a bearer token
with the required scopes in these examples:

```sh
curl -fsS -H "Authorization: Bearer $BOOT_TOKEN" -H 'Content-Type: application/json' \
  --data '{"metadata":{"name":"production"},"spec":{"kernel":"https://images.example.invalid/kernel-a","initrd":"/initrd-a","params":"console=ttyS0"}}' \
  "$BOOT_URL/bootconfigs?revisionName=release-a"

curl -fsS -X PUT -H "Authorization: Bearer $BOOT_TOKEN" -H 'Content-Type: application/json' \
  --data '{"metadata":{"name":"production"},"spec":{"kernel":"https://images.example.invalid/kernel-b","initrd":"/initrd-b","params":"console=ttyS0"}}' \
  "$BOOT_URL/bootconfigs/production/revisions/release-b"

curl -fsS -X PUT -H "Authorization: Bearer $BOOT_TOKEN" -H 'Content-Type: application/json' \
  -H 'If-None-Match: *' \
  --data '{"metadata":{"name":"compute"},"spec":{"selector":{"groups":["compute"]},"configRef":{"name":"production"}}}' \
  "$BOOT_URL/bootassignments/by-name/compute"
```

Creating a series sets both default and latest to its first revision. Ensuring
`release-b` advances latest, leaving default unchanged; identical named retries
return the same revision, while changed content at that name returns 409. GET
`/bootconfigs/production` returns pointer UIDs and its strong ETag. PUT
`/bootconfigs/production/default` with that exact `If-Match` and
`{"revisionUid":"<B UID>"}` promotes B. Repeat with the refreshed ETag and A UID
to roll back.

An assignment's `configRef` takes a series `name` and at most one of `uid`,
`revisionName`, or `selector` (`default` or `latest`). A bare name follows
default. Pin a canary to `revisionName: release-b` or a UID; a consumer pinned
to A stays on A through promotion and rollback. An assignment following latest
deploys proposals immediately, which is why advancing latest requires promotion
authority when such a consumer exists.

Selectors use exact MAC, NID, xname, or group alternatives. The highest matching
family wins: MAC 100, NID 75, xname 50, group 25, untargeted default 1. Scores do
not accumulate. Ties use higher priority (0–100), then ascending assignment
name. Empty and `default` profiles are equivalent; other profiles match exactly.
Legacy additive scoring is unchanged. Any matching new assignment opts that
node/profile into the new path, ahead of legacy selection. With no match the
original legacy path runs. If the selected reference is broken, return a
non-booting 503 script; do not fall back to a different boot image.

## Conditional writes and status

GET `/bootassignments/by-name/{name}` or `/bootassignments/{uid}` supplies the
current strong ETag. Initial named PUT requires `If-None-Match: *`; updates,
patches, deletes, and status writes require the exact current `If-Match`.
Missing, stale, wildcard, or weak update conditions return 412. Conflicting or
duplicate conditional headers return 400. An unchanged desired-spec replay
preserves the UID and ETag. Collection POST creates only: duplicate names return
409, and renames are not supported. DELETE returns 204.

PUT/PATCH `/bootconfigs/{name}/status` and
`/bootassignments/{uid}/status` accept only health fields (`phase`, `error`).
They preserve spec, identity, revision content, and deployment pointers. Series
status writes use the series ETag; assignment status writes use the assignment
ETag, so a stale spec update cannot overwrite a concurrent status change.
Revision retirement returns 409 `boot_config_retirement_disabled` until safe
retention semantics are available.

## Freshness and diagnostics

Before every declarative cache lookup, resolve current node identity/membership
and atomically select the assignment and revision. No identity cache is used;
failed refresh refuses declarative delivery. Render-cache keys include the
detached node, assignment, content, revision identity, profile, and template.
The bounded cache avoids rendering, not validation. Requests concurrent with a
mutation can see a coherent old or new snapshot; requests starting after a
successful mutation response use the new state.

Both metrics listeners expose `boot_service_declarative_cache_hits_total` for
actual hits and `boot_service_declarative_membership_cache_ttl_seconds` (zero).
No per-node labels are added. Refreshing SMD on each request increases SMD load
for opted-in deployments; measure this before a large rollout.

## Explicit legacy adoption and recovery

```sh
client --server "$BOOT_URL" boot-adopt --source-uid "$SOURCE_UID" \
  --series production --assignment compute
# Review the JSON plan, then repeat with --execute.
```

The command reads the source without changing it. `--dry-run` is explicit but
optional; it conflicts with `--execute`. The conservative translator accepts
an untargeted default or one exact selector, preserving content, profile, and
priority. It rejects wildcard hosts, multiple selectors, or multiple groups
before any write because their legacy accumulated score cannot be preserved.
The error identifies the source UID. Review interactions with other assignments
before executing, especially untargeted defaults.

Execution creates a deterministic `legacy-<source UID>` revision, using a valid
digest-based name when necessary, then creates an assignment pinned to its UID.
It never deletes legacy records or overwrites a different target assignment.
After a partial or ambiguous network failure, retry the same source/target
mapping. Named ensures reuse committed content; successful replay does not
create duplicate revisions or assignments. Partial writes are retained for
inspection, not destructively rolled back. The JSON receipt reports actual
HTTP requests started/completed and transport ambiguity; errors remain nonzero.

To undo adoption, delete only the new assignment with its current ETag; legacy
selection becomes eligible again. Preserve the revision history. Before upgrade,
stop writers, back up the complete data directory and record the exact binary
and dependency versions. For recovery, stop writers, restore that directory,
and restart the matching version. Restart and container recreation must retain
the durable directory; ephemeral storage is unsuitable. Restoration is a
matching-version recovery procedure, not an old-binary downgrade guarantee.
