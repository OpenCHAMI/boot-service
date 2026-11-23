// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthCachesUnchangedPublicKeys(t *testing.T) {
	f := newDeclarativeFixture(t)
	first, err := f.verifier.lookupKey(context.Background(), "key-1", "RS256")
	require.NoError(t, err)
	second, err := f.verifier.lookupKey(context.Background(), "key-1", "RS256")
	require.NoError(t, err)
	require.Same(t, first, second)
}
