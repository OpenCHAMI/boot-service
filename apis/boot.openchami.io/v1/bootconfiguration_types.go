// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"errors"

	bootvalidation "github.com/openchami/boot-service/pkg/validation"
	"github.com/openchami/fabrica/pkg/resource"
)

// BootConfiguration represents a BootConfiguration resource.
type BootConfiguration struct {
	APIVersion string                  `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                  `json:"kind" yaml:"kind"`
	Metadata   resource.Metadata       `json:"metadata" yaml:"metadata"`
	Spec       BootConfigurationSpec   `json:"spec" yaml:"spec"`
	Status     BootConfigurationStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// BootConfigurationSpec defines the desired state of BootConfiguration.
//
// A boot configuration specifies the kernel, initrd, and parameters needed to boot a node,
// along with targeting criteria (hosts, MACs, NIDs, groups) to determine which nodes use
// this configuration.
//
// Profile support allows organizing configurations by operational scenario (e.g., "compute",
// "debug", "login"). When requesting a boot script with a specific profile, the service
// selects from matching profile configurations. If no matching profile exists, it falls back
// to the default profile (empty profile field).
//
// Selection priority: exact MAC match (100) > NID match (75) > host pattern (50) >
// group membership (25) > default (1). When scores tie, the Priority field determines selection.
// See docs/PROFILES.md for comprehensive profile documentation.
type BootConfigurationSpec struct { // nolint:revive
	// Node targeting criteria (at least one required)
	Hosts  []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`   // XName patterns (e.g., "x0c0s*")
	MACs   []string `json:"macs,omitempty" yaml:"macs,omitempty"`     // MAC addresses (case-insensitive)
	NIDs   []int32  `json:"nids,omitempty" yaml:"nids,omitempty"`     // Numeric node IDs
	Groups []string `json:"groups,omitempty" yaml:"groups,omitempty"` // Inventory group memberships

	// Boot profile for organizing configurations
	// Empty or "default" indicates the default fallback profile.
	// See docs/PROFILES.md for profile usage and selection logic.
	Profile string `json:"profile,omitempty" yaml:"profile,omitempty"`

	// Boot parameters
	Kernel string `json:"kernel" yaml:"kernel"`                     // Required: kernel URL or path
	Initrd string `json:"initrd,omitempty" yaml:"initrd,omitempty"` // Optional: initrd/initramfs URL or path
	Params string `json:"params,omitempty" yaml:"params,omitempty"` // Kernel parameters (console, root, etc.)

	// Priority for tiebreaking within the same profile when multiple configs match
	// Higher values take precedence. Default configurations typically use priority 1.
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`
}

// BootConfigurationStatus defines the observed state of BootConfiguration.
type BootConfigurationStatus struct { // nolint:revive
	Phase       string   `json:"phase,omitempty" yaml:"phase,omitempty"` // Active, Pending, Failed
	LastUpdated string   `json:"lastUpdated,omitempty" yaml:"lastUpdated,omitempty"`
	AppliedTo   []string `json:"appliedTo,omitempty" yaml:"appliedTo,omitempty"`
	Error       string   `json:"error,omitempty" yaml:"error,omitempty"`
}

// Validate implements custom validation logic for BootConfiguration.
func (r *BootConfiguration) Validate(ctx context.Context) error { //nolint:revive,unused
	_ = ctx

	if r.Spec.Kernel == "" {
		return errors.New("kernel field is required")
	}

	// Note: Targeting criteria (hosts, macs, nids, groups) are all optional.
	// Configurations with no targeting criteria act as catch-all defaults (score=1 in selection).
	// This is intentional for supporting default profile configurations that apply when
	// more specific profiles don't match. See docs/PROFILES.md for details.

	for _, host := range r.Spec.Hosts {
		if !bootvalidation.ValidateXNameOrDefault(host) {
			return errors.New("invalid host XName format: " + host)
		}
	}

	for _, mac := range r.Spec.MACs {
		if !bootvalidation.ValidateMAC(mac) {
			return errors.New("invalid MAC address format: " + mac)
		}
	}

	if !bootvalidation.ValidateURLOrPath(r.Spec.Kernel) {
		return errors.New("invalid kernel URL or path: " + r.Spec.Kernel)
	}

	if r.Spec.Initrd != "" && !bootvalidation.ValidateURLOrPathOptional(r.Spec.Initrd) {
		return errors.New("invalid initrd URL or path: " + r.Spec.Initrd)
	}

	if r.Spec.Priority < 0 || r.Spec.Priority > 100 {
		return errors.New("priority must be between 0 and 100")
	}

	return nil
}
