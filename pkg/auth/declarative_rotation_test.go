// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthKeyRotation(t *testing.T) {
	f := newDeclarativeFixture(t)
	rotated, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var changed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		key := f.key
		if changed.Load() {
			key = rotated
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{publicJWK(key)}})
	}))
	defer server.Close()
	v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
	old := f.signed(t, "read:boot", nil)
	claims := jwt.MapClaims{"iss": testDeclarativeIssuer, "aud": "boot-service", "exp": time.Now().Add(time.Hour).Unix(), "scope": "read:boot"}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "key-1"
	newToken, err := token.SignedString(rotated)
	require.NoError(t, err)
	require.Equal(t, 204, declarativeRequest(v, "read:boot", "Bearer "+old).Code)
	changed.Store(true)
	require.Equal(t, 204, declarativeRequest(v, "read:boot", "Bearer "+newToken).Code)
	require.Equal(t, 401, declarativeRequest(v, "read:boot", "Bearer "+old).Code)
}

func TestDeclarativeAuthFindsRotatedUnknownKID(t *testing.T) {
	f := newDeclarativeFixture(t)
	rotated, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var changed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		key := publicJWK(f.key)
		if changed.Load() {
			key = publicJWK(rotated)
			key["kid"] = "key-2"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{key}})
	}))
	defer server.Close()
	v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
	claims := jwt.MapClaims{"iss": testDeclarativeIssuer, "aud": "boot-service", "exp": time.Now().Add(time.Hour).Unix(), "scope": "read:boot"}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "key-2"
	signed, err := token.SignedString(rotated)
	require.NoError(t, err)
	require.Equal(t, 401, declarativeRequest(v, "read:boot", "Bearer "+signed).Code)
	changed.Store(true)
	require.Equal(t, 204, declarativeRequest(v, "read:boot", "Bearer "+signed).Code)
}
