// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestAssignmentConditions(t *testing.T) {
	// Given an existing assignment and its committed ETag.
	m, _ := testManager(t)
	ctx := context.Background()
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		c    Conditions
		want error
	}{
		{"missing", Conditions{}, ErrPrecondition}, {"wildcard", Conditions{IfMatch: []string{"*"}}, ErrPrecondition},
		{"weak", Conditions{IfMatch: []string{"W/" + created.ETag}}, ErrPrecondition}, {"stale", Conditions{IfMatch: []string{`"old"`}}, ErrPrecondition},
		{"both", Conditions{IfMatch: []string{created.ETag}, IfNoneMatch: []string{"*"}}, ErrBadCondition},
		{"multiple", Conditions{IfMatch: []string{created.ETag, created.ETag}}, ErrBadCondition},
		{"list", Conditions{IfMatch: []string{created.ETag + "," + created.ETag}}, ErrBadCondition},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// When an update is attempted with the condition.
			_, err := m.PutByName(ctx, "target", assignment("target"), tc.c)
			// Then it fails without changing the committed ETag.
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			got, err := m.GetByName(ctx, "target")
			if err != nil || got.ETag != created.ETag {
				t.Fatalf("got %#v: %v", got, err)
			}
		})
	}
}

func TestAssignmentReplayAndIsolation(t *testing.T) {
	// Given normalized selectors and opaque metadata on a persisted assignment.
	m, _ := testManager(t)
	ctx := context.Background()
	a := assignment("target")
	a.Metadata.Labels = map[string]string{"a": "b"}
	a.Metadata.Annotations = map[string]string{"raw": "x"}
	a.Status.Phase = "ready"
	a.Spec.Profile = "default"
	a.Spec.Selector = &v1.BootAssignmentSelector{MACs: []string{"AA-BB-CC-DD-EE-FF"}, Groups: []string{"second", "first"}}
	created, err := m.Create(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	created.Assignment.Metadata.Labels["a"] = "changed"
	a.Spec.Profile = ""
	a.Spec.Selector.MACs = []string{"aa:bb:cc:dd:ee:ff"}
	a.Spec.Selector.Groups = []string{"first", "second"}
	// When an equivalent desired spec is applied.
	replayed, err := m.PutByName(ctx, "target", a, Conditions{IfMatch: []string{created.ETag}})
	// Then the UID, ETag, timestamps and opaque values remain unchanged.
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ETag != created.ETag || replayed.Assignment.Metadata.UID != created.Assignment.Metadata.UID || replayed.Assignment.Metadata.Labels["a"] != "b" || replayed.Assignment.Status.Phase != "ready" {
		t.Fatalf("replay = %#v", replayed)
	}
}

func TestAssignmentReferenceValidation(t *testing.T) {
	// Given a real native revision store containing one series.
	m, _ := testManager(t)
	ctx := context.Background()
	tests := []struct {
		name string
		ref  revision.Reference
	}{
		{"missing series", revision.Reference{Name: "missing"}},
		{"missing revision", revision.Reference{Name: "config", RevisionName: "absent"}},
		{"wrong series", revision.Reference{Name: "missing", UID: revision.UIDExample}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// When an assignment references a non-resolving revision.
			a := assignment(tc.name)
			a.Metadata.Name = "invalid"
			a.Spec.ConfigRef = tc.ref
			_, err := m.Create(ctx, a)
			// Then the write is rejected and no assignment exists.
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v", err)
			}
			if _, err := m.GetByName(ctx, "invalid"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestAssignmentTransformUsesCurrentSpec(t *testing.T) {
	// Given a committed assignment.
	m, _ := testManager(t)
	ctx := context.Background()
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	// When a spec-only transform runs under the matching ETag.
	updated, err := m.TransformByUID(ctx, created.Assignment.Metadata.UID, Conditions{IfMatch: []string{created.ETag}}, func(s v1.BootAssignmentSpec) (v1.BootAssignmentSpec, error) { s.Priority = 11; return s, nil })
	// Then the changed spec is committed with the same identity.
	if err != nil || updated.Assignment.Spec.Priority != 11 || updated.Assignment.Metadata.UID != created.Assignment.Metadata.UID || updated.ETag == created.ETag {
		t.Fatalf("got %#v: %v", updated, err)
	}
}

func TestAssignmentNamedTransformUsesCurrentSpec(t *testing.T) {
	// Given a committed assignment and its ETag.
	m, _ := testManager(t)
	ctx := context.Background()
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	// When a named spec transform is applied.
	result, err := m.TransformByName(ctx, "target", Conditions{IfMatch: []string{created.ETag}}, func(s v1.BootAssignmentSpec) (v1.BootAssignmentSpec, error) { s.Priority = 21; return s, nil })
	// Then it retains identity and commits the transformed spec.
	if err != nil || result.Assignment.Metadata.UID != created.Assignment.Metadata.UID || result.Assignment.Spec.Priority != 21 {
		t.Fatalf("got %#v: %v", result, err)
	}
}

func TestAssignmentReplayWithEmptySelectorAlternatives(t *testing.T) {
	// Given a stored selector with one populated family.
	m, _ := testManager(t)
	ctx := context.Background()
	a := assignment("target")
	a.Spec.Selector = &v1.BootAssignmentSelector{Groups: []string{"group"}}
	created, err := m.Create(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	// When an equivalent desired selector supplies empty alternative lists.
	a.Spec.Selector.MACs = []string{}
	a.Spec.Selector.NIDs = []int32{}
	a.Spec.Selector.XNames = []string{}
	result, err := m.PutByName(ctx, "target", a, Conditions{IfMatch: []string{created.ETag}})
	// Then the committed ETag remains unchanged.
	if err != nil || result.ETag != created.ETag {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestAssignmentDeletedETagCannotAuthorizeFurtherWrites(t *testing.T) {
	m, _ := testManager(t)
	ctx := context.Background()
	a := assignment("target")
	created, err := m.Create(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	condition := Conditions{IfMatch: []string{created.ETag}}
	if err := m.DeleteByUID(ctx, created.Assignment.Metadata.UID, condition); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func() error{
		"put": func() error { _, err := m.PutByUID(ctx, created.Assignment.Metadata.UID, a, condition); return err },
		"patch": func() error {
			_, err := m.TransformByUID(ctx, created.Assignment.Metadata.UID, condition, func(spec v1.BootAssignmentSpec) (v1.BootAssignmentSpec, error) {
				t.Fatal("transform executed after deletion")
				return spec, nil
			})
			return err
		},
		"delete": func() error { return m.DeleteByUID(ctx, created.Assignment.Metadata.UID, condition) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); !errors.Is(err, ErrPrecondition) {
				t.Fatalf("stale deleted ETag: %v", err)
			}
		})
	}
}
