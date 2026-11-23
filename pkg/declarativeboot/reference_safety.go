// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

// ReferenceFailure identifies a selected assignment whose actual resolution
// failed. Reference contains only the series name and revision discriminator;
// the underlying error may contain sensitive data and must not be exposed.
type ReferenceFailure struct {
	AssignmentName string
	// ResolvedUID is an independently validated pointer identity when resolution fails.
	ResolvedUID string
	Reference   revision.Reference
	cause       error
}

// Error deliberately excludes the underlying error and resource contents.
func (*ReferenceFailure) Error() string { return "selected boot assignment reference unavailable" }

// String keeps formatting a copied value as safe as formatting the error pointer.
func (e ReferenceFailure) String() string { return e.Error() }

// GoString exposes only reference identity, never the underlying cause.
func (e ReferenceFailure) GoString() string {
	return fmt.Sprintf("declarativeboot.ReferenceFailure{AssignmentName:%q, Reference:%#v}", e.AssignmentName, e.Reference)
}

// Unwrap preserves the resolver error; callers must not expose its contents.
func (e *ReferenceFailure) Unwrap() error { return e.cause }

// WrapSelectedReferenceFailure wraps an actual resolution failure for the
// already-selected assignment. Nil means resolution succeeded, not no-match.
// Cancellation and deadlines propagate unchanged. Delivery must handle this
// type before inspecting its cause, return a non-booting 503, and never fall
// back; no assignment/no match is a separate selection result. The caller must
// not expose the unwrapped error. This does not perform mutation validation.
func WrapSelectedReferenceFailure(assignment v1.BootAssignment, err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &ReferenceFailure{AssignmentName: assignment.Metadata.Name, Reference: assignment.Spec.ConfigRef, cause: err}
}
