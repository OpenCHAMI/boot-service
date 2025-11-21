// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package bmc defines the BMC resource for the boot service
package bmc

import (
	"context"

	"github.com/openchami/fabrica/pkg/resource"
)

// BMC represents a BMC resource
type BMC struct {
	resource.Resource
	Spec   BMCSpec   `json:"spec" validate:"required"`
	Status BMCStatus `json:"status,omitempty"`
}

// BMCSpec defines the desired state of BMC
type BMCSpec struct { // nolint:revive
	XName       string    `json:"xname,omitempty"`
	Description string    `json:"description,omitempty" validate:"max=200"`
	Interface   Interface `json:"interface,omitempty"`
}

// Interface defines the Ethernet Interface details
type Interface struct {
	MAC  string `json:"mac,omitempty"`
	IP   string `json:"ip,omitempty"`
	Type string `json:"type,omitempty"` // e.g., "management", "data"
}

// BMCStatus defines the observed state of BMC
type BMCStatus struct { // nolint:revive
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
	Ready   bool   `json:"ready"`
	// Add your status fields here
}

// Validate implements custom validation logic for BMC
func (r *BMC) Validate(ctx context.Context) error { //nolint:revive
	// Add custom validation logic here
	// Example:
	// if r.Spec.Name == "forbidden" {
	//     return errors.New("name 'forbidden' is not allowed")
	// }

	return nil
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("BMC", "bmc")
}
