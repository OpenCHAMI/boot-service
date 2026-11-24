// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

func TestAssignmentStatusPreventsStaleSpecOverwrite(t *testing.T) {
	// Given a persisted assignment and the original spec ETag.
	ctx := context.Background()
	m, backend := testManager(t)
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	target := StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}
	// When a status update is committed and a spec update uses the old tag.
	updated, err := m.PutStatusByUID(ctx, target, v1.BootAssignmentStatus{Phase: "ready"})
	if err != nil {
		t.Fatal(err)
	}
	changed := assignment("target")
	changed.Spec.Priority = 42
	_, staleErr := m.PutByUID(ctx, target.UID, changed, target.Conditions)
	// Then the old tag fails and the new tag updates spec without losing status.
	if !errors.Is(staleErr, ErrPrecondition) {
		t.Fatalf("stale spec update: %v", staleErr)
	}
	if updated.ETag == created.ETag {
		t.Fatal("status update did not change ETag")
	}
	result, err := m.PutByUID(ctx, target.UID, changed, Conditions{IfMatch: []string{updated.ETag}})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := NewBackendStore(backend).Load(ctx, target.UID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Assignment.Status.Phase != "ready" || stored.Status.Phase != "ready" || stored.Spec.Priority != 42 {
		t.Fatalf("status or spec lost: %#v", stored)
	}
}
