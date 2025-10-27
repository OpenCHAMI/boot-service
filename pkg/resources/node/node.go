// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package node

import (
	"context"
	"errors"

	"github.com/openchami/boot-service/pkg/validation"
	"github.com/openchami/fabrica/pkg/resource"
)

// Node represents a Node resource
type Node struct {
	resource.Resource
	Spec   NodeSpec   `json:"spec"`
	Status NodeStatus `json:"status,omitempty"`
}

// NodeSpec defines the desired state of Node
type NodeSpec struct {
	XName      string      `json:"xname"`
	NID        int32       `json:"nid,omitempty"`
	BootMAC    string      `json:"bootMac,omitempty"`
	Role       string      `json:"role,omitempty"`
	SubRole    string      `json:"subRole,omitempty"`
	Hostname   string      `json:"hostname,omitempty"`
	Interfaces []Interface `json:"interfaces,omitempty"`
	Groups     []string    `json:"groups,omitempty"` // Groups from inventory service
}

type Interface struct {
	MAC  string `json:"mac,omitempty"`
	IP   string `json:"ip,omitempty"`
	Type string `json:"type,omitempty"` // e.g., "management", "data"
}

// NodeStatus defines the observed state of Node
type NodeStatus struct {
	LastBoot          string `json:"lastBoot,omitempty"`          // RFC3339 timestamp
	BootConfiguration string `json:"bootConfiguration,omitempty"` // Reference to active config
	State             string `json:"state,omitempty"`             // Ready, Booting, Failed
	LastHSMSync       string `json:"lastHSMSync,omitempty"`       // Last sync with HSM
	Error             string `json:"error,omitempty"`             // Error message if any
}

// Validate implements custom validation logic for Node
func (r *Node) Validate(ctx context.Context) error {
	// Validate XName format
	if !validation.ValidateXName(r.Spec.XName) {
		return errors.New("invalid XName format: " + r.Spec.XName)
	}

	// Validate BootMAC if provided
	if r.Spec.BootMAC != "" && !validation.ValidateMAC(r.Spec.BootMAC) {
		return errors.New("invalid BootMAC format: " + r.Spec.BootMAC)
	}

	return nil
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("Node", "nod")
}
