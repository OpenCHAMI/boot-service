<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Boot Profiles Guide

This document describes how to use boot profiles to manage different boot configurations for different node types or operational scenarios.

## Table of Contents

1. [Overview](#overview)
2. [Core Concepts](#core-concepts)
3. [Quick Start](#quick-start)
4. [Creating Boot Configurations](#creating-boot-configurations)
5. [Requesting Boot Scripts](#requesting-boot-scripts)
6. [Profile Selection Algorithm](#profile-selection-algorithm)
7. [API Reference](#api-reference)
8. [Best Practices](#best-practices)
9. [Troubleshooting](#troubleshooting)
10. [Migration Guide](#migration-guide)

## Overview

Boot profiles allow you to organize boot configurations into logical groups, enabling:

- **Different boot environments**: Compute nodes, storage nodes, login nodes, management nodes
- **Operational scenarios**: Standard production boot, maintenance/debug boot, specialized workload boot
- **Configuration management**: Keep related boot parameters organized and maintainable
- **Dynamic selection**: Choose the profile at runtime based on operational needs

## Core Concepts

### What is a Profile?

A **profile** is a label you assign to boot configurations to group them logically. It's purely organizational—profiles don't exist independently; they're just values in the `spec.profile` field of BootConfiguration resources.

### The Default Profile

**The "default" profile is special and you must create it.** Here's what it means:

- **Definition**: Any BootConfiguration with an empty or omitted `profile` field belongs to the "default" profile
- **Not auto-created**: The service does NOT create a default profile at startup—you must create it
- **Safety net**: When a requested profile doesn't exist or doesn't match the node, the system falls back to default profile configurations
- **Required**: For the service to work reliably, at least one default configuration should always exist

**Example of a default configuration** (note the missing `profile` field):

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: default-boot
spec:
  # No 'profile' field here - this makes it the DEFAULT
  kernel: "http://files.openchami.org/vmlinuz-generic"
  initrd: "http://files.openchami.org/initramfs-generic.img"
  params: "console=ttyS0,115200 root=/dev/ram0"
  priority: 1  # Low priority ensures specific profiles take precedence
```

**Example of a named profile** (compare to above):

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: compute-standard
spec:
  profile: "compute"  # This makes it a NAMED profile, not default
  hosts:
    - "x0c0s*"
  kernel: "http://files.openchami.org/vmlinuz-compute"
  initrd: "http://files.openchami.org/initramfs-compute.img"
  params: "console=ttyS0,115200 root=/dev/ram0 cgroup_memory=1"
  priority: 50
```

### Node Targeting vs. Profile Assignment

**Important distinction**: Profiles don't "assign" configurations to nodes. Instead:

1. **Boot configurations define matching criteria** (`hosts`, `macs`, `nids`, `groups`) that select which nodes they apply to
2. **Profiles group related configurations** for organizational purposes
3. **Both factors determine selection**: When you request a boot script, the system:
   - Finds all configurations matching your requested profile
   - Among those, scores each based on how well the targeting criteria match your node
   - Returns the highest-scoring configuration

**Example flow**:

```
Node boots with MAC aa:bb:cc:dd:ee:ff
↓
Node requests: "Give me boot script for profile=compute"
↓
System searches:
  - Configurations with profile="compute" (only these)
  - That also match MAC aa:bb:cc:dd:ee:ff (or other targeting criteria)
  - Scores each match (MAC match = 100 points, pattern match = 50 points, etc.)
↓
Returns highest-scoring config for compute profile
Or falls back to default profile if no compute configs match
```

## Quick Start

### Scenario: Boot Different Node Types

You have compute nodes and login nodes. You want each to boot with different kernels.

**Step 1: Create a default configuration** (required fallback)

```bash
curl -X POST http://boot-service:8080/bootconfigurations \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {
      "name": "default-boot"
    },
    "spec": {
      "kernel": "http://files.openchami.org/vmlinuz-generic",
      "initrd": "http://files.openchami.org/initramfs-generic.img",
      "params": "console=ttyS0,115200 root=/dev/ram0",
      "priority": 1
    }
  }'
```

**Step 2: Create a compute profile**

```bash
curl -X POST http://boot-service:8080/bootconfigurations \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {
      "name": "compute-standard"
    },
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

**Step 3: Create a login profile**

```bash
curl -X POST http://boot-service:8080/bootconfigurations \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {
      "name": "login-standard"
    },
    "spec": {
      "profile": "login",
      "hosts": ["login[0-9]"],
      "kernel": "http://files.openchami.org/vmlinuz-login",
      "initrd": "http://files.openchami.org/initramfs-login.img",
      "params": "console=ttyS0,115200 root=/dev/nvme0n1p1",
      "priority": 50
    }
  }'
```

**Step 4: Request boot scripts**

```bash
# Compute node requests boot script for compute profile
curl "http://boot-service:8080/boot/v1/bootscript?host=x0c0s0b0n0&profile=compute"

# Login node requests boot script for login profile
curl "http://boot-service:8080/boot/v1/bootscript?host=login0&profile=login"

# Unknown node (or no profile specified) gets default
curl "http://boot-service:8080/boot/v1/bootscript?host=unknown-node"
```

## Creating Boot Configurations

### Overview

Boot configurations are created as BootConfiguration resources using the modern REST API. Each configuration specifies:

- **Targeting criteria** (`hosts`, `macs`, `nids`, `groups`): How to identify which nodes use this config
- **Profile** (optional): Name for logical grouping (leave empty for default profile)
- **Boot parameters** (`kernel`, `initrd`, `params`): What to boot and with which arguments
- **Priority** (optional): Tiebreaker when multiple configs match a node

### Creating via Modern API (Recommended)

Use the `POST /bootconfigurations` endpoint with a JSON body:

```bash
curl -X POST http://boot-service:8080/bootconfigurations \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {
      "name": "my-config-name"
    },
    "spec": {
      "profile": "compute",
      "hosts": ["x0c0s*"],
      "kernel": "http://files.openchami.org/vmlinuz",
      "initrd": "http://files.openchami.org/initramfs.img",
      "params": "console=ttyS0,115200 root=/dev/ram0",
      "priority": 50
    }
  }'
```

**Response** (successful creation):
```json
{
  "apiVersion": "boot.openchami.io/v1",
  "kind": "BootConfiguration",
  "metadata": {
    "name": "my-config-name",
    "uid": "boo-a1b2c3d4"
  },
  "spec": {
    "profile": "compute",
    "hosts": ["x0c0s*"],
    "kernel": "http://files.openchami.org/vmlinuz",
    "initrd": "http://files.openchami.org/initramfs.img",
    "params": "console=ttyS0,115200 root=/dev/ram0",
    "priority": 50
  }
}
```

### BootConfiguration Spec Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `profile` | string | No | Profile name for grouping configs. Empty/omitted = default profile. |
| `hosts` | []string | No | XName patterns (e.g., `"x0c0s*"`, `"login[0-9]"`) |
| `macs` | []string | No | MAC addresses (e.g., `"aa:bb:cc:dd:ee:ff"`) |
| `nids` | []int | No | Numeric node IDs (e.g., `42`, `100`) |
| `groups` | []string | No | Inventory group memberships |
| `kernel` | string | **Yes** | URL or path to kernel image |
| `initrd` | string | No | URL or path to initrd/initramfs image |
| `params` | string | No | Kernel parameters (console, root, etc.) |
| `priority` | int | No | Priority for tiebreaking (0-100). Higher = takes precedence. |

**Note**: At least one targeting criterion (`hosts`, `macs`, `nids`, or `groups`) should be specified. If none are specified, the config acts as a catch-all default.

### Examples

#### Example 1: Default Configuration (Fallback)

This config has NO profile, so it's the "default" profile.

**JSON (for REST API)**:
```json
{
  "metadata": {"name": "default-boot"},
  "spec": {
    "kernel": "http://files.openchami.org/vmlinuz-generic",
    "initrd": "http://files.openchami.org/initramfs-generic.img",
    "params": "console=ttyS0,115200 root=/dev/ram0",
    "priority": 1
  }
}
```

**YAML (for documentation)**:
```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: default-boot
spec:
  # No 'profile' field - this is DEFAULT
  kernel: "http://files.openchami.org/vmlinuz-generic"
  initrd: "http://files.openchami.org/initramfs-generic.img"
  params: "console=ttyS0,115200 root=/dev/ram0"
  priority: 1
```

When used: Returned as fallback when requested profile doesn't exist or doesn't match the node.

#### Example 2: Compute Profile

Matches compute nodes by XName pattern.

**JSON**:
```json
{
  "metadata": {"name": "compute-standard"},
  "spec": {
    "profile": "compute",
    "hosts": ["x0c0s*"],
    "kernel": "http://files.openchami.org/vmlinuz-compute",
    "initrd": "http://files.openchami.org/initramfs-compute.img",
    "params": "console=ttyS0,115200 root=/dev/ram0 cgroup_memory=1",
    "priority": 50
  }
}
```

**YAML**:
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

When used: When a compute node requests `?profile=compute`.

#### Example 3: Login Node Profile

Matches login nodes by pattern.

**JSON**:
```json
{
  "metadata": {"name": "login-standard"},
  "spec": {
    "profile": "login",
    "hosts": ["login[0-9]"],
    "kernel": "http://files.openchami.org/vmlinuz-login",
    "initrd": "http://files.openchami.org/initramfs-login.img",
    "params": "console=ttyS0,115200 root=/dev/nvme0n1p1",
    "priority": 50
  }
}
```

**YAML**:
```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: login-standard
spec:
  profile: "login"
  hosts:
    - "login[0-9]"
  kernel: "http://files.openchami.org/vmlinuz-login"
  initrd: "http://files.openchami.org/initramfs-login.img"
  params: "console=ttyS0,115200 root=/dev/nvme0n1p1"
  priority: 50
```

When used: When a login node requests `?profile=login`.

#### Example 4: Debug Profile (MAC-Specific)

Matches a specific node by MAC address for debugging.

**JSON**:
```json
{
  "metadata": {"name": "debug-mode"},
  "spec": {
    "profile": "debug",
    "macs": ["aa:bb:cc:dd:ee:ff"],
    "kernel": "http://files.openchami.org/vmlinuz-debug",
    "initrd": "http://files.openchami.org/initramfs-debug.img",
    "params": "console=ttyS0,115200 root=/dev/ram0 rd.break=pre-mount systemd.log_level=debug",
    "priority": 100
  }
}
```

**YAML**:
```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: debug-mode
spec:
  profile: "debug"
  macs:
    - "aa:bb:cc:dd:ee:ff"
  kernel: "http://files.openchami.org/vmlinuz-debug"
  initrd: "http://files.openchami.org/initramfs-debug.img"
  params: "console=ttyS0,115200 root=/dev/ram0 rd.break=pre-mount systemd.log_level=debug"
  priority: 100
```

When used: When the node with MAC `aa:bb:cc:dd:ee:ff` requests `?profile=debug`. The high priority ensures this config is chosen.

#### Example 5: Profile with Group Membership

Matches nodes that belong to specific inventory groups.

**JSON**:
```json
{
  "metadata": {"name": "compute-gpu"},
  "spec": {
    "profile": "compute",
    "groups": ["gpu-enabled", "hpc-cluster"],
    "kernel": "http://files.openchami.org/vmlinuz-compute-gpu",
    "initrd": "http://files.openchami.org/initramfs-compute-gpu.img",
    "params": "console=ttyS0,115200 root=/dev/ram0 nvidia_drm.modeset=1",
    "priority": 75
  }
}
```

**YAML**:
```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: compute-gpu
spec:
  profile: "compute"
  groups:
    - "gpu-enabled"
    - "hpc-cluster"
  kernel: "http://files.openchami.org/vmlinuz-compute-gpu"
  initrd: "http://files.openchami.org/initramfs-compute-gpu.img"
  params: "console=ttyS0,115200 root=/dev/ram0 nvidia_drm.modeset=1"
  priority: 75
```

When used: When a node that's a member of both `gpu-enabled` and `hpc-cluster` groups requests `?profile=compute`.

## Requesting Boot Scripts

### Legacy API Endpoint (`/boot/v1/bootscript`)

The legacy BSS-compatible endpoint returns an iPXE boot script. Use the `profile` query parameter to request a specific profile:

```bash
# Request compute profile with MAC address
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute"

# Request debug profile with XName
curl "http://boot-service:8080/boot/v1/bootscript?host=x0c0s0b0n0&profile=debug"

# Request login profile with NID
curl "http://boot-service:8080/boot/v1/bootscript?nid=42&profile=login"

# Request default profile (omit profile parameter or use &profile=default)
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff"
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=default"
```

**Response**: An iPXE script that can be executed by the boot firmware:

```ipxe
#!ipxe
echo Starting boot sequence for x0c0s0b0n0
set base-url http://files.openchami.org
kernel ${base-url}/vmlinuz-compute root=/dev/ram0 console=ttyS0,115200 cgroup_memory=1
initrd ${base-url}/initramfs-compute.img
boot
```

### Node Identification Methods

You can identify the requesting node in three ways. Provide exactly one:

| Method | Parameter | Format | Example |
|--------|-----------|--------|----------|
| **MAC Address** | `?mac=` | Standard MAC format | `?mac=aa:bb:cc:dd:ee:ff` |
| **XName** (Cray) | `?host=` | Cray hardware naming | `?host=x0c0s0b0n0` |
| **NID** (Numeric ID) | `?nid=` | Integer node ID | `?nid=42` |

### Using with PXE

In your DHCP/PXE configuration, reference this endpoint with dynamic node identifiers:

```ipxe
# Standard boot with node's MAC address
chain http://boot-service:8080/boot/v1/bootscript?mac=${net0/mac}&profile=compute

# Debug boot (if special boot flag is set)
chain http://boot-service:8080/boot/v1/bootscript?mac=${net0/mac}&profile=debug

# No profile specified - uses default profile
chain http://boot-service:8080/boot/v1/bootscript?mac=${net0/mac}
```

### Modern API Endpoint (`GET /bootconfigurations`)

The modern REST API also supports the profile concept:

```bash
# List ALL boot configurations
curl "http://boot-service:8080/bootconfigurations"

# List configurations for a specific profile (requires filtering)
# Note: The API doesn't have a built-in profile filter; use client-side filtering
curl "http://boot-service:8080/bootconfigurations" | jq '.[] | select(.spec.profile == "compute")'

# Get a specific configuration by UID
curl "http://boot-service:8080/bootconfigurations/boo-a1b2c3d4"
```

## Profile Selection Algorithm

When you request a boot script, the boot service follows this algorithm:

**Step 1: Filter by Profile**
- If no profile requested: target = "default"
- If profile requested: target = requested profile
- Find all configs with `spec.profile == target` (or `spec.profile == ""` if default)

**Step 2: Score Each Candidate**

For each configuration matching the profile, calculate a score based on how well the targeting criteria match your node:

| Matching Criterion | Points | Notes |
|--------------------|--------|-------|
| Exact MAC match | **100** | Highest priority |
| NID match | **75** | Direct numeric ID match |
| Host/XName pattern match | **50** | Regex or glob pattern |
| Group membership (per group) | **25** | Cumulative for multiple groups |
| Catch-all (no criteria) | **1** | Lowest; used as universal fallback |

**Step 3: Select Best Match**
- Sort candidates by: Score (descending) → Priority (descending)
- Return the highest-ranked configuration

**Step 4: Fallback**
- If requested profile had matches: Return best match from Step 2
- If requested profile had NO matches:
  - Repeat search with target = "default" profile
  - If default matches found: Return best match from default
  - If NO default found: Return error

**Example with Data**:

**Scenario 1: Exact MAC Match**

Given:
- Config A: profile="compute", macs=["aa:bb:cc:dd:ee:ff"], priority=50
- Config B: profile="compute", hosts=["x0c0s*"], priority=50
- Config C: profile="" (default), priority=1

Request: `?mac=aa:bb:cc:dd:ee:ff&profile=compute`

Step 1: Filter profile → Only A and B (profile="compute")
Step 2: Score:
  - Config A: 100 (MAC exact match)
  - Config B: 50 (XName pattern doesn't match aa:bb:cc:dd:ee:ff)
Step 3: Select → **Config A** (100 > 50)

**Scenario 2: Pattern Match**

Same configs as above.

Request: `?host=x0c0s0b0n0&profile=compute`

Step 1: Filter profile → Only A and B (profile="compute")
Step 2: Score:
  - Config A: 0 (MAC "aa:bb:cc:dd:ee:ff" doesn't match node x0c0s0b0n0)
  - Config B: 50 (pattern "x0c0s*" matches x0c0s0b0n0)
Step 3: Select → **Config B** (50 > 0)

**Scenario 3: Fallback to Default**

Same configs as above.

Request: `?mac=aa:bb:cc:dd:ee:ff&profile=storage`

Step 1: Filter profile → No configs with profile="storage"
Step 2: No candidates for storage
Step 4: Fallback to default profile
  - Config C: profile="" (default), score=1
Step 3: Select → **Config C** (default fallback)

**Scenario 4: No Default Profile Error**

Given:
- Config A: profile="compute", hosts=["x0c0s*"], priority=50
- NO default config exists

Request: `?host=login0&profile=compute`

Step 1: Filter profile → Only A (profile="compute")
Step 2: Score:
  - Config A: 0 (pattern "x0c0s*" doesn't match node login0)
Step 3: No match
Step 4: Fallback to default → No default config exists
Step 4: **Error** - "no matching configurations found"

## API Reference

### Creating Boot Configurations

**Endpoint**: `POST /bootconfigurations`

**Request Body**:
```json
{
  "metadata": {
    "name": "config-name"
  },
  "spec": {
    "profile": "compute",
    "hosts": ["x0c0s*"],
    "macs": [],
    "nids": [],
    "groups": [],
    "kernel": "http://files.openchami.org/vmlinuz",
    "initrd": "http://files.openchami.org/initramfs.img",
    "params": "console=ttyS0,115200 root=/dev/ram0",
    "priority": 50
  }
}
```

**Example**:
```bash
curl -X POST http://boot-service:8080/bootconfigurations \
  -H "Content-Type: application/json" \
  -d '{"metadata":{"name":"my-config"},"spec":{"profile":"compute","hosts":["x0c0s*"],"kernel":"http://example.com/vmlinuz"}}'
```

**Response**: `201 Created` with full resource representation.

### Listing Boot Configurations

**Endpoint**: `GET /bootconfigurations`

**Query Parameters**: None (filtering must be done client-side)

**Example**:
```bash
curl "http://boot-service:8080/bootconfigurations"
```

**Response**: `200 OK` with array of all BootConfiguration resources.

### Getting a Specific Configuration

**Endpoint**: `GET /bootconfigurations/{uid}`

**Example**:
```bash
curl "http://boot-service:8080/bootconfigurations/boo-a1b2c3d4"
```

**Response**: `200 OK` with the specific BootConfiguration resource, or `404 Not Found`.

### Updating Boot Configurations

**Endpoint**: `PATCH /bootconfigurations/{uid}`

**Request Body** (partial update - include only fields to change):
```json
{
  "spec": {
    "priority": 75
  }
}
```

**Example** (change profile priority):
```bash
curl -X PATCH http://boot-service:8080/bootconfigurations/boo-a1b2c3d4 \
  -H "Content-Type: application/json" \
  -d '{"spec":{"priority":75}}'
```

**Example** (change profile name):
```bash
curl -X PATCH http://boot-service:8080/bootconfigurations/boo-a1b2c3d4 \
  -H "Content-Type: application/json" \
  -d '{"spec":{"profile":"login"}}'
```

**Response**: `200 OK` with updated resource.

### Deleting Boot Configurations

**Endpoint**: `DELETE /bootconfigurations/{uid}`

**Example**:
```bash
curl -X DELETE "http://boot-service:8080/bootconfigurations/boo-a1b2c3d4"
```

**Response**: `204 No Content` on success, or `404 Not Found` if already deleted.

## Best Practices

### Profile Naming

Choose clear, descriptive profile names:

- ✅ `compute`, `login`, `storage`, `management`
- ✅ `debug`, `maintenance`, `production`
- ❌ `v1`, `type-a`, `special`

### Configuration Organization

1. **Always create a default profile**: Define at least one BootConfiguration with an empty `profile` field as your fallback
2. **Use consistent profile naming**: Choose clear names like `compute`, `login`, `storage`, `debug` (not `v1`, `type-a`, etc.)
3. **Name configurations descriptively**: Use metadata names like `compute-standard`, `login-production` to indicate purpose
4. **Version boot images**: Include version info in kernel/initrd URLs (e.g., `vmlinuz-5.10.0-v2`)
5. **Set reasonable priorities**: Default=1, standard profiles=50, specialized/debug=100
6. **Document profile usage**: Comment in configs or keep external docs of what each profile is for

### Priority Field Guidelines

The `priority` field breaks ties when multiple configs have the same score. Suggested values:

| Profile Type | Priority | Rationale |
|---|---|---|
| Default/Fallback | 1 | Lowest; only used when nothing else matches |
| Standard production | 50 | Normal operation |
| Specialized (GPU, storage) | 75 | More specific than standard |
| Debug/Maintenance | 100 | Highest; forces selection when you need it |

**Note**: The targeting criteria score (MAC=100, NID=75, pattern=50, group=25) usually dominates priority. Priority mainly matters when scores are identical.

### Group Usage with Profiles

Combine profiles with groups for powerful organization:

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: compute-hpc-optimized
spec:
  profile: "compute"
  groups:
    - "hpc-cluster"           # Apply only to HPC cluster nodes
    - "gpu-enabled"           # Further filter to GPU nodes
  kernel: "http://files.openchami.org/vmlinuz-hpc"
  initrd: "http://files.openchami.org/initramfs-hpc.img"
  params: "console=ttyS0,115200 root=/dev/ram0 numa=on acpi=on"
  priority: 75
```

## Troubleshooting

### "No matching configurations found"

**Error**: You request a boot script but get this error.

**Cause**: Either:
1. Requested profile has no configurations that match the node's targeting criteria
2. No default profile exists to fall back to

**Solution**:
1. Check that a default configuration exists (profile field empty or omitted)
   ```bash
   curl http://boot-service:8080/bootconfigurations | jq '.[] | select(.spec.profile == "")'
   ```
2. Verify configurations exist for the requested profile:
   ```bash
   curl http://boot-service:8080/bootconfigurations | jq '.[] | select(.spec.profile == "compute")'
   ```
3. Check that your node matches the targeting criteria:
   - For MAC: compare node's MAC with `spec.macs`
   - For XName: compare node's XName with `spec.hosts` patterns
   - For NID: compare node's NID with `spec.nids`
   - For groups: check node membership in `spec.groups`

### Wrong Profile Being Used

**Symptom**: You request `?profile=compute` but get a different profile's boot script.

**Diagnosis** - Check in this order:

1. **Verify request parameters**:
   ```bash
   # Did you include the profile parameter?
   curl -v "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute" 2>&1 | grep -i profile
   ```

2. **Check what configurations exist**:
   ```bash
   curl "http://boot-service:8080/bootconfigurations" | jq '.[] | {name: .metadata.name, profile: .spec.profile}'
   ```

3. **Verify targeting criteria match**:
   ```bash
   # List compute profile configs
   curl "http://boot-service:8080/bootconfigurations" | jq '.[] | select(.spec.profile == "compute") | {name, hosts: .spec.hosts, macs: .spec.macs}'

   # Does your node's MAC/XName match the criteria?
   ```

4. **Check scoring** - If multiple configs match, which scores highest?
   ```bash
   # Look at both profile and priority
   curl "http://boot-service:8080/bootconfigurations" | jq '.[] | {name: .metadata.name, profile: .spec.profile, priority: .spec.priority}'
   ```

**Common issues**:
- Profile name typo (e.g., `compue` vs `compute`)
- Node doesn't match targeting criteria (wrong MAC, XName pattern doesn't apply)
- Multiple configs with same score; the one with higher priority was selected
- Requesting non-existent profile without a default profile to fall back to

### Profile Doesn't Exist - Falling Back to Default

**Symptom**: You request `?profile=special` but get the default profile instead.

**This is expected behavior**. Here's what happens:

```
Request: ?profile=special
↓
System looks for configs with profile="special"
↓
None found
↓
Fallback to profile="" (default)
↓
Return default profile config
```

**Why this is good**: Provides resilience. If a profile doesn't exist, nodes don't fail—they get a reasonable default.

**If you need to enforce specific profiles**:
1. Set up each required profile with at least one configuration
2. Make sure no default profile exists (or set its priority very low, e.g., 0)
3. Alternative: Use a catch-all config with very specific priority/criteria to catch unmatched nodes

### Verifying Which Configuration Is Being Used

Boot scripts include comments identifying the node and config:

```bash
# Request the boot script
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute"

# Output will include iPXE script with kernel/initrd URLs showing which config was selected
```

**Alternative**: Query the configurations directly and review which matches:

```bash
# Get all configs, filter to your profile
curl "http://boot-service:8080/bootconfigurations" | jq '.[] | select(.spec.profile == "compute")'

# Look at kernel URLs in returned configs to identify which you should be using
```

### Default Profile Creation Checklist

When setting up the boot service, ensure you have at least one default configuration:

- [ ] Created at least one BootConfiguration with empty `profile` field (or no profile field)
- [ ] Default config has reasonable `kernel` and `params` values
- [ ] Default config priority is low (recommend 1-10) so specific profiles take precedence
- [ ] Verified default config exists:
   ```bash
   curl http://boot-service:8080/bootconfigurations | jq '.[] | select(.spec.profile == "")'
   ```
- [ ] If result is empty: Default profile doesn't exist! Create one immediately.

## Migration Guide

### From Non-Profiled Setup

If you have existing boot configurations without profiles:

**Step 1**: Current state
- Existing configs have no `profile` field
- These are automatically treated as "default" profile
- Everything continues to work as-is

**Step 2**: Add profiles gradually
- New configurations can include explicit `profile` fields
- Existing configs without profiles remain as default
- No breaking changes

**Step 3**: Update existing configs (optional)

Add a profile to organize better:

```bash
# First, get the UID of the config to update
curl http://boot-service:8080/bootconfigurations | jq '.[] | select(.metadata.name == "old-config") | .metadata.uid'

# Then PATCH to add profile
curl -X PATCH http://boot-service:8080/bootconfigurations/{uid} \
  -H "Content-Type: application/json" \
  -d '{"spec": {"profile": "compute"}}'
```

### Backward Compatibility

- Existing clients requesting `?profile=` on old configs: Works (matched against empty profile = default)
- Existing clients requesting no profile: Works (defaults to default profile)
- No API changes needed

## See Also

- [CONFIGURATION.md](CONFIGURATION.md) - Service configuration options
- [AUTHENTICATION.md](AUTHENTICATION.md) - JWT and authentication setup
- `examples/` directory - Sample YAML configurations and curl commands
