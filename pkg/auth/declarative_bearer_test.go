// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthBearerGrammarOverHTTP(t *testing.T) {
	f := newDeclarativeFixture(t)
	server := httptest.NewServer(f.verifier.Require(ScopeReadBoot)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	defer server.Close()
	token := f.signed(t, "read:boot", nil)
	for _, tc := range []struct {
		name, header string
		want         int
	}{
		{"one space", "Bearer " + token, 204},
		{"multiple spaces and case", "bEaReR   " + token, 204},
		{"nonbreaking space", "Bearer\u00a0" + token, 401},
		{"em space", "Bearer\u2003" + token, 401},
		{"tab", "Bearer\t" + token, 401},
		{"internal space", "Bearer " + token + " extra", 401},
		{"unicode credential suffix", "Bearer " + token + "\u00a0", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, server.URL, nil)
			require.NoError(t, err)
			r.Header.Set("Authorization", tc.header)
			response, err := server.Client().Do(r)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, tc.want, response.StatusCode)
		})
	}
}

func TestDeclarativeAuthRejectsBearerTrailingSpace(t *testing.T) {
	f := newDeclarativeFixture(t)
	token := f.signed(t, "read:boot", nil)
	require.Equal(t, http.StatusUnauthorized, declarativeRequest(f.verifier, ScopeReadBoot, "Bearer "+token+" ").Code)
}
