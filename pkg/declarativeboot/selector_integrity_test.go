// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"errors"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

func TestSelectAssignmentIntegrity(t *testing.T) {
	tests := []struct {
		name     string
		selector *v1.BootAssignmentSelector
	}{
		{"malformed mac", &v1.BootAssignmentSelector{MACs: []string{"bad"}}},
		{"malformed mac after matching value", &v1.BootAssignmentSelector{MACs: []string{"aa:bb:cc:dd:ee:ff", "bad"}}},
		{"wildcard xname", &v1.BootAssignmentSelector{XNames: []string{"x0c0s*"}}},
		{"empty group", &v1.BootAssignmentSelector{Groups: []string{""}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := selectionFixture("invalid", tt.selector, 0, "different")
			_, matched, err := SelectAssignment([]Snapshot{selectionFixture("valid", nil, 0, ""), candidate}, v1.NodeSpec{BootMAC: "aa:bb:cc:dd:ee:ff"}, "")
			var integrity *SelectorIntegrityError
			if matched || !errors.As(err, &integrity) {
				t.Fatalf("matched=%v err=%v, want typed integrity error", matched, err)
			}
		})
	}
}

func TestSelectAssignmentDuplicateNamesAreIntegrityError(t *testing.T) {
	input := []Snapshot{selectionFixture("same", nil, 0, ""), selectionFixture("same", nil, 0, "")}
	_, matched, err := SelectAssignment(input, v1.NodeSpec{}, "")
	var integrity *SelectorIntegrityError
	if matched || !errors.As(err, &integrity) {
		t.Fatalf("matched=%v err=%v, want typed integrity error", matched, err)
	}
}

func TestSelectAssignmentNoMatch(t *testing.T) {
	got, matched, err := SelectAssignment(nil, v1.NodeSpec{}, "")
	if err != nil || matched || got.Assignment.Metadata.Name != "" {
		t.Fatalf("got=%v matched=%v err=%v, want no match", got, matched, err)
	}
}

func TestSelectAssignmentInvalidNodeMAC(t *testing.T) {
	input := []Snapshot{selectionFixture("mac", &v1.BootAssignmentSelector{MACs: []string{"aa:bb:cc:dd:ee:ff"}}, 0, "")}
	_, matched, err := SelectAssignment(input, v1.NodeSpec{BootMAC: "invalid"}, "")
	var integrity *SelectorIntegrityError
	if matched || !errors.As(err, &integrity) {
		t.Fatalf("matched=%v err=%v, want typed integrity error", matched, err)
	}
}

func TestSelectAssignmentGroupValuesAreLiteral(t *testing.T) {
	for _, group := range []string{"rack*", "Compute"} {
		input := []Snapshot{selectionFixture("literal", &v1.BootAssignmentSelector{Groups: []string{group}}, 0, "")}
		for _, actual := range []string{group, "rack1", "compute"} {
			_, matched, err := SelectAssignment(input, v1.NodeSpec{Groups: []string{actual}}, "")
			if err != nil || matched != (actual == group) {
				t.Fatalf("selector %q group %q: matched=%v err=%v", group, actual, matched, err)
			}
		}
	}
}
