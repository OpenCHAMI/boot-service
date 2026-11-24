// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"fmt"

	bootvalidation "github.com/openchami/boot-service/pkg/validation"
	"github.com/openchami/fabrica/pkg/resource"
)

// BootConfig contains immutable boot content for a named revision series.
type BootConfig struct {
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   resource.Metadata `json:"metadata" yaml:"metadata"`
	Spec       BootConfigSpec    `json:"spec" yaml:"spec"`
	Status     BootConfigStatus  `json:"status,omitempty" yaml:"status,omitempty"`
}

// BootConfigSpec contains the boot artifacts and kernel parameters.
type BootConfigSpec struct {
	Kernel string `json:"kernel" yaml:"kernel" validate:"required"`
	Initrd string `json:"initrd,omitempty" yaml:"initrd,omitempty"`
	Params string `json:"params,omitempty" yaml:"params,omitempty"`
}

// BootConfigStatus contains non-deployment health information.
type BootConfigStatus struct {
	Phase string `json:"phase,omitempty" yaml:"phase,omitempty"`
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
}

// Validate checks a new boot content resource.
func (r *BootConfig) Validate(_ context.Context) error {
	if err := validateNewResourceName(r.Metadata.Name); err != nil {
		return fmt.Errorf("metadata.name: %w", err)
	}
	if !bootvalidation.ValidateURLOrPath(r.Spec.Kernel) {
		return fmt.Errorf("invalid kernel URL or path: %q", r.Spec.Kernel)
	}
	if !bootvalidation.ValidateURLOrPathOptional(r.Spec.Initrd) {
		return fmt.Errorf("invalid initrd URL or path: %q", r.Spec.Initrd)
	}
	return nil
}
