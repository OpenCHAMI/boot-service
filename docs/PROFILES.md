<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Boot Profiles Guide

This document describes how to use boot profiles to manage different boot configurations for different node types or operational scenarios.

## Overview

Boot profiles allow you to organize boot configurations into logical groups, enabling:

- **Different boot environments**: Compute nodes, storage nodes, login nodes, management nodes
- **Operational scenarios**: Standard production boot, maintenance/debug boot, specialized workload boot
- **Configuration management**: Keep related boot parameters organized and maintainable
- **Dynamic selection**: Choose the profile at runtime based on operational needs

### Default Profile

If no profile is specified in a boot configuration or in a boot script request, the **default profile** is used. This ensures backward compatibility and provides a fallback when specific profiles are unavailable.

## Creating Boot Configurations with Profiles

### Basic Example

Boot configurations are managed as Kubernetes-style resources. Each configuration can specify a profile:

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: compute-standard
spec:
  profile: "compute"           # Profile name (optional)
  hosts:
    - "x0c0s[0-9].*"          # XName pattern for compute nodes
  macs: []
  nids: []
  groups: []
  kernel: "http://files.openchami.org/vmlinuz-5.10.0"
  initrd: "http://files.openchami.org/initramfs-5.10.0.img"
  params: "console=ttyS0,115200 root=/dev/ram0 systemd.unit=multi-user.target"
  priority: 50
```

### Profile Examples

#### Compute Profile

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: compute-standard
spec:
  profile: "compute"
  hosts:
    - "x0c0s*"                # Match all compute slots
  kernel: "http://files.openchami.org/vmlinuz-compute"
  initrd: "http://files.openchami.org/initramfs-compute.img"
  params: "console=ttyS0,115200 root=/dev/ram0 cgroup_memory=1"
  priority: 75
```

#### Login Node Profile

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
  priority: 80
```

#### Management/Debug Profile

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: debug-mode
spec:
  profile: "debug"
  macs:
    - "aa:bb:cc:dd:ee:ff"     # Specific node for debugging
  kernel: "http://files.openchami.org/vmlinuz-debug"
  initrd: "http://files.openchami.org/initramfs-debug.img"
  params: "console=ttyS0,115200 root=/dev/ram0 rd.break=pre-mount systemd.log_level=debug"
  priority: 100                 # Higher priority to force selection
```

#### Default Profile

```yaml
apiVersion: boot.openchami.io/v1
kind: BootConfiguration
metadata:
  name: default-boot
spec:
  # No profile specified - this is the default
  kernel: "http://files.openchami.org/vmlinuz-generic"
  initrd: "http://files.openchami.org/initramfs-generic.img"
  params: "console=ttyS0,115200 root=/dev/ram0"
  priority: 1                   # Low priority, used as fallback
```

## Requesting Boot Scripts with Profiles

### iPXE Endpoint

The `/boot/v1/bootscript` endpoint supports a `profile` query parameter to request a specific profile:

```bash
# Request compute profile
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute"

# Request debug profile
curl "http://boot-service:8080/boot/v1/bootscript?host=x0c0s0b0n0&profile=debug"

# Request with NID
curl "http://boot-service:8080/boot/v1/bootscript?nid=42&profile=login"

# Use default profile (omit profile parameter)
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff"

# Explicitly request default
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=default"
```

### Node Identification Methods

The endpoint supports three ways to identify nodes:

1. **MAC address** (most common for PXE):
   ```
   ?mac=aa:bb:cc:dd:ee:ff
   ```

2. **XName** (Cray hardware naming):
   ```
   ?host=x0c0s0b0n0
   ```

3. **NID** (numeric node ID):
   ```
   ?nid=42
   ```

### Using with PXE

In your PXE configuration, you can pass the profile dynamically:

```ipxe
# Standard boot
chain http://boot-service:8080/boot/v1/bootscript?mac=${net0/mac}&profile=compute

# Debug boot (if debug flag is set)
chain http://boot-service:8080/boot/v1/bootscript?mac=${net0/mac}&profile=debug
```

## Profile Selection Logic

When you request a boot script with a specific profile, the boot service uses this algorithm:

