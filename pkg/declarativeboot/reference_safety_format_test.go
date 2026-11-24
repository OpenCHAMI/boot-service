// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/revision"
)

type secretValueError string

func (e secretValueError) Error() string { return string(e) }

func TestReferenceIntegrityFormattingRedactsValueCause(t *testing.T) {
	const secret = "synthetic-token-kernel-param-632a8a"
	cause := secretValueError(secret)
	a := assignment("safe-assignment")
	a.Spec.ConfigRef = revision.Reference{Name: "safe-series", RevisionName: "safe-revision"}
	err := WrapSelectedReferenceFailure(a, cause)
	var failure *ReferenceFailure
	if !errors.As(err, &failure) {
		t.Fatal("missing selected-reference failure")
	}
	wrapped := fmt.Errorf("outer: %w", err)
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"pointer", failure},
		{"copied-value", *failure},
		{"wrapped-error", wrapped},
		{"nested-error", fmt.Errorf("outer again: %w", wrapped)},
		{"joined-error", errors.Join(wrapped, errors.New("secondary failure"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
				if strings.Contains(fmt.Sprintf(format, tc.value), secret) {
					t.Errorf("%s exposed the backend cause", format)
				}
			}
			if strings.Contains(fmt.Sprint(tc.value), secret) {
				t.Error("default formatting exposed the backend cause")
			}
		})
	}
	for _, value := range []any{failure, *failure} {
		debug := fmt.Sprintf("%#v", value)
		for _, identity := range []string{"safe-assignment", "safe-series", "safe-revision"} {
			if !strings.Contains(debug, identity) {
				t.Errorf("debug formatting omitted reference identity %q", identity)
			}
		}
	}
	var found *ReferenceFailure
	var backend secretValueError
	if !errors.Is(wrapped, cause) || !errors.As(wrapped, &found) || !errors.As(wrapped, &backend) || errors.Unwrap(failure) != cause {
		t.Fatal("wrapping no longer preserves the selected failure and underlying cause")
	}
}
