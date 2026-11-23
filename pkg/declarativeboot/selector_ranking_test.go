// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"math/rand"
	"slices"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestSelectAssignmentRanking(t *testing.T) {
	node := v1.NodeSpec{XName: "x0c0s0b0n1", NID: 42, BootMAC: "aa:bb:cc:dd:ee:ff", Groups: []string{"a", "b", "c", "d", "e"}}
	tests := []struct {
		name          string
		first, second Snapshot
		want          string
	}{
		{"five groups cannot beat mac", selectionFixture("groups", &v1.BootAssignmentSelector{Groups: []string{"a", "b", "c", "d", "e"}}, 100, ""), selectionFixture("mac", &v1.BootAssignmentSelector{MACs: []string{node.BootMAC}}, 0, ""), "mac"},
		{"two groups cannot beat mac", selectionFixture("groups", &v1.BootAssignmentSelector{Groups: []string{"a", "b"}}, 100, ""), selectionFixture("mac", &v1.BootAssignmentSelector{MACs: []string{node.BootMAC}}, 0, ""), "mac"},
		{"nid beats xname", selectionFixture("xname", &v1.BootAssignmentSelector{XNames: []string{node.XName}}, 100, ""), selectionFixture("nid", &v1.BootAssignmentSelector{NIDs: []int32{42}}, 0, ""), "nid"},
		{"xname beats group", selectionFixture("groups", &v1.BootAssignmentSelector{Groups: []string{"a", "b", "c"}}, 100, ""), selectionFixture("xname", &v1.BootAssignmentSelector{XNames: []string{node.XName}}, 0, ""), "xname"},
		{"group beats catchall", selectionFixture("default", nil, 100, ""), selectionFixture("group", &v1.BootAssignmentSelector{Groups: []string{"a"}}, 0, ""), "group"},
		{"family scores are not added", selectionFixture("mixed", &v1.BootAssignmentSelector{NIDs: []int32{42}, XNames: []string{node.XName}, Groups: []string{"a"}}, 100, ""), selectionFixture("mac", &v1.BootAssignmentSelector{MACs: []string{node.BootMAC}}, 0, ""), "mac"},
		{"highest matched family within mixed selector", selectionFixture("nid", &v1.BootAssignmentSelector{NIDs: []int32{42}}, 0, ""), selectionFixture("mixed", &v1.BootAssignmentSelector{NIDs: []int32{42}, MACs: []string{node.BootMAC}}, 0, ""), "mixed"},
		{"higher priority", selectionFixture("a", nil, 10, ""), selectionFixture("z", nil, 11, ""), "z"},
		{"case-sensitive name", selectionFixture("alpha", nil, 10, ""), selectionFixture("Alpha", nil, 10, ""), "Alpha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, order := range [][]Snapshot{{tt.first, tt.second}, {tt.second, tt.first}} {
				got, matched, err := SelectAssignment(order, node, "")
				if err != nil || !matched || got.Assignment.Metadata.Name != tt.want {
					t.Fatalf("got %q matched=%v err=%v, want %q", got.Assignment.Metadata.Name, matched, err, tt.want)
				}
			}
		})
	}
}

func TestSelectAssignmentPermutations(t *testing.T) {
	entries := []Snapshot{selectionFixture("z", nil, 1, ""), selectionFixture("b", nil, 3, ""), selectionFixture("a", nil, 3, ""), selectionFixture("c", nil, 2, "")}
	rng := rand.New(rand.NewSource(13))
	for i := 0; i < 100; i++ {
		order := slices.Clone(entries)
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		got, matched, err := SelectAssignment(order, v1.NodeSpec{}, "")
		if err != nil || !matched || got.Assignment.Metadata.Name != "a" {
			t.Fatalf("permutation %d: got %q matched=%v err=%v", i, got.Assignment.Metadata.Name, matched, err)
		}
	}
}

func TestSelectAssignmentBrokenReferenceWins(t *testing.T) {
	good := selectionFixture("good", nil, 100, "")
	good.Assignment.Spec.ConfigRef = revision.Reference{Name: "exists"}
	broken := selectionFixture("broken", &v1.BootAssignmentSelector{NIDs: []int32{42}}, 0, "")
	broken.Assignment.Spec.ConfigRef = revision.Reference{Name: "missing"}
	got, matched, err := SelectAssignment([]Snapshot{good, broken}, v1.NodeSpec{NID: 42}, "")
	if err != nil || !matched || got.Assignment.Spec.ConfigRef.Name != "missing" {
		t.Fatalf("got %#v matched=%v err=%v", got.Assignment.Spec.ConfigRef, matched, err)
	}
}

func TestSelectAssignmentResultIsDetached(t *testing.T) {
	input := []Snapshot{selectionFixture("selected", &v1.BootAssignmentSelector{MACs: []string{"AA-BB-CC-DD-EE-FF"}, Groups: []string{"a"}}, 1, "")}
	input[0].Assignment.Metadata.Labels = map[string]string{"key": "original"}
	got, matched, err := SelectAssignment(input, v1.NodeSpec{BootMAC: "aa:bb:cc:dd:ee:ff"}, "")
	if err != nil || !matched || got.ETag != "etag-selected" {
		t.Fatalf("matched=%v err=%v etag=%q", matched, err, got.ETag)
	}
	got.Assignment.Spec.Selector.MACs[0] = "changed"
	got.Assignment.Spec.Selector.Groups[0] = "changed"
	got.Assignment.Metadata.Labels["key"] = "changed"
	if input[0].Assignment.Spec.Selector.MACs[0] != "AA-BB-CC-DD-EE-FF" || input[0].Assignment.Spec.Selector.Groups[0] != "a" || input[0].Assignment.Metadata.Labels["key"] != "original" {
		t.Fatal("selection mutated or aliases input")
	}
}
