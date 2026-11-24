// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/openchami/fabrica/pkg/revision"
)

func TestNativeIdentityClassificationPreservesCause(t *testing.T) {
	// Given native identity, cancellation, and I/O errors.
	tests := []struct {
		name    string
		input   error
		want    error
		corrupt bool
	}{
		{"identity", fmt.Errorf("native: %w", revision.ErrRevisionIdentityMismatch), revision.ErrRevisionIdentityMismatch, true},
		{"cancellation", context.Canceled, context.Canceled, false},
		{"io", io.ErrUnexpectedEOF, io.ErrUnexpectedEOF, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// When the native error crosses the BOOT boundary.
			err := classifyRevisionIdentityError(tc.input)
			// Then the original cause remains visible with the correct category.
			if !errors.Is(err, tc.want) || errors.Is(err, ErrRevisionCorrupt) != tc.corrupt {
				t.Fatalf("got %v, cause %v, corrupt %v", err, tc.want, tc.corrupt)
			}
		})
	}
}
