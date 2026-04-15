// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package v1 defines the boot.openchami.io/v1 Fabrica API resource types.
package v1

import (
	"context"

	"github.com/openchami/fabrica/pkg/resource"
)

// BMC represents a BMC resource.
type BMC struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   resource.Metadata `json:"metadata"`
	Spec       BMCSpec           `json:"spec" validate:"required"`
	Status     BMCStatus         `json:"status,omitempty"`
}

// BMCSpec defines the desired state of BMC.
type BMCSpec struct { // nolint:revive
	XName       string       `json:"xname,omitempty"`
	Description string       `json:"description,omitempty" validate:"max=200"`
	Interface   BMCInterface `json:"interface,omitempty"`
}

// BMCInterface defines the Ethernet interface details.
type BMCInterface struct {
	MAC  string `json:"mac,omitempty"`
	IP   string `json:"ip,omitempty"`
	Type string `json:"type,omitempty"`
}

// BMCStatus defines the observed state of BMC.
type BMCStatus struct { // nolint:revive
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
	Ready   bool   `json:"ready"`
}

// Validate implements custom validation logic for BMC.
func (r *BMC) Validate(ctx context.Context) error { //nolint:revive,unused
	_ = ctx
	return nil
}
