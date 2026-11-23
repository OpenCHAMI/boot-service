// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/uid"
)

func prepareProposal(ctx context.Context, req ProposalRequest) (v1.BootConfig, error) {
	if !req.Capabilities.Propose {
		return v1.BootConfig{}, ErrForbidden
	}
	if err := validateReference(revision.Reference{Name: req.Name, RevisionName: req.RevisionName}); err != nil {
		return v1.BootConfig{}, err
	}
	if req.Resource.Metadata.Name != "" && req.Resource.Metadata.Name != req.Name {
		return v1.BootConfig{}, ErrConflict
	}
	config := req.Resource
	config.Metadata = *req.Resource.Metadata.Clone()
	config.Metadata.Name = req.Name
	config.Metadata.UID = ""
	config.Metadata.CreatedAt, config.Metadata.UpdatedAt = time.Time{}, time.Time{}
	config.APIVersion, config.Kind = "v1", "BootConfig"
	config.Status = v1.BootConfigStatus{}
	if err := config.Validate(ctx); err != nil {
		return v1.BootConfig{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return config, nil
}

func initializeProposal(config *v1.BootConfig) error {
	id, err := (uid.Codec{}).Generate("bootconfig")
	if err != nil {
		return fmt.Errorf("generate boot config UID: %w", err)
	}
	config.Metadata.UID = id
	config.Metadata.CreatedAt = time.Now().UTC()
	config.Metadata.UpdatedAt = config.Metadata.CreatedAt
	return nil
}

// CreateSeries synchronously creates A under the shared assignment gate.
func (s *RevisionService) CreateSeries(ctx context.Context, req ProposalRequest) (ProposalResult, error) {
	var result ProposalResult
	config, err := prepareProposal(ctx, req)
	if err != nil {
		return result, err
	}
	err = s.manager.WithMutation(ctx, func(view LockedView) error {
		_, _, err := storage.GetBootConfigSeries(ctx, req.Name)
		if err == nil {
			return revision.ErrSeriesExists
		}
		if !errors.Is(err, revision.ErrSeriesNotFound) {
			return err
		}
		if !req.Capabilities.Promote && dependentSeries(view.Assignments(), req.Name) {
			return ErrForbidden
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := initializeProposal(&config); err != nil {
			return err
		}
		result.Record, result.ETag, err = storage.CreateBootConfigSeries(ctx, req.Name, req.RevisionName, &config)
		result.Created = err == nil
		return err
	})
	return result, err
}

// EnsureRevision idempotently binds a nonempty name to immutable content.
func (s *RevisionService) EnsureRevision(ctx context.Context, req ProposalRequest) (ProposalResult, error) {
	if req.RevisionName == "" {
		return ProposalResult{}, ErrInvalid
	}
	return s.proposeRevision(ctx, req, true)
}

// CreateRevision creates an unnamed revision; it is never an idempotent replay.
func (s *RevisionService) CreateRevision(ctx context.Context, req ProposalRequest) (ProposalResult, error) {
	if req.RevisionName != "" {
		return ProposalResult{}, ErrInvalid
	}
	return s.proposeRevision(ctx, req, false)
}

func (s *RevisionService) proposeRevision(ctx context.Context, req ProposalRequest, named bool) (ProposalResult, error) {
	var result ProposalResult
	config, err := prepareProposal(ctx, req)
	if err != nil {
		return result, err
	}
	err = s.manager.WithMutation(ctx, func(view LockedView) error {
		_, _, err := storage.GetBootConfigSeries(ctx, req.Name)
		if err != nil {
			return err
		}
		if named {
			_, err = storage.ResolveBootConfigRevision(ctx, revision.Reference{Name: req.Name, RevisionName: req.RevisionName})
			if err == nil || errors.Is(err, revision.ErrRevisionRetired) {
				result.Record, result.Created, result.ETag, err = storage.EnsureBootConfigRevisionByName(ctx, req.Name, req.RevisionName, &config)
				return classifyRevisionIdentityError(err)
			}
			if !errors.Is(err, revision.ErrRevisionNotFound) {
				return classifyRevisionIdentityError(err)
			}
		}
		_, err = storage.ListBootConfigRevisions(ctx, req.Name)
		if err = classifyRevisionListError(err); err != nil {
			return err
		}
		if !req.Capabilities.Promote && dependentRevision(view.Assignments(), req.Name, req.RevisionName) {
			return ErrForbidden
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := initializeProposal(&config); err != nil {
			return err
		}
		if named {
			result.Record, result.Created, result.ETag, err = storage.EnsureBootConfigRevisionByName(ctx, req.Name, req.RevisionName, &config)
		} else {
			result.Record, result.Created, result.ETag, err = storage.CreateBootConfigRevision(ctx, req.Name, &config)
		}
		return classifyRevisionIdentityError(err)
	})
	return result, err
}
