// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"slices"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

// SelectorIntegrityError identifies invalid selection data rather than a no-match.
type SelectorIntegrityError struct {
	Assignment string
	Cause      error
}

func (e *SelectorIntegrityError) Error() string {
	return fmt.Sprintf("assignment selection integrity %q: %v", e.Assignment, e.Cause)
}

func (e *SelectorIntegrityError) Unwrap() error { return e.Cause }

// SelectAssignment ranks detached snapshots without resolving their references.
// A false match with nil error delegates to legacy selection. Empty and "default"
// profiles are equivalent; other requested profiles match exactly. The returned
// snapshot is independent of the inputs, including selector slices and metadata.
func SelectAssignment(assignments []Snapshot, node v1.NodeSpec, profile string) (Snapshot, bool, error) {
	ordered := slices.Clone(assignments)
	slices.SortStableFunc(ordered, func(a, b Snapshot) int {
		return cmp.Compare(a.Assignment.Metadata.Name, b.Assignment.Metadata.Name)
	})
	var winner Snapshot
	bestScore := 0
	for i, candidate := range ordered {
		name := candidate.Assignment.Metadata.Name
		if i > 0 && name == ordered[i-1].Assignment.Metadata.Name {
			return Snapshot{}, false, &SelectorIntegrityError{Assignment: name, Cause: fmt.Errorf("duplicate assignment name")}
		}
		// Validate only the selector: reference resolution belongs to the caller.
		validated := cloneAssignment(candidate.Assignment)
		validated.Metadata.Name = "selector"
		validated.Spec.ConfigRef = revision.Reference{Name: "selector"}
		validated.Spec.Priority = 0
		if err := validated.Validate(context.Background()); err != nil {
			return Snapshot{}, false, &SelectorIntegrityError{Assignment: name, Cause: err}
		}
		if defaultProfile(validated.Spec.Profile) != defaultProfile(profile) {
			continue
		}
		score, err := selectorScore(validated.Spec.Selector, node)
		if err != nil {
			return Snapshot{}, false, &SelectorIntegrityError{Assignment: name, Cause: err}
		}
		if score > bestScore || (score > 0 && score == bestScore && candidate.Assignment.Spec.Priority > winner.Assignment.Spec.Priority) {
			winner = candidate
			bestScore = score
		}
	}
	if bestScore == 0 {
		return Snapshot{}, false, nil
	}
	winner.Assignment = cloneAssignment(winner.Assignment)
	return winner, true, nil
}

func defaultProfile(profile string) string {
	if profile == "" {
		return "default"
	}
	return profile
}

func selectorScore(selector *v1.BootAssignmentSelector, node v1.NodeSpec) (int, error) {
	if selector == nil || len(selector.MACs)+len(selector.NIDs)+len(selector.XNames)+len(selector.Groups) == 0 {
		return 1, nil
	}
	if len(selector.MACs) > 0 && node.BootMAC != "" {
		mac, err := net.ParseMAC(node.BootMAC)
		if err != nil {
			return 0, fmt.Errorf("invalid node boot MAC: %w", err)
		}
		if slices.Contains(selector.MACs, mac.String()) {
			return 100, nil
		}
	}
	if slices.Contains(selector.NIDs, node.NID) {
		return 75, nil
	}
	if slices.Contains(selector.XNames, node.XName) {
		return 50, nil
	}
	for _, group := range selector.Groups {
		if slices.Contains(node.Groups, group) {
			return 25, nil
		}
	}
	return 0, nil
}
