// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

// Package bootadopt plans explicit, non-destructive legacy boot adoption.
package bootadopt

import (
	"context"
	"crypto/sha256"
	"fmt"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

// Plan contains the immutable content and explicit targeting to create.
type Plan struct {
	SourceUID    string            `json:"sourceUid"`
	RevisionName string            `json:"revisionName"`
	Config       v1.BootConfig     `json:"config"`
	Assignment   v1.BootAssignment `json:"assignment"`
}

// BuildPlan rejects targeting whose accumulated legacy score cannot be
// represented losslessly. It never edits the source or accesses storage.
func BuildPlan(source v1.BootConfiguration, series, assignment string) (Plan, error) {
	fail := func(err error) (Plan, error) {
		return Plan{}, fmt.Errorf("source %q cannot be adopted: %w", source.Metadata.UID, err)
	}
	s := source.Spec
	if source.Metadata.UID == "" || len(s.Hosts)+len(s.MACs)+len(s.NIDs)+len(s.Groups) > 1 {
		return fail(fmt.Errorf("only an untargeted default or one exact selector can preserve legacy ranking"))
	}
	name := "legacy-" + source.Metadata.UID
	if v1.ValidateRevisionName(name) != nil {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(source.Metadata.UID)))
		name = "legacy-" + digest[:32] + "." + digest[32:]
	}
	plan := Plan{SourceUID: source.Metadata.UID, RevisionName: name,
		Config:     v1.BootConfig{Metadata: resource.Metadata{Name: series}, Spec: v1.BootConfigSpec{Kernel: s.Kernel, Initrd: s.Initrd, Params: s.Params}},
		Assignment: v1.BootAssignment{Metadata: resource.Metadata{Name: assignment}, Spec: v1.BootAssignmentSpec{Profile: s.Profile, Priority: s.Priority, ConfigRef: revision.Reference{Name: series, RevisionName: name}}},
	}
	if len(s.Hosts)+len(s.MACs)+len(s.NIDs)+len(s.Groups) > 0 {
		plan.Assignment.Spec.Selector = &v1.BootAssignmentSelector{XNames: append([]string(nil), s.Hosts...), MACs: append([]string(nil), s.MACs...), NIDs: append([]int32(nil), s.NIDs...), Groups: append([]string(nil), s.Groups...)}
	}
	if err := plan.Config.Validate(context.Background()); err != nil {
		return fail(err)
	}
	if err := plan.Assignment.Validate(context.Background()); err != nil {
		return fail(err)
	}
	return plan, nil
}
