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

// Node represents a Node resource.
type Node struct {
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   resource.Metadata `json:"metadata" yaml:"metadata"`
	Spec       NodeSpec          `json:"spec" yaml:"spec"`
	Status     NodeStatus        `json:"status,omitempty" yaml:"status,omitempty"`
}

// NodeSpec defines the desired state of Node.
type NodeSpec struct { // nolint:revive
	XName      string          `json:"xname" yaml:"xname"`
	NID        int32           `json:"nid,omitempty" yaml:"nid,omitempty"`
	BootMAC    string          `json:"bootMac,omitempty" yaml:"bootMac,omitempty"`
	Role       string          `json:"role,omitempty" yaml:"role,omitempty"`
	SubRole    string          `json:"subRole,omitempty" yaml:"subRole,omitempty"`
	Hostname   string          `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Interfaces []NodeInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
	Groups     []string        `json:"groups,omitempty" yaml:"groups,omitempty"`
}

// NodeInterface represents a network interface.
type NodeInterface struct {
	MAC  string `json:"mac,omitempty" yaml:"mac,omitempty"`
	IP   string `json:"ip,omitempty" yaml:"ip,omitempty"`
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
}

// NodeStatus defines the observed state of Node.
type NodeStatus struct { // nolint:revive
	LastBoot          string `json:"lastBoot,omitempty" yaml:"lastBoot,omitempty"`
	BootConfiguration string `json:"bootConfiguration,omitempty" yaml:"bootConfiguration,omitempty"`
	State             string `json:"state,omitempty" yaml:"state,omitempty"`
	LastHSMSync       string `json:"lastHSMSync,omitempty" yaml:"lastHSMSync,omitempty"`
	Error             string `json:"error,omitempty" yaml:"error,omitempty"`
}

// Validate implements custom validation logic for Node.
func (r *Node) Validate(ctx context.Context) error { //nolint:revive,unused
	_ = ctx

	if !bootvalidation.ValidateXName(r.Spec.XName) {
		return errors.New("invalid XName format: " + r.Spec.XName)
	}

	if r.Spec.BootMAC != "" && !bootvalidation.ValidateMAC(r.Spec.BootMAC) {
		return errors.New("invalid BootMAC format: " + r.Spec.BootMAC)
	}

	return nil
}
