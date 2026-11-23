// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
)

func selectionFixture(name string, selector *v1.BootAssignmentSelector, priority int, profile string) Snapshot {
	return Snapshot{Assignment: v1.BootAssignment{
		Metadata: resource.Metadata{Name: name},
		Spec:     v1.BootAssignmentSpec{Selector: selector, Priority: priority, Profile: profile},
	}, ETag: "etag-" + name}
}

func TestSelectAssignmentFamilies(t *testing.T) {
	node := v1.NodeSpec{XName: "x0c0s0b0n1", Hostname: "x0c0s0b0n2", NID: 42, BootMAC: "AA-BB-CC-DD-EE-FF", Groups: []string{"compute", "rack"}}
	tests := []struct {
		name     string
		selector *v1.BootAssignmentSelector
		matched  bool
	}{
		{"nil catch-all", nil, true},
		{"empty catch-all", &v1.BootAssignmentSelector{}, true},
		{"mac match alternative", &v1.BootAssignmentSelector{MACs: []string{"00:11:22:33:44:55", "aabb.ccdd.eeff"}}, true},
		{"mac mismatch", &v1.BootAssignmentSelector{MACs: []string{"00:11:22:33:44:55"}}, false},
		{"nid match alternative", &v1.BootAssignmentSelector{NIDs: []int32{43, 42}}, true},
		{"nid mismatch", &v1.BootAssignmentSelector{NIDs: []int32{43}}, false},
		{"xname match alternative", &v1.BootAssignmentSelector{XNames: []string{"x0c0s0b0n0", "x0c0s0b0n1"}}, true},
		{"xname exact not hostname", &v1.BootAssignmentSelector{XNames: []string{"x0c0s0b0n2"}}, false},
		{"group match alternative", &v1.BootAssignmentSelector{Groups: []string{"other", "rack"}}, true},
		{"group exact mismatch", &v1.BootAssignmentSelector{Groups: []string{"compute2"}}, false},
		{"mixed OR", &v1.BootAssignmentSelector{MACs: []string{"00:11:22:33:44:55"}, NIDs: []int32{43}, Groups: []string{"compute"}}, true},
		{"mixed no match", &v1.BootAssignmentSelector{NIDs: []int32{43}, Groups: []string{"other"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, matched, err := SelectAssignment([]Snapshot{selectionFixture("candidate", tt.selector, 0, "")}, node, "")
			if err != nil || matched != tt.matched {
				t.Fatalf("matched=%v, err=%v; want matched=%v", matched, err, tt.matched)
			}
			if matched && got.Assignment.Metadata.Name != "candidate" {
				t.Fatalf("selected %q, want candidate", got.Assignment.Metadata.Name)
			}
		})
	}
}

func TestSelectAssignmentProfiles(t *testing.T) {
	tests := []struct {
		name, requested, assignment string
		matched                     bool
	}{
		{"empty to empty", "", "", true},
		{"empty to default", "", "default", true},
		{"default to empty", "default", "", true},
		{"default to default", "default", "default", true},
		{"empty excludes explicit", "", "compute", false},
		{"explicit exact", "compute", "compute", true},
		{"explicit case sensitive", "Compute", "compute", false},
		{"explicit does not fall back", "compute", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, matched, err := SelectAssignment([]Snapshot{selectionFixture("candidate", nil, 0, tt.assignment)}, v1.NodeSpec{}, tt.requested)
			if err != nil || matched != tt.matched {
				t.Fatalf("matched=%v, err=%v; want %v", matched, err, tt.matched)
			}
		})
	}
}

func TestSelectAssignmentMACFormats(t *testing.T) {
	tests := []struct{ name, candidate, node string }{
		{"colon and dash", "AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff"},
		{"dot and colon", "aabb.ccdd.eeff", "AA:BB:CC:DD:EE:FF"},
		{"dash and dot", "AA-BB-CC-DD-EE-FF", "aabb.ccdd.eeff"},
		{"eui64", "00:11:22:33:44:55:66:77", "00-11-22-33-44-55-66-77"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []Snapshot{selectionFixture("mac", &v1.BootAssignmentSelector{MACs: []string{tt.candidate}}, 0, "")}
			got, matched, err := SelectAssignment(input, v1.NodeSpec{BootMAC: tt.node}, "")
			if err != nil || !matched || got.Assignment.Spec.Selector.MACs[0] != tt.candidate || input[0].Assignment.Spec.Selector.MACs[0] != tt.candidate {
				t.Fatalf("matched=%v err=%v input=%q output=%q", matched, err, input[0].Assignment.Spec.Selector.MACs[0], got.Assignment.Spec.Selector)
			}
		})
	}
}