1. **Exact profile match**: Look for configurations with the requested profile
2. **Score based matching**: Among matching profiles, select based on:
   - MAC address match: 100 points (exact match)
   - NID match: 75 points
   - Host/XName pattern: 50 points
   - Group membership: 25 points per group
   - Default configuration: 1 point
3. **Priority tiebreaker**: If multiple configs have the same score, use the `priority` field
4. **Fallback to default**: If requested profile has no matches, fall back to default profile
5. **Error if no default**: Return error only if no default profile exists

### Example Selection Scenario

Given these configurations:

```yaml
# Config A: compute profile with exact MAC match
Profile: compute
MACs: [aa:bb:cc:dd:ee:ff]
Priority: 50
# Score: 100 (MAC) = 100 points

# Config B: compute profile with XName pattern
Profile: compute
Hosts: [x0c0s*]
Priority: 50
# Score: 50 (pattern) = 50 points

# Config C: default profile (fallback)
Profile: ""
Priority: 1
# Score: 1 (default) = 1 point
```

When requesting `?mac=aa:bb:cc:dd:ee:ff&profile=compute`:
- **Result**: Config A selected (100 points > 50 points)

When requesting `?host=x0c0s0b0n0&profile=compute`:
- **Result**: Config B selected (50 points for compute profile match)

When requesting `?mac=aa:bb:cc:dd:ee:ff&profile=storage`:
- **Result**: Default profile (no storage configs, fallback to default)

## Best Practices

### Profile Naming

Choose clear, descriptive profile names:

- ✅ `compute`, `login`, `storage`, `management`
- ✅ `debug`, `maintenance`, `production`
- ❌ `v1`, `type-a`, `special`

### Configuration Organization

1. **Create a default profile**: Always define at least one default configuration
2. **Use consistent naming**: Match your organizational structure
3. **Document purpose**: Use metadata annotations if your system supports them
4. **Version boot images**: Include version numbers in kernel/initrd paths

### Priority Management

- Default profile: priority `1` (lowest)
- Standard profiles: priority `50`
- Specialized/debug profiles: priority `100`
- Exact matches: Naturally have higher scores regardless of priority

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

### Getting the Wrong Profile

Check in this order:

1. **Verify request**: Is the `profile` parameter correct?
   ```bash
   curl -I "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute"
   ```

2. **Check configurations exist**: List all boot configurations in your system
   ```bash
   curl "http://boot-service:8080/bootconfigurations"
   ```

3. **Verify profile field**: Ensure configurations have the correct profile name
   ```bash
   curl "http://boot-service:8080/bootconfigurations" | jq '.[] | {name, profile: .spec.profile}'
   ```

4. **Review scoring**: Multiple configs with same profile? Check priorities and host patterns

### Profile Not Found Falls to Default

This is the expected behavior:

- Requested profile: `compute`
- Available profiles: only `default`
- **Result**: Returns default profile

This is intentional for resilience. If you need to enforce a specific profile, use `priority` to make default config lower priority than production configs.

### Debugging Boot Script Content

To see which configuration is being used:

```bash
# Get and inspect the script
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute" | head -20

# Look for kernel/initrd URLs to identify which config was selected
curl "http://boot-service:8080/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute" | grep -E "kernel|initrd"
```

## Migration from Non-Profiled Setup

If you have existing boot configurations without profiles:

1. **No action needed**: Configurations without a profile are treated as `default` profile
2. **Gradual adoption**: Add profiles to new configurations as needed
3. **Backward compatibility**: Existing clients continue to work unchanged
4. **Optional migration**: Gradually add profiles to existing configurations:

```bash
# Before: no profile field
PATCH /bootconfigurations/config-id
spec:
  kernel: "..."
  initrd: "..."

# After: add profile to organize
PATCH /bootconfigurations/config-id
spec:
  profile: "compute"            # New field
  kernel: "..."
  initrd: "..."
```

## See Also

- [API Documentation](API.md) - Boot configuration resource specification
- [Configuration Guide](CONFIGURATION.md) - Service configuration options
- Boot Service Repository Examples: `examples/` directory for sample YAML configurations
