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
	"github.com/openchami/fabrica/pkg/uid"
)

// ErrRevisionCorrupt is operational corruption, not an invalid user reference.
var ErrRevisionCorrupt = errors.New("boot revision store is inconsistent")

// NativeReferenceResolver uses the generated native revision store. The caller
// must already hold the shared Manager gate when resolving during a mutation.
type NativeReferenceResolver struct{}

func (NativeReferenceResolver) ResolveReference(ctx context.Context, ref revision.Reference) error {
	if err := validateReference(ref); err != nil {
		return err
	}
	_, err := storage.ListBootConfigRevisions(ctx, ref.Name)
	err = classifyRevisionListError(err)
	if err != nil {
		if errors.Is(err, ErrRevisionCorrupt) {
			return err
		}
		return referenceError(err)
	}
	_, err = storage.ResolveBootConfigRevision(ctx, ref)
	if err != nil {
		return referenceError(classifyRevisionIdentityError(err))
	}
	return nil
}

func classifyRevisionListError(err error) error {
	if errors.Is(err, revision.ErrRevisionNotFound) {
		return fmt.Errorf("%w: committed record missing", ErrRevisionCorrupt)
	}
	return classifyRevisionIdentityError(err)
}

func classifyRevisionIdentityError(err error) error {
	if errors.Is(err, revision.ErrRevisionIdentityMismatch) {
		return fmt.Errorf("%w: %w", ErrRevisionCorrupt, err)
	}
	return err
}

// classifyMissingRevision is called under the Manager gate after an ambiguous
// native not-found result; it never reenters the Manager.
func classifyMissingRevision(ctx context.Context, name string, err error) error {
	if !errors.Is(err, revision.ErrRevisionNotFound) {
		return classifyRevisionIdentityError(err)
	}
	_, listErr := storage.ListBootConfigRevisions(ctx, name)
	if listErr = classifyRevisionListError(listErr); listErr != nil {
		return listErr
	}
	return err
}

func validateReference(ref revision.Reference) error {
	if err := ref.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := v1.ValidateRevisionName(ref.Name); err != nil {
		return fmt.Errorf("%w: series name: %w", ErrInvalid, err)
	}
	if ref.RevisionName != "" {
		if err := v1.ValidateRevisionName(ref.RevisionName); err != nil {
			return fmt.Errorf("%w: revision name: %w", ErrInvalid, err)
		}
	}
	if ref.UID != "" {
		parsed, err := (uid.Codec{}).Parse(ref.UID)
		if err != nil || parsed.Format != uid.FormatTypeID || parsed.Prefix != "rev" {
			return fmt.Errorf("%w: invalid revision UID", ErrInvalid)
		}
	}
	return nil
}
