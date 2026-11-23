// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const testDeclarativeIssuer = "http://tokensmith.example/issuer"

type declarativeFixture struct {
	key      *rsa.PrivateKey
	server   *httptest.Server
	verifier *DeclarativeVerifier
}

func newDeclarativeFixture(t *testing.T) *declarativeFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{publicJWK(key)}})
	}))
	t.Cleanup(server.Close)
	return &declarativeFixture{key: key, server: server, verifier: NewDeclarativeVerifier(DeclarativeConfig{
		Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client(),
	})}
}

func publicJWK(key *rsa.PrivateKey) map[string]string {
	return map[string]string{
		"kty": "RSA", "kid": "key-1", "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

func (f *declarativeFixture) signed(t *testing.T, scope any, changes map[string]any) string {
	t.Helper()
	claims := jwt.MapClaims{"iss": testDeclarativeIssuer, "aud": []string{"other", "boot-service"}, "sub": "user", "exp": time.Now().Add(time.Hour).Unix(), "scope": scope}
	for k, v := range changes {
		claims[k] = v
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "key-1"
	value, err := token.SignedString(f.key)
	require.NoError(t, err)
	return value
}

func declarativeRequest(v *DeclarativeVerifier, scope string, headers ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/management", nil)
	for _, h := range headers {
		r.Header.Add("Authorization", h)
	}
	w := httptest.NewRecorder()
	v.Require(scope)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := DeclarativePrincipalFromContext(r.Context())
		if !ok || !p.HasScope(scope) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(w, r)
	return w
}
