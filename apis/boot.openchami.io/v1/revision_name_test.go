// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1_test

import (
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

func TestValidateRevisionName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{"mixed case", "Release.A-1", true},
		{"maximum length", strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61), true},
		{"empty", "", false},
		{"too long", strings.Repeat("a", 254), false},
		{"long label", strings.Repeat("a", 64), false},
		{"empty label", "a..b", false},
		{"path separator", "a/b", false},
		{"leading hyphen", "-release", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v1.ValidateRevisionName(tc.value)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidateRevisionName(%q) = %v, want valid %t", tc.value, err, tc.valid)
			}
		})
	}
}
