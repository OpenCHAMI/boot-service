// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	fabricastorage "github.com/openchami/fabrica/pkg/storage"
)

// AssignmentStore must enumerate every UID, including files with invalid JSON.
type AssignmentStore interface {
	ListUIDs(context.Context) ([]string, error)
	Load(context.Context, string) (*v1.BootAssignment, error)
	Save(context.Context, *v1.BootAssignment) error
	Update(context.Context, *v1.BootAssignment) error
	Delete(context.Context, string) error
}

// GeneratedStore binds the generated storage functions to the repository.
// Initialize the generated backend before creating a manager.
func GeneratedStore() AssignmentStore {
	return generatedStore{}
}

type generatedStore struct{}

func (generatedStore) ListUIDs(ctx context.Context) ([]string, error) {
	return storage.ListBootAssignmentUIDs(ctx)
}
func (generatedStore) Load(ctx context.Context, id string) (*v1.BootAssignment, error) {
	return storage.LoadBootAssignment(ctx, id)
}
func (generatedStore) Save(ctx context.Context, a *v1.BootAssignment) error {
	return storage.SaveBootAssignment(ctx, a)
}
func (generatedStore) Update(ctx context.Context, a *v1.BootAssignment) error {
	return storage.UpdateBootAssignment(ctx, a)
}
func (generatedStore) Delete(ctx context.Context, id string) error {
	return storage.DeleteBootAssignment(ctx, id)
}

type backendStore struct{ backend fabricastorage.StorageBackend }

// NewBackendStore provides an isolated adapter using the generated UID-file format.
func NewBackendStore(backend fabricastorage.StorageBackend) AssignmentStore {
	return backendStore{backend: backend}
}

func (s backendStore) ListUIDs(ctx context.Context) ([]string, error) {
	return s.backend.List(ctx, "BootAssignment")
}
func (s backendStore) Load(ctx context.Context, id string) (*v1.BootAssignment, error) {
	raw, err := s.backend.Load(ctx, "BootAssignment", id)
	if err != nil {
		return nil, err
	}
	var a v1.BootAssignment
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("decode assignment %q: %w", id, err)
	}
	return &a, nil
}
func (s backendStore) Save(ctx context.Context, a *v1.BootAssignment) error {
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return s.backend.Save(ctx, "BootAssignment", a.Metadata.UID, raw)
}
func (s backendStore) Update(ctx context.Context, a *v1.BootAssignment) error {
	exists, err := s.backend.Exists(ctx, "BootAssignment", a.Metadata.UID)
	if err != nil {
		return err
	}
	if !exists {
		return fabricastorage.ErrNotFound
	}
	return s.Save(ctx, a)
}
func (s backendStore) Delete(ctx context.Context, id string) error {
	return s.backend.Delete(ctx, "BootAssignment", id)
}
