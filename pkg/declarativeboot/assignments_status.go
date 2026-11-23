// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

// StatusTarget identifies an assignment and its conditional request headers.
type StatusTarget struct {
	UID        string
	Conditions Conditions
}

// PutStatusByUID replaces only status under the exact current strong ETag.
func (m *Manager) PutStatusByUID(ctx context.Context, target StatusTarget, status v1.BootAssignmentStatus) (MutationResult, error) {
	return m.TransformStatusByUID(ctx, target, func(v1.BootAssignmentStatus) (v1.BootAssignmentStatus, error) { return status, nil })
}

// TransformStatusByUID calls transform on the current status under the shared
// gate. The callback must not reenter Manager or retain mutable status data.
func (m *Manager) TransformStatusByUID(ctx context.Context, target StatusTarget, transform func(v1.BootAssignmentStatus) (v1.BootAssignmentStatus, error)) (MutationResult, error) {
	var result MutationResult
	err := m.WithMutation(ctx, func(_ LockedView) error {
		if err := target.Conditions.Validate(); err != nil {
			return err
		}
		old, exists := m.byUID[target.UID]
		if !exists {
			return ErrPrecondition
		}
		if err := target.Conditions.check(snapshot(old).ETag, true); err != nil {
			return err
		}
		status, err := transform(old.Status)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if status == old.Status {
			result = MutationResult{Snapshot: snapshot(old)}
			return nil
		}
		next := cloneAssignment(old)
		next.Status = status
		result, err = m.commitAssignmentLocked(ctx, old, next)
		return err
	})
	return result, err
}
