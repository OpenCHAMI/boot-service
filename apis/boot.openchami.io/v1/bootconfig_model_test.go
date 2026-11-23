// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/resource"
)

func TestBootConfigValidation(t *testing.T) {
	tests := []struct {
		name, resourceName, kernel, initrd string
		valid                              bool
	}{
		{"https", "Release.A-1", "https://images.example.invalid/kernel", "/boot/initrd", true},
		{"absolute path", "default", "/boot/kernel", "", true},
		{"maximum name", strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61), "/kernel", "", true},
		{"missing name", "", "/kernel", "", false},
		{"name too long", strings.Repeat("a", 254), "/kernel", "", false},
		{"label too long", strings.Repeat("a", 64), "/kernel", "", false},
		{"bad name", "bad/name", "/kernel", "", false},
		{"empty label", "a..b", "/kernel", "", false},
		{"missing kernel", "valid", "", "", false},
		{"s3 kernel", "valid", "s3://bucket/kernel", "", false},
		{"relative kernel", "valid", "kernel", "", false},
		{"s3 initrd", "valid", "/kernel", "s3://bucket/initrd", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := BootConfig{Metadata: resource.Metadata{Name: tc.resourceName}, Spec: BootConfigSpec{Kernel: tc.kernel, Initrd: tc.initrd}}
			err := config.Validate(context.Background())
			if (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, want valid %t", err, tc.valid)
			}
		})
	}
}

func TestBootConfigRejectsNonStringContent(t *testing.T) {
	var config BootConfig
	if err := json.Unmarshal([]byte(`{"spec":{"kernel":42}}`), &config); err == nil {
		t.Fatal("numeric kernel was accepted")
	}
}
