// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/storage"
)

func TestReferenceIntegrityRestoredBrokenWinner(t *testing.T) {
	for _, ref := range []revision.Reference{
		{Name: "missing"},
		{Name: "config", RevisionName: "missing"},
	} {
		t.Run(ref.Name+"/"+ref.RevisionName, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			backend, err := storage.NewFileBackend(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := backend.Close(); err != nil {
					t.Error(err)
				}
			})
			refs := newReferenceFixture(t, dir)
			store := NewBackendStore(backend)
			manager, err := NewManager(ctx, store, refs)
			if err != nil {
				t.Fatal(err)
			}
			created, err := manager.Create(ctx, assignment("winner"))
			if err != nil {
				t.Fatal(err)
			}
			broken := created.Assignment
			broken.Spec.ConfigRef = ref
			raw, err := json.Marshal(broken)
			if err != nil {
				t.Fatal(err)
			}
			if err := backend.Save(ctx, "BootAssignment", broken.Metadata.UID, raw); err != nil {
				t.Fatal(err)
			}

			reopened, err := NewManager(ctx, store, refs)
			if err != nil {
				t.Fatalf("startup must allow repair: %v", err)
			}
			winner, err := reopened.GetByName(ctx, "winner")
			if err != nil {
				t.Fatal(err)
			}
			cause := refs.ResolveReference(ctx, winner.Assignment.Spec.ConfigRef)
			if cause == nil {
				t.Fatal("expected real resolution failure")
			}
			err = WrapSelectedReferenceFailure(winner.Assignment, cause)
			var failure *ReferenceFailure
			if !errors.As(err, &failure) || !errors.Is(err, cause) {
				t.Fatalf("missing typed cause: %v", err)
			}
			if failure.AssignmentName != "winner" || failure.Reference != ref {
				t.Fatalf("wrong context: %#v", failure)
			}
		})
	}
}

func TestReferenceIntegrityMissingCommittedRecord(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	backend, err := storage.NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	refs := newReferenceFixture(t, dir)
	manager, err := NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatal(err)
	}
	record, err := refs.store.Resolve(ctx, revision.Reference{Name: "config"})
	if err != nil {
		t.Fatal(err)
	}
	a := assignment("winner")
	a.Spec.ConfigRef.UID = record.Metadata.UID
	if _, err := manager.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", record.Metadata.UID+".json")); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatalf("startup must allow repair: %v", err)
	}
	winner, err := reopened.GetByName(ctx, "winner")
	if err != nil {
		t.Fatal(err)
	}
	cause := refs.ResolveReference(ctx, winner.Assignment.Spec.ConfigRef)
	if cause == nil {
		t.Fatal("missing committed record unexpectedly resolved")
	}
	err = WrapSelectedReferenceFailure(winner.Assignment, cause)
	var failure *ReferenceFailure
	if !errors.As(err, &failure) || !errors.Is(err, cause) || failure.Reference.UID != record.Metadata.UID {
		t.Fatalf("missing record failure: %v", err)
	}
}

func TestReferenceIntegrityContextAndSecrets(t *testing.T) {
	a := assignment("winner")
	a.Spec.ConfigRef = revision.Reference{Name: "series", UID: "rev_identity", RevisionName: "revision", Selector: revision.SelectorLatest}
	secret := "secret-kernel-params"
	cause := errors.New(secret)
	err := WrapSelectedReferenceFailure(a, cause)
	var failure *ReferenceFailure
	if !errors.As(err, &failure) || failure.Reference != a.Spec.ConfigRef || failure.AssignmentName != "winner" || !errors.Is(err, cause) {
		t.Fatalf("reference failure context: %#v", failure)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("cause leaked into diagnostic string")
	}
	if errors.Is(err, ErrInvalid) {
		t.Fatal("operational error changed into invalid user reference")
	}
	if WrapSelectedReferenceFailure(a, nil) != nil {
		t.Fatal("nil must remain nil")
	}
	for _, cause := range []error{context.Canceled, fmt.Errorf("deadline: %w", context.DeadlineExceeded)} {
		got := WrapSelectedReferenceFailure(a, cause)
		if got != cause {
			t.Fatalf("cancellation changed: %v", got)
		}
		if errors.As(got, &failure) {
			t.Fatal("cancellation classified as reference failure")
		}
	}
}

func TestReferenceIntegrityDiagnosticFormattingOmitsCause(t *testing.T) {
	a := assignment("winner")
	a.Spec.ConfigRef = revision.Reference{Name: "config", RevisionName: "A"}
	cause := errors.New("credential=hidden kernel-param=hidden")
	err := WrapSelectedReferenceFailure(a, cause)
	encoded, marshalErr := json.Marshal(err)
	if marshalErr != nil || strings.Contains(string(encoded), "hidden") {
		t.Fatalf("encoded failure disclosed cause: %s, %v", encoded, marshalErr)
	}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		diagnostic := fmt.Sprintf(format, err)
		if strings.Contains(diagnostic, "hidden") {
			t.Fatalf("diagnostic %s disclosed cause: %s", format, diagnostic)
		}
	}
	var failure *ReferenceFailure
	wrapped := fmt.Errorf("delivery: %w", err)
	if !errors.As(wrapped, &failure) || !errors.Is(wrapped, cause) || errors.Is(wrapped, ErrInvalid) {
		t.Fatalf("lost selected failure or cause: %v", wrapped)
	}
}
