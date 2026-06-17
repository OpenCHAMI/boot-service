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
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   resource.Metadata `json:"metadata" yaml:"metadata"`
	Spec       BMCSpec           `json:"spec" yaml:"spec" validate:"required"`
	Status     BMCStatus         `json:"status,omitempty" yaml:"status,omitempty"`
}

// BMCSpec defines the desired state of BMC.
type BMCSpec struct { // nolint:revive
	XName       string       `json:"xname,omitempty" yaml:"xname,omitempty"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty" validate:"max=200"`
	Interface   BMCInterface `json:"interface,omitempty" yaml:"interface,omitempty"`
}

// BMCInterface defines the Ethernet interface details.
type BMCInterface struct {
	MAC  string `json:"mac,omitempty" yaml:"mac,omitempty"`
	IP   string `json:"ip,omitempty" yaml:"ip,omitempty"`
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
}

// BMCStatus defines the observed state of BMC.
type BMCStatus struct { // nolint:revive
	Phase   string `json:"phase,omitempty" yaml:"phase,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	Ready   bool   `json:"ready" yaml:"ready"`
}

// Validate implements custom validation logic for BMC.
func (r *BMC) Validate(ctx context.Context) error { //nolint:revive,unused
	_ = ctx
	return nil
}
