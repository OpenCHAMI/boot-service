// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package bmc

import (
	"context"

	"github.com/openchami/fabrica/pkg/resource"
)

// bmc represents a bmc resource
type BMC struct {
	resource.Resource
	Spec   BMCSpec   `json:"spec" validate:"required"`
	Status BMCStatus `json:"status,omitempty"`
}

// BMCSpec defines the desired state of BMC
type BMCSpec struct {
	XName       string    `json:"xname,omitempty"`
	Description string    `json:"description,omitempty" validate:"max=200"`
	Interface   Interface `json:"interface,omitempty"`
}

type Interface struct {
	MAC  string `json:"mac,omitempty"`
	IP   string `json:"ip,omitempty"`
	Type string `json:"type,omitempty"` // e.g., "management", "data"
}

// BMCStatus defines the observed state of BMC
type BMCStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
	Ready   bool   `json:"ready"`
	// Add your status fields here
}

// Validate implements custom validation logic for BMC
func (r *BMC) Validate(ctx context.Context) error {
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
