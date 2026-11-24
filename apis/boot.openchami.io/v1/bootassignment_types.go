// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"fmt"
	"net"

	bootvalidation "github.com/openchami/boot-service/pkg/validation"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/uid"
)

// BootAssignment selects nodes for one boot configuration reference.
type BootAssignment struct {
	APIVersion string               `json:"apiVersion" yaml:"apiVersion"`
	Kind       string               `json:"kind" yaml:"kind"`
	Metadata   resource.Metadata    `json:"metadata" yaml:"metadata"`
	Spec       BootAssignmentSpec   `json:"spec" yaml:"spec"`
	Status     BootAssignmentStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// BootAssignmentSpec defines the desired selector and revision reference.
type BootAssignmentSpec struct {
	Selector  *BootAssignmentSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
	Profile   string                  `json:"profile,omitempty" yaml:"profile,omitempty"`
	Priority  int                     `json:"priority,omitempty" yaml:"priority,omitempty"`
	ConfigRef revision.Reference      `json:"configRef" yaml:"configRef"`
}

// BootAssignmentSelector contains alternative exact-match selector families.
type BootAssignmentSelector struct {
	MACs   []string `json:"macs,omitempty" yaml:"macs,omitempty"`
	NIDs   []int32  `json:"nids,omitempty" yaml:"nids,omitempty"`
	XNames []string `json:"xnames,omitempty" yaml:"xnames,omitempty"`
	Groups []string `json:"groups,omitempty" yaml:"groups,omitempty"`
}

// BootAssignmentStatus contains non-deployment health information.
type BootAssignmentStatus struct {
	Phase string `json:"phase,omitempty" yaml:"phase,omitempty"`
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
}

// Validate checks the assignment and normalizes conventional MAC spellings.
func (r *BootAssignment) Validate(_ context.Context) error {
	if err := validateNewResourceName(r.Metadata.Name); err != nil {
		return fmt.Errorf("metadata.name: %w", err)
	}
	if err := r.Spec.ConfigRef.Validate(); err != nil {
		return fmt.Errorf("configRef: %w", err)
	}
	if err := validateNewResourceName(r.Spec.ConfigRef.Name); err != nil {
		return fmt.Errorf("configRef.name: %w", err)
	}
	if r.Spec.ConfigRef.RevisionName != "" {
		if err := ValidateRevisionName(r.Spec.ConfigRef.RevisionName); err != nil {
			return fmt.Errorf("configRef.revisionName: %w", err)
		}
	}
	if r.Spec.ConfigRef.UID != "" {
		parsed, err := (uid.Codec{}).Parse(r.Spec.ConfigRef.UID)
		if err != nil || parsed.Format != uid.FormatTypeID || parsed.Prefix != "rev" {
			return fmt.Errorf("invalid revision UID %q", r.Spec.ConfigRef.UID)
		}
	}
	if r.Spec.Priority < 0 || r.Spec.Priority > 100 {
		return fmt.Errorf("priority must be between 0 and 100")
	}
	if r.Spec.Selector == nil {
		return nil
	}
	for i, mac := range r.Spec.Selector.MACs {
		parsed, err := net.ParseMAC(mac)
		if err != nil {
			return fmt.Errorf("invalid MAC %q: %w", mac, err)
		}
		r.Spec.Selector.MACs[i] = parsed.String()
	}
	for _, xname := range r.Spec.Selector.XNames {
		if !bootvalidation.ValidateXName(xname) {
			return fmt.Errorf("invalid xname %q", xname)
		}
	}
	for _, group := range r.Spec.Selector.Groups {
		if group == "" {
			return fmt.Errorf("invalid group %q", group)
		}
	}
	return nil
}
