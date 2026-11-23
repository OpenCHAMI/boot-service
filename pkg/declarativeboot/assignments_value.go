// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"slices"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

func cloneAssignment(a v1.BootAssignment) v1.BootAssignment {
	a.Metadata.Labels = maps.Clone(a.Metadata.Labels)
	a.Metadata.Annotations = maps.Clone(a.Metadata.Annotations)
	if a.Spec.Selector != nil {
		s := *a.Spec.Selector
		s.MACs = slices.Clone(s.MACs)
		s.NIDs = slices.Clone(s.NIDs)
		s.XNames = slices.Clone(s.XNames)
		s.Groups = slices.Clone(s.Groups)
		a.Spec.Selector = &s
	}
	return a
}

func snapshot(a v1.BootAssignment) Snapshot {
	a = cloneAssignment(a)
	// The typed resource contains only JSON-encodable fields.
	raw, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(raw)
	return Snapshot{Assignment: a, ETag: `"` + hex.EncodeToString(digest[:]) + `"`}
}
