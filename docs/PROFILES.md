<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Boot Profiles Guide

Boot profiles are labels on `BootConfiguration.spec.profile`. They organize boot
configurations into logical groups such as `compute`, `login`, or `debug`.

## Current Behavior Split

There are two profile-related behaviors in this repository and they are not the
same:

1. The boot script controller supports requested profiles and fallback to the default profile.
2. The legacy HTTP bootscript endpoint ignores the `profile` query parameter and auto-selects the best match across profiles.

**That distinction matters more than anything else in this document.**

## Profile Model

- `spec.profile == ""` or omitted means the configuration belongs to the default profile.
- Any non-empty `spec.profile` groups that configuration under a named profile.
- Profiles do not assign nodes by themselves; matching still depends on `hosts`, `macs`, `nids`, and `groups`.

Example default profile configuration:

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: default-boot
spec:
  kernel: "http://files.openchami.org/vmlinuz-generic"
  initrd: "http://files.openchami.org/initramfs-generic.img"
  params: "console=ttyS0,115200 root=/dev/ram0"
  priority: 1
```

Example named profile configuration:

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: compute-standard
spec:
  profile: "compute"
  hosts:
    - "x0c0s*"
  kernel: "http://files.openchami.org/vmlinuz-compute"
  initrd: "http://files.openchami.org/initramfs-compute.img"
  params: "console=ttyS0,115200 root=/dev/ram0 cgroup_memory=1"
  priority: 50
```

## Creating Profiled Configurations

Create configurations through the modern API:

```bash
curl -X POST http://boot-service:8080/bootconfigurations \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {"name": "compute-standard"},
    "spec": {
      "profile": "compute",
      "hosts": ["x0c0s*"],
      "kernel": "http://files.openchami.org/vmlinuz-compute",
      "initrd": "http://files.openchami.org/initramfs-compute.img",
      "params": "console=ttyS0,115200 root=/dev/ram0 cgroup_memory=1",
      "priority": 50
    }
  }'
```

Useful fields on `BootConfiguration.spec`:

- `profile`: logical profile label such as `compute` or `debug`
- `hosts`: XName or hostname glob patterns used for node matching
- `macs`: exact boot MAC addresses with the highest match score
- `nids`: numeric node identifiers used for explicit node targeting
- `groups`: group labels matched against node group membership
- `kernel`: kernel image URL served to iPXE
- `initrd`: initramfs image URL served to iPXE
- `params`: kernel command-line arguments appended to the boot entry
- `priority`: tie-breaker used after the match score is computed

## Controller Behavior

The controller method `GenerateBootScript(ctx, identifier, profile)` honors the
requested profile.

Current controller rules are:

- when `profile` is empty, search across all profiles and choose the best match by score then priority
- when `profile` is non-empty, search only that profile first
- when a requested profile has no match, retry against the default profile

This behavior is covered by tests in `pkg/controllers/bootscript/controller_profile_test.go`.

## Legacy HTTP Endpoint Behavior

The legacy endpoint is:

- `GET /boot/v1/bootscript`

It accepts node identifiers through:

- `?mac=`
- `?host=`
- `?nid=`

Current limitation: the handler ignores any `profile` query parameter and always
calls the controller with an empty profile.

That means all of these requests behave the same as far as profile selection is
concerned:

```bash
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff"
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute"
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=default"
```

In every case, the server auto-resolves the best configuration across profiles.

## Selection Algorithm

The current score model is:

- exact MAC match: `100`
- NID match: `75`
- host/XName pattern match: `50`
- group membership: `25` per matched group
- catch-all/default config: `1`

Candidates are ordered by:

1. score descending
2. `priority` descending

## Operational Guidance

Use profiles today for:

- organizing `BootConfiguration` resources
- controller-level integrations that call `GenerateBootScript(..., profile)` directly
- preparing for a future HTTP surface that may expose explicit profile selection

Do not assume profile-specific HTTP behavior from `/boot/v1/bootscript` yet.

## Troubleshooting

### A profile query parameter does nothing

That is the current expected behavior of the legacy HTTP route. The handler
accepts the parameter but ignores it, and matching is still based on best score
and priority across profiles.

### The wrong profile appears to win

Inspect the stored boot configurations and compare their targeting fields and
priority values:

```bash
curl "http://boot-service:8080/bootconfigurations" | jq '.[] | {name: .metadata.name, profile: .spec.profile, priority: .spec.priority, hosts: .spec.hosts, macs: .spec.macs, nids: .spec.nids, groups: .spec.groups}'
```

### Default fallback is missing

Make sure at least one configuration has an empty or omitted `profile` field.

```bash
curl "http://boot-service:8080/bootconfigurations" | jq '.[] | select((.spec.profile // "") == "")'
```

## See Also

- [API.md](API.md) for the current HTTP endpoint surface
- [CONFIGURATION.md](CONFIGURATION.md) for server configuration behavior
