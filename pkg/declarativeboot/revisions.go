// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
)

// RevisionService shares the single Manager gate with assignment mutations.
// It performs no background writes. Generated storage must be initialized first.
type RevisionService struct{ manager *Manager }

// NewRevisionService binds revision operations to the existing single-writer gate.
func NewRevisionService(manager *Manager) *RevisionService { return &RevisionService{manager: manager} }

// ProposalRequest is a verified actor's desired immutable BootConfig.
type ProposalRequest struct {
	Name, RevisionName string
	Resource           v1.BootConfig
	Capabilities       Capabilities
}

// ProposalResult reports the committed native revision and current series ETag.
type ProposalResult struct {
	Record  revision.Record[*v1.BootConfig]
	ETag    string
	Created bool
}

// PromotionRequest carries the native compare-and-swap precondition.
type PromotionRequest struct {
	Name, UID, IfMatch string
	Capabilities       Capabilities
}

// GetSeries returns native state and its authoritative strong ETag.
func (s *RevisionService) GetSeries(ctx context.Context, name string) (revision.SeriesStatus, string, error) {
	var status revision.SeriesStatus
	var etag string
	if err := validateReference(revision.Reference{Name: name}); err != nil {
		return status, etag, err
	}
	err := s.manager.WithMutation(ctx, func(LockedView) error {
		var err error
		status, etag, err = storage.GetBootConfigSeries(ctx, name)
		return err
	})
	return status, etag, err
}

// PromoteDefault requires only promote capability and the exact native ETag.
func (s *RevisionService) PromoteDefault(ctx context.Context, req PromotionRequest) (revision.DefaultPromotionResult, error) {
	var result revision.DefaultPromotionResult
	if !req.Capabilities.Promote {
		return result, ErrForbidden
	}
	if err := validateReference(revision.Reference{Name: req.Name, UID: req.UID}); err != nil {
		return result, err
	}
	if req.UID == "" {
		return result, ErrInvalid
	}
	err := s.manager.WithMutation(ctx, func(LockedView) error {
		var err error
		result, err = storage.PromoteBootConfigDefault(ctx, req.Name, req.UID, req.IfMatch)
		return classifyMissingRevision(ctx, req.Name, err)
	})
	return result, err
}
