// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

func (m *Manager) commitAssignmentLocked(ctx context.Context, old, next v1.BootAssignment) (MutationResult, error) {
	next.Metadata.UpdatedAt = time.Now().UTC()
	if next.Metadata.UpdatedAt.Before(old.Metadata.UpdatedAt) {
		next.Metadata.UpdatedAt = old.Metadata.UpdatedAt
	}
	if err := ctx.Err(); err != nil {
		return MutationResult{}, err
	}
	if err := m.store.Update(ctx, &next); err != nil {
		return MutationResult{}, m.reconcileUpdateError(ctx, old, err)
	}
	m.byUID[next.Metadata.UID] = cloneAssignment(next)
	if err := ctx.Err(); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Snapshot: snapshot(next)}, nil
}

func (m *Manager) reconcileUpdateError(ctx context.Context, old v1.BootAssignment, updateErr error) error {
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	current, err := m.store.Load(readCtx, old.Metadata.UID)
	if err == nil {
		if current == nil || current.Metadata.UID != old.Metadata.UID || current.Metadata.Name != old.Metadata.Name || current.Kind != "BootAssignment" || current.APIVersion != "v1" || current.Metadata.CreatedAt.IsZero() || current.Metadata.UpdatedAt.IsZero() || current.Metadata.UpdatedAt.Before(current.Metadata.CreatedAt) {
			err = ErrInvalid
		} else {
			candidate := cloneAssignment(*current)
			err = candidate.Validate(readCtx)
		}
	}
	if err != nil {
		m.unavailable = fmt.Errorf("%w: reload assignment: %w", ErrStoreUncertain, err)
		return errors.Join(fmt.Errorf("update assignment: %w", updateErr), m.unavailable)
	}
	m.byUID[old.Metadata.UID] = cloneAssignment(*current)
	return fmt.Errorf("update assignment: %w", updateErr)
}
