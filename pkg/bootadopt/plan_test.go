// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootadopt

import (
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
)

func TestPlanPreservesSourceAndRejectsLossySelectors(t *testing.T) {
	source := v1.BootConfiguration{Metadata: resource.Metadata{UID: "opaque/UID"}, Spec: v1.BootConfigurationSpec{
		Hosts: []string{"x0c0s0b0n0"}, Kernel: "/kernel", Initrd: "/initrd", Params: "console=tty0", Profile: "debug", Priority: 9,
	}}
	plan, err := BuildPlan(source, "production", "canary")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Config.Spec.Kernel != source.Spec.Kernel || plan.Assignment.Spec.Profile != "debug" || plan.Assignment.Spec.Priority != 9 || plan.Assignment.Spec.Selector.XNames[0] != source.Spec.Hosts[0] {
		t.Fatal("lossy plan")
	}
	if v1.ValidateRevisionName(plan.RevisionName) != nil || !strings.HasPrefix(plan.RevisionName, "legacy-") {
		t.Fatal("invalid fallback revision name")
	}
	again, err := BuildPlan(source, "other", "other")
	if err != nil || again.RevisionName != plan.RevisionName {
		t.Fatal("revision name depends on target")
	}
	for _, spec := range []v1.BootConfigurationSpec{
		{Kernel: "/kernel", Hosts: []string{"x0c0s0b0n0"}, Groups: []string{"red"}},
		{Kernel: "/kernel", Groups: []string{"red", "blue"}},
		{Kernel: "/kernel", Hosts: []string{"x0c0s*"}},
	} {
		source.Spec = spec
		if _, err := BuildPlan(source, "production", "canary"); err == nil || !strings.Contains(err.Error(), source.Metadata.UID) {
			t.Fatal("unsafe source accepted or not identified")
		}
	}
}
