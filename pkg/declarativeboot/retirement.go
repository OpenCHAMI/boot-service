// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"

	"github.com/openchami/fabrica/pkg/revision"
)

// RetirementDisabledError identifies the unconditional revision-retirement denial.
type RetirementDisabledError struct{}

// Error is the stable code for a future HTTP 409 response.
func (RetirementDisabledError) Error() string { return "boot_config_retirement_disabled" }

// ErrRetirementDisabled is returned for every BootConfig revision retirement.
var ErrRetirementDisabled = RetirementDisabledError{}

// DenyBootConfigRetirement must be called by every retirement alias after actor
// authorization. It never reads or mutates storage and needs no manager gate.
// Series deletion and immutable UID spec mutation remain unsupported (HTTP 405).
func DenyBootConfigRetirement() error { return ErrRetirementDisabled }

// RetireRevision denies retirement regardless of the reference or context.
// Callers must authorize the request before invoking this fail-closed boundary.
func (*RevisionService) RetireRevision(context.Context, revision.Reference) error {
	return DenyBootConfigRetirement()
}
