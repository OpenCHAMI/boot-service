// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import "github.com/openchami/fabrica/pkg/revision"

func dependentSeries(assignments []Snapshot, name string) bool {
	for _, item := range assignments {
		if item.Assignment.Spec.ConfigRef.Name == name {
			return true
		}
	}
	return false
}

func dependentRevision(assignments []Snapshot, name, revisionName string) bool {
	for _, item := range assignments {
		ref := item.Assignment.Spec.ConfigRef
		if ref.Name == name && (ref.Selector == revision.SelectorLatest || revisionName != "" && ref.RevisionName == revisionName) {
			return true
		}
	}
	return false
}
