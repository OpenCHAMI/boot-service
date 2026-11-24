// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
)

// SelectionSnapshot contains detached content and the identity that selected it.
type SelectionSnapshot struct {
	Assignment   Snapshot
	Content      v1.BootConfigSpec
	RevisionUID  string
	RevisionName string
	Digest       string
}

// SelectForDelivery exposes detached selection values without leaking the gate.
func (s *RevisionService) SelectForDelivery(ctx context.Context, node v1.NodeSpec, profile string) (v1.BootConfigSpec, v1.BootAssignment, revision.Metadata, bool, error) {
	snapshot, matched, err := s.SelectSnapshot(ctx, node, profile)
	metadata := revision.Metadata{UID: snapshot.RevisionUID, RevisionName: snapshot.RevisionName, Digest: snapshot.Digest}
	return snapshot.Content, snapshot.Assignment.Assignment, metadata, matched, err
}

// HasAssignments checks the current store without resolving node identity.
func (s *RevisionService) HasAssignments(ctx context.Context) (bool, error) {
	assignments, err := s.manager.List(ctx)
	return len(assignments) != 0, err
}

// SelectSnapshot selects and resolves a reference under the same mutation gate.
func (s *RevisionService) SelectSnapshot(ctx context.Context, node v1.NodeSpec, profile string) (SelectionSnapshot, bool, error) {
	var result SelectionSnapshot
	var matched bool
	err := s.manager.WithMutation(ctx, func(view LockedView) error {
		winner, found, err := SelectAssignment(view.Assignments(), node, profile)
		if err != nil || !found {
			return err
		}
		matched = true
		ref := winner.Assignment.Spec.ConfigRef
		if err := validateReference(ref); err != nil {
			return WrapSelectedReferenceFailure(winner.Assignment, err)
		}
		record, err := storage.ResolveBootConfigRevision(ctx, ref)
		if err != nil {
			failure := WrapSelectedReferenceFailure(winner.Assignment, classifyMissingRevision(ctx, ref.Name, err))
			var selected *ReferenceFailure
			if errors.As(failure, &selected) && ref.UID == "" && ref.RevisionName == "" {
				series, _, pointerErr := storage.GetBootConfigSeries(ctx, ref.Name)
				if pointerErr == nil {
					pointer := series.DefaultRevisionUID
					if ref.Selector == revision.SelectorLatest {
						pointer = series.LatestRevisionUID
					}
					if pointer != "" && validateReference(revision.Reference{Name: ref.Name, UID: pointer}) == nil {
						selected.ResolvedUID = pointer
					}
				}
			}
			return failure
		}
		if record.Resource == nil || record.Metadata.RetiredAt != nil || record.Metadata.UID == "" || record.Metadata.Digest == "" || record.Metadata.SeriesName != ref.Name {
			return WrapSelectedReferenceFailure(winner.Assignment, fmt.Errorf("invalid revision record: %w", ErrRevisionCorrupt))
		}
		if err := validateReference(revision.Reference{Name: ref.Name, UID: record.Metadata.UID}); err != nil || record.Metadata.Number == 0 || (ref.UID != "" && ref.UID != record.Metadata.UID) || (ref.RevisionName != "" && ref.RevisionName != record.Metadata.RevisionName) {
			return WrapSelectedReferenceFailure(winner.Assignment, fmt.Errorf("revision identity integrity: %w", ErrRevisionCorrupt))
		}
		if record.Metadata.RevisionName != "" {
			if err := v1.ValidateRevisionName(record.Metadata.RevisionName); err != nil {
				return WrapSelectedReferenceFailure(winner.Assignment, fmt.Errorf("revision name integrity: %w", ErrRevisionCorrupt))
			}
			bound, err := storage.ResolveBootConfigRevision(ctx, revision.Reference{Name: ref.Name, RevisionName: record.Metadata.RevisionName})
			if err != nil {
				err = classifyRevisionIdentityError(err)
				if errors.Is(err, revision.ErrRevisionNotFound) || errors.Is(err, revision.ErrRevisionRetired) || errors.Is(err, revision.ErrSeriesNotFound) {
					err = fmt.Errorf("revision name binding: %w: %w", ErrRevisionCorrupt, err)
				}
				return WrapSelectedReferenceFailure(winner.Assignment, err)
			}
			if bound.Metadata.UID != record.Metadata.UID || bound.Metadata.Number != record.Metadata.Number || bound.Metadata.Digest != record.Metadata.Digest || bound.Metadata.RevisionName != record.Metadata.RevisionName || !bound.Metadata.CreatedAt.Equal(record.Metadata.CreatedAt) {
				return WrapSelectedReferenceFailure(winner.Assignment, fmt.Errorf("revision name binding integrity: %w", ErrRevisionCorrupt))
			}
		}
		if ref.UID == "" && ref.RevisionName == "" {
			series, _, err := storage.GetBootConfigSeries(ctx, ref.Name)
			if err != nil {
				return WrapSelectedReferenceFailure(winner.Assignment, err)
			}
			pointer := series.DefaultRevisionUID
			if ref.Selector == revision.SelectorLatest {
				pointer = series.LatestRevisionUID
			}
			if pointer != record.Metadata.UID {
				return WrapSelectedReferenceFailure(winner.Assignment, fmt.Errorf("revision pointer integrity: %w", ErrRevisionCorrupt))
			}
		}
		digest, digestErr := storage.BootConfigRevisionDigest(record.Resource)
		if digestErr != nil || digest != record.Metadata.Digest || record.Resource.Metadata.Name != ref.Name || record.Resource.APIVersion != "v1" || record.Resource.Kind != "BootConfig" {
			return WrapSelectedReferenceFailure(winner.Assignment, fmt.Errorf("revision content integrity: %w", ErrRevisionCorrupt))
		}
		if err := record.Resource.Validate(ctx); err != nil {
			return WrapSelectedReferenceFailure(winner.Assignment, fmt.Errorf("invalid revision content: %w", ErrRevisionCorrupt))
		}
		result = SelectionSnapshot{Assignment: winner, Content: record.Resource.Spec, RevisionUID: record.Metadata.UID, RevisionName: record.Metadata.RevisionName, Digest: record.Metadata.Digest}
		return nil
	})
	return result, matched, err
}
