// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestBootAssignmentValidation(t *testing.T) {
	base := BootAssignment{Metadata: resource.Metadata{Name: "Assignment.A"}, Spec: BootAssignmentSpec{ConfigRef: revision.Reference{Name: "Config.A"}}}
	tests := []struct {
		name   string
		change func(*BootAssignment)
		valid  bool
	}{
		{"bare reference", func(*BootAssignment) {}, true},
		{"explicit default", func(a *BootAssignment) { a.Spec.ConfigRef.Selector = revision.SelectorDefault }, true},
		{"latest", func(a *BootAssignment) { a.Spec.ConfigRef.Selector = revision.SelectorLatest }, true},
		{"revision name", func(a *BootAssignment) { a.Spec.ConfigRef.RevisionName = "Revision.A" }, true},
		{"revision uid", func(a *BootAssignment) { a.Spec.ConfigRef.UID = revision.UIDExample }, true},
		{"no name", func(a *BootAssignment) { a.Metadata.Name = "" }, false},
		{"bad name", func(a *BootAssignment) { a.Metadata.Name = "-bad" }, false},
		{"missing ref", func(a *BootAssignment) { a.Spec.ConfigRef.Name = "" }, false},
		{"invalid ref name", func(a *BootAssignment) { a.Spec.ConfigRef.Name = "bad/name" }, false},
		{"invalid revision name", func(a *BootAssignment) { a.Spec.ConfigRef.RevisionName = "bad/name" }, false},
		{"invalid uid", func(a *BootAssignment) { a.Spec.ConfigRef.UID = "bogus" }, false},
		{"wrong uid prefix", func(a *BootAssignment) { a.Spec.ConfigRef.UID = "node_01h2xcejqtf2nbrexx3vqjhp41" }, false},
		{"conflicting selectors", func(a *BootAssignment) {
			a.Spec.ConfigRef.UID = revision.UIDExample
			a.Spec.ConfigRef.Selector = revision.SelectorLatest
		}, false},
		{"unknown selector", func(a *BootAssignment) { a.Spec.ConfigRef.Selector = "other" }, false},
		{"priority zero", func(a *BootAssignment) { a.Spec.Priority = 0 }, true},
		{"priority hundred", func(a *BootAssignment) { a.Spec.Priority = 100 }, true},
		{"priority negative", func(a *BootAssignment) { a.Spec.Priority = -1 }, false},
		{"priority high", func(a *BootAssignment) { a.Spec.Priority = 101 }, false},
		{"selector forms", func(a *BootAssignment) {
			a.Spec.Selector = &BootAssignmentSelector{MACs: []string{"AA-BB-CC-DD-EE-FF", "aabb.ccdd.eeff"}, NIDs: []int32{0, 12}, XNames: []string{"x0c0s0b0n0"}, Groups: []string{"compute"}}
		}, true},
		{"bad mac", func(a *BootAssignment) { a.Spec.Selector = &BootAssignmentSelector{MACs: []string{"bad"}} }, false},
		{"empty mac", func(a *BootAssignment) { a.Spec.Selector = &BootAssignmentSelector{MACs: []string{""}} }, false},
		{"wildcard xname", func(a *BootAssignment) { a.Spec.Selector = &BootAssignmentSelector{XNames: []string{"x*"}} }, false},
		{"bad group", func(a *BootAssignment) { a.Spec.Selector = &BootAssignmentSelector{Groups: []string{""}} }, false},
		{"literal wildcard group", func(a *BootAssignment) { a.Spec.Selector = &BootAssignmentSelector{Groups: []string{"compute*"}} }, true},
		{"uppercase group", func(a *BootAssignment) { a.Spec.Selector = &BootAssignmentSelector{Groups: []string{"Compute"}} }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assignment := base
			tc.change(&assignment)
			err := assignment.Validate(context.Background())
			if (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, want valid %t", err, tc.valid)
			}
		})
	}
}

func TestBootAssignmentRejectsMalformedScalar(t *testing.T) {
	for _, input := range []string{`{"spec":{"priority":"2"}}`, `{"spec":{"selector":{"nids":[1.5]}}}`, `{"spec":{"configRef":{"name":1}}}`} {
		var assignment BootAssignment
		if err := json.Unmarshal([]byte(input), &assignment); err == nil {
			t.Errorf("accepted malformed assignment %s", input)
		}
	}
}

func TestBootAssignmentNormalizesMACs(t *testing.T) {
	assignment := BootAssignment{
		Metadata: resource.Metadata{Name: "assignment"},
		Spec: BootAssignmentSpec{
			Selector:  &BootAssignmentSelector{MACs: []string{"AA-BB-CC-DD-EE-FF", "aabb.ccdd.eeff"}},
			ConfigRef: revision.Reference{Name: "config"},
		},
	}
	if err := assignment.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, mac := range assignment.Spec.Selector.MACs {
		if mac != "aa:bb:cc:dd:ee:ff" {
			t.Fatalf("MAC was not normalized: %q", mac)
		}
	}
}
