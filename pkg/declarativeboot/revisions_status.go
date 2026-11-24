// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
)

// TransformStatus changes health status under the same gate and strong series
// precondition as deployment mutations. The callback must not reenter Manager.
func (s *RevisionService) TransformStatus(ctx context.Context, name string, conditions Conditions, transform func(v1.BootConfigStatus) (v1.BootConfigStatus, error)) (v1.BootConfigStatus, string, error) {
	var status v1.BootConfigStatus
	var etag string
	if err := conditions.Validate(); err != nil {
		return status, etag, err
	}
	if err := validateReference(revision.Reference{Name: name}); err != nil {
		return status, etag, err
	}
	err := s.manager.WithMutation(ctx, func(LockedView) error {
		var err error
		status, etag, err = storage.MutateBootConfigSeriesStatus(ctx, name, func(current *v1.BootConfigStatus, currentETag string) error {
			if err := conditions.check(currentETag, true); err != nil {
				return err
			}
			updated, err := transform(*current)
			if err != nil {
				return err
			}
			*current = updated
			return nil
		})
		return err
	})
	return status, etag, err
}
