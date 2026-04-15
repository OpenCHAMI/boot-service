#!/bin/bash
# SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors
# SPDX-License-Identifier: MIT
#
# Example commands for requesting boot scripts with profiles.
# See docs/PROFILES.md for comprehensive documentation.

# Set boot service URL
BOOT_SERVICE="http://localhost:8080"

# 1. Request boot script with compute profile (MAC address)
echo "=== Compute Profile (by MAC) ==="
curl -s "${BOOT_SERVICE}/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=compute" | head -20

# 2. Request boot script with login profile (XName/host)
echo -e "\n=== Login Profile (by XName) ==="
curl -s "${BOOT_SERVICE}/boot/v1/bootscript?host=x0c0s0b0n0&profile=login" | head -20

# 3. Request boot script with debug profile (NID)
echo -e "\n=== Debug Profile (by NID) ==="
curl -s "${BOOT_SERVICE}/boot/v1/bootscript?nid=42&profile=debug" | head -20

# 4. Request with default profile (empty profile parameter)
echo -e "\n=== Default Profile (empty profile) ==="
curl -s "${BOOT_SERVICE}/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=" | head -20

# 5. Request without profile parameter (implicit default)
echo -e "\n=== No Profile Parameter (defaults to default) ==="
curl -s "${BOOT_SERVICE}/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff" | head -20

# 6. Request that will fallback (profile doesn't exist)
echo -e "\n=== Non-existent Profile (falls back to default) ==="
curl -s "${BOOT_SERVICE}/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff&profile=nonexistent" | head -20

# API Response Examples
echo -e "\n=== Full Response (JSON from REST API for inspection) ==="
curl -s "${BOOT_SERVICE}/bootconfigurations" | head -50

# Hint: Use jq to filter profile information
echo -e "\n=== List all profile names in system ==="
curl -s "${BOOT_SERVICE}/bootconfigurations" 2>/dev/null | \
  jq -r '.[] | "\(.metadata.name): profile=\(.spec.profile // "default")"' || \
  echo "(Install jq to parse JSON: brew install jq)"
