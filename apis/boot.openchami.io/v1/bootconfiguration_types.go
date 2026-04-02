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
	APIVersion string                  `json:"apiVersion"`
	Kind       string                  `json:"kind"`
	Metadata   resource.Metadata       `json:"metadata"`
	Spec       BootConfigurationSpec   `json:"spec"`
	Status     BootConfigurationStatus `json:"status,omitempty"`
}

// BootConfigurationSpec defines the desired state of BootConfiguration.
type BootConfigurationSpec struct { // nolint:revive
	Hosts  []string `json:"hosts,omitempty"`
	MACs   []string `json:"macs,omitempty"`
	NIDs   []int32  `json:"nids,omitempty"`
	Groups []string `json:"groups,omitempty"`

	Kernel string `json:"kernel"`
	Initrd string `json:"initrd,omitempty"`
	Params string `json:"params,omitempty"`

	Priority int `json:"priority,omitempty"`
}

// BootConfigurationStatus defines the observed state of BootConfiguration.
type BootConfigurationStatus struct { // nolint:revive
	Phase       string   `json:"phase,omitempty"`
	LastUpdated string   `json:"lastUpdated,omitempty"`
	AppliedTo   []string `json:"appliedTo,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// Validate implements custom validation logic for BootConfiguration.
func (r *BootConfiguration) Validate(ctx context.Context) error { //nolint:revive,unused
	_ = ctx

	if r.Spec.Kernel == "" {
		return errors.New("kernel field is required")
	}

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

	if len(r.Spec.Hosts) == 0 && len(r.Spec.MACs) == 0 && len(r.Spec.NIDs) == 0 && len(r.Spec.Groups) == 0 {
		return errors.New("at least one targeting method (hosts, macs, nids, or groups) must be specified")
	}

	return nil
}
