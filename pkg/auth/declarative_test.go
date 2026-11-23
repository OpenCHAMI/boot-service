// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthCapabilities(t *testing.T) {
	f := newDeclarativeFixture(t)
	for _, tc := range []struct {
		name     string
		scope    any
		required string
		want     int
	}{
		{"reader", "read:boot", "read:boot", 204},
		{"proposer only", []string{"propose:boot"}, "propose:boot", 204},
		{"promoter only", "promote:boot", "promote:boot", 204},
		{"no implicit read", "propose:boot", "read:boot", 403},
		{"no implicit promotion", "propose:boot read:boot", "promote:boot", 403},
		{"exact match", "read:boot-extra", "read:boot", 403},
		{"malformed scope", []any{"read:boot", 9}, "read:boot", 401},
		{"scope newline", "read:boot\npromote:boot", "read:boot", 401},
		{"null scope", nil, "read:boot", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := declarativeRequest(f.verifier, tc.required, "Bearer "+f.signed(t, tc.scope, nil))
			require.Equal(t, tc.want, w.Code)
		})
	}
}

func TestDeclarativeAuthClaimsAndHeaders(t *testing.T) {
	f := newDeclarativeFixture(t)
	for _, tc := range []struct {
		name    string
		changes map[string]any
	}{
		{"issuer", map[string]any{"iss": "http://wrong"}},
		{"audience", map[string]any{"aud": "wrong"}},
		{"expired", map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}},
		{"missing expiry", map[string]any{"exp": nil}},
		{"future not before", map[string]any{"nbf": time.Now().Add(time.Hour).Unix()}},
		{"malformed audience", map[string]any{"aud": 42}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := declarativeRequest(f.verifier, "read:boot", "Bearer "+f.signed(t, "read:boot", tc.changes))
			require.Equal(t, 401, w.Code)
		})
	}
	valid := f.signed(t, "read:boot", nil)
	for _, headers := range [][]string{nil, {"Bearer"}, {"Basic " + valid}, {"Bearer " + valid, "Bearer " + valid}, {"Bearer " + valid + " extra"}, {"Bearer\n" + valid}} {
		w := declarativeRequest(f.verifier, "read:boot", headers...)
		require.Equal(t, 401, w.Code)
		require.NotContains(t, fmt.Sprintf("%+v %s", w, w.Body.String()), valid)
	}
}

func TestDeclarativeAuthActualHTTP(t *testing.T) {
	f := newDeclarativeFixture(t)
	endpoint := httptest.NewServer(f.verifier.Require("propose:boot")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := DeclarativePrincipalFromContext(r.Context())
		if !ok || !p.HasScope("propose:boot") {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	defer endpoint.Close()
	request, err := http.NewRequest(http.MethodPost, endpoint.URL, nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+f.signed(t, "propose:boot", nil))
	response, err := endpoint.Client().Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusNoContent, response.StatusCode)
}

func TestDeclarativeAuthDisabledAndInvalidSetup(t *testing.T) {
	for _, config := range []DeclarativeConfig{{}, {Enabled: true}, {Enabled: true, Issuer: "bad", JWKSURL: "bad"}} {
		w := declarativeRequest(NewDeclarativeVerifier(config), "read:boot", "Bearer secret-token")
		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		require.False(t, strings.Contains(w.Body.String(), "secret-token"))
	}
}
