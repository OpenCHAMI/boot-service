// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthRejectsAmbiguousSignedObjects(t *testing.T) {
	f := newDeclarativeFixture(t)
	header := `{"alg":"RS256","kid":"key-1"}`
	claims := fmt.Sprintf(`{"iss":%q,"aud":"boot-service","exp":%d,"scope":"read:boot"}`, testDeclarativeIssuer, time.Now().Add(time.Hour).Unix())
	for _, tc := range []struct{ name, header, claims string }{
		{"duplicate algorithm", `{"alg":"none","alg":"RS256","kid":"key-1"}`, claims},
		{"critical header", `{"alg":"RS256","kid":"key-1","crit":["unsupported"]}`, claims},
		{"duplicate scope", header, fmt.Sprintf(`{"iss":%q,"aud":"boot-service","exp":%d,"scope":"read:boot","scope":"promote:boot"}`, testDeclarativeIssuer, time.Now().Add(time.Hour).Unix())},
		{"not an object", header, `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := base64.RawURLEncoding.EncodeToString([]byte(tc.header)) + "." + base64.RawURLEncoding.EncodeToString([]byte(tc.claims))
			signature, err := jwt.SigningMethodRS256.Sign(input, f.key)
			require.NoError(t, err)
			token := input + "." + base64.RawURLEncoding.EncodeToString(signature)
			require.Equal(t, 401, declarativeRequest(f.verifier, "read:boot", "Bearer "+token).Code)
		})
	}
}

func TestDeclarativeAuthFailsClosedWhenJWKSDisappearsAfterCaching(t *testing.T) {
	f := newDeclarativeFixture(t)
	var available atomic.Bool
	available.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !available.Load() {
			w.WriteHeader(503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{publicJWK(f.key)}})
	}))
	defer server.Close()
	v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
	token := f.signed(t, "read:boot", nil)
	require.Equal(t, 204, declarativeRequest(v, "read:boot", "Bearer "+token).Code)
	available.Store(false)
	require.Equal(t, 503, declarativeRequest(v, "read:boot", "Bearer "+token).Code)
	available.Store(true)
	require.Equal(t, 204, declarativeRequest(v, "read:boot", "Bearer "+token).Code)
}
