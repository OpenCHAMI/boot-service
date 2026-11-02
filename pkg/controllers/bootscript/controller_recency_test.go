// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	apiv1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

func TestGenerateBootScript_Recency(t *testing.T) {
	t.Parallel()
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	tests := []struct {
		name      string
		configure func(*apiv1.BootConfiguration, *apiv1.BootConfiguration)
		profile   string
		want      int
	}{
		{name: "omitted priority selects newest", want: 1},
		{name: "newest name sorts first", configure: func(a, b *apiv1.BootConfiguration) {
			a.Metadata.Name, b.Metadata.Name = "z-debug", "a-base"
		}, want: 1},
		{name: "update of older resource wins", configure: func(a, b *apiv1.BootConfiguration) {
			b.Metadata.CreatedAt = earlier.Add(-time.Hour)
		}, want: 1},
		{name: "explicit zero equals omitted priority", configure: func(a, b *apiv1.BootConfiguration) {
			a.Spec.Priority = 0
		}, want: 1},
		{name: "equal nonzero priorities use recency", configure: func(a, b *apiv1.BootConfiguration) {
			a.Spec.Priority, b.Spec.Priority = 50, 50
		}, want: 1},
		{name: "higher priority beats recency", configure: func(a, b *apiv1.BootConfiguration) {
			a.Spec.Priority = 1
		}, want: 0},
		{name: "higher score beats priority and recency", configure: func(a, b *apiv1.BootConfiguration) {
			b.Spec.MACs = nil
			b.Spec.Hosts = []string{"x0c0s0b0n0"}
			b.Spec.Priority = 100
		}, want: 0},
		{name: "missing update uses creation", configure: func(a, b *apiv1.BootConfiguration) {
			a.Metadata.UpdatedAt, b.Metadata.UpdatedAt = time.Time{}, time.Time{}
		}, want: 1},
		{name: "known time beats unknown time", configure: func(a, b *apiv1.BootConfiguration) {
			a.Metadata.CreatedAt, a.Metadata.UpdatedAt = time.Time{}, time.Time{}
		}, want: 1},
		{name: "equal times use name", configure: func(a, b *apiv1.BootConfiguration) {
			b.Metadata.UpdatedAt = earlier
		}, want: 0},
		{name: "same instant in another timezone uses name", configure: func(a, b *apiv1.BootConfiguration) {
			b.Metadata.UpdatedAt = earlier.In(time.FixedZone("offset", 3600))
		}, want: 0},
		{name: "missing times use name", configure: func(a, b *apiv1.BootConfiguration) {
			a.Metadata.CreatedAt, a.Metadata.UpdatedAt = time.Time{}, time.Time{}
			b.Metadata.CreatedAt, b.Metadata.UpdatedAt = time.Time{}, time.Time{}
		}, want: 0},
		{name: "equal time and name use UID", configure: func(a, b *apiv1.BootConfiguration) {
			b.Metadata.UpdatedAt = earlier
			b.Metadata.Name = a.Metadata.Name
		}, want: 0},
		{name: "explicit default profile", profile: "default", want: 1},
		{name: "requested profile fallback", profile: "missing", want: 1},
		{name: "profileless recency across profiles", configure: func(a, b *apiv1.BootConfiguration) {
			a.Spec.Profile, b.Spec.Profile = "compute", "service"
		}, want: 1},
		{name: "requested profile is retained", profile: "compute", configure: func(a, b *apiv1.BootConfiguration) {
			a.Spec.Profile, b.Spec.Profile = "compute", "service"
		}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configs []apiv1.BootConfiguration
			// Decode omitted priority through the same API representation as clients.
			if err := json.Unmarshal([]byte(`[
  {
    "metadata": {
      "name": "a-debug",
      "uid": "bootconfig-a"
    },
    "spec": {
      "macs": [
        "02:00:00:00:00:01"
      ],
      "kernel": "http://example.invalid/debug",
      "initrd": "http://example.invalid/debug-initrd",
      "params": "image=debug"
    }
  },
  {
    "metadata": {
      "name": "z-base",
      "uid": "bootconfig-b"
    },
    "spec": {
      "macs": [
        "02:00:00:00:00:01"
      ],
      "kernel": "http://example.invalid/base",
      "initrd": "http://example.invalid/base-initrd",
      "params": "image=base"
    }
  }
]`), &configs); err != nil {
				t.Fatal(err)
			}
			configs[0].Metadata.CreatedAt, configs[0].Metadata.UpdatedAt = earlier, earlier
			configs[1].Metadata.CreatedAt, configs[1].Metadata.UpdatedAt = later, later
			if tt.configure != nil {
				tt.configure(&configs[0], &configs[1])
			}
			expected := configs[tt.want]
			nodes := []apiv1.Node{{Spec: apiv1.NodeSpec{XName: "x0c0s0b0n0", BootMAC: "02:00:00:00:00:01", NID: 1}}}
			for _, order := range []string{"forward", "reverse"} {
				t.Run(order, func(t *testing.T) {
					ordered := append([]apiv1.BootConfiguration(nil), configs...)
					if order == "reverse" {
						ordered[0], ordered[1] = ordered[1], ordered[0]
					}
					controller := newTestControllerWithData(t, nodes, ordered)
					script, err := controller.GenerateBootScript(context.Background(), nodes[0].Spec.BootMAC, tt.profile)
					if err != nil {
						t.Fatal(err)
					}
					for _, value := range []string{expected.Spec.Kernel, expected.Spec.Initrd, expected.Spec.Params} {
						if !strings.Contains(script, value) {
							t.Errorf("selected script missing %q: %s", value, script)
						}
					}
				})
			}
		})
	}
}
