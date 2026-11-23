// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
)

// ListRevisions returns authoritative native records without authorization.
func (s *RevisionService) ListRevisions(ctx context.Context, name string) ([]revision.Record[*v1.BootConfig], error) {
	if err := validateReference(revision.Reference{Name: name}); err != nil {
		return nil, err
	}
	var result []revision.Record[*v1.BootConfig]
	err := s.manager.WithMutation(ctx, func(LockedView) error {
		var err error
		result, err = storage.ListBootConfigRevisions(ctx, name)
		return classifyRevisionListError(err)
	})
	return result, err
}

// GetRevision reads a specific UID from its authoritative series.
func (s *RevisionService) GetRevision(ctx context.Context, ref revision.Reference) (revision.Record[*v1.BootConfig], error) {
	var result revision.Record[*v1.BootConfig]
	if err := validateReference(ref); err != nil {
		return result, err
	}
	if ref.UID == "" {
		return result, ErrInvalid
	}
	err := s.manager.WithMutation(ctx, func(LockedView) error {
		var err error
		result, err = storage.GetBootConfigRevision(ctx, ref.Name, ref.UID)
		return classifyMissingRevision(ctx, ref.Name, err)
	})
	return result, err
}

// ResolveRevision reads the exact native reference under the shared gate.
func (s *RevisionService) ResolveRevision(ctx context.Context, ref revision.Reference) (revision.Record[*v1.BootConfig], error) {
	var result revision.Record[*v1.BootConfig]
	if err := validateReference(ref); err != nil {
		return result, err
	}
	err := s.manager.WithMutation(ctx, func(LockedView) error {
		var err error
		result, err = storage.ResolveBootConfigRevision(ctx, ref)
		return classifyMissingRevision(ctx, ref.Name, err)
	})
	return result, err
}
