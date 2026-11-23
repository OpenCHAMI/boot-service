// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthAlgorithmPolicy(t *testing.T) {
	f := newDeclarativeFixture(t)
	jwk := publicJWK(f.key)
	delete(jwk, "alg")
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecKey := map[string]string{"kty": "EC", "kid": "ec-key", "use": "sig", "alg": "ES256", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(ec.X.FillBytes(make([]byte, 32))), "y": base64.RawURLEncoding.EncodeToString(ec.Y.FillBytes(make([]byte, 32)))}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk, ecKey}})
	}))
	defer server.Close()
	v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
	claims := jwt.MapClaims{"iss": testDeclarativeIssuer, "aud": "boot-service", "exp": time.Now().Add(time.Hour).Unix(), "scope": "read:boot"}
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodRS256, jwt.SigningMethodRS384, jwt.SigningMethodRS512, jwt.SigningMethodPS256, jwt.SigningMethodPS384, jwt.SigningMethodPS512} {
		t.Run(method.Alg(), func(t *testing.T) {
			token := jwt.NewWithClaims(method, claims)
			token.Header["kid"] = "key-1"
			signed, err := token.SignedString(f.key)
			require.NoError(t, err)
			require.Equal(t, 204, declarativeRequest(v, "read:boot", "Bearer "+signed).Code)
		})
	}
	t.Run("trusted ES256 rejected", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		token.Header["kid"] = "ec-key"
		signed, err := token.SignedString(ec)
		require.NoError(t, err)
		require.Equal(t, 401, declarativeRequest(v, "read:boot", "Bearer "+signed).Code)
	})
}

func TestDeclarativeAuthBadSignatureAndUnknownKey(t *testing.T) {
	f := newDeclarativeFixture(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	claims := jwt.MapClaims{"iss": testDeclarativeIssuer, "aud": "boot-service", "exp": time.Now().Add(time.Hour).Unix(), "scope": "read:boot"}
	for _, tc := range []struct {
		name, kid string
		key       *rsa.PrivateKey
	}{
		{"bad signature", "key-1", other}, {"unknown kid", "unknown", f.key}, {"missing kid", "", f.key},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			if tc.kid != "" {
				token.Header["kid"] = tc.kid
			}
			signed, err := token.SignedString(tc.key)
			require.NoError(t, err)
			w := declarativeRequest(f.verifier, "read:boot", "Bearer "+signed)
			require.Equal(t, 401, w.Code)
			require.NotContains(t, w.Body.String(), signed)
		})
	}
}

func TestDeclarativeAuthRejectsJWKAlgorithmMismatch(t *testing.T) {
	f := newDeclarativeFixture(t)
	claims := jwt.MapClaims{"iss": testDeclarativeIssuer, "aud": "boot-service", "exp": time.Now().Add(time.Hour).Unix(), "scope": "read:boot"}
	token := jwt.NewWithClaims(jwt.SigningMethodRS384, claims)
	token.Header["kid"] = "key-1"
	signed, err := token.SignedString(f.key)
	require.NoError(t, err)
	require.Equal(t, 401, declarativeRequest(f.verifier, "read:boot", "Bearer "+signed).Code)
}
