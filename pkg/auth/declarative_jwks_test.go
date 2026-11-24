// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthUnusableJWKS(t *testing.T) {
	f := newDeclarativeFixture(t)
	token := f.signed(t, "read:boot", nil)
	private := publicJWK(f.key)
	private["d"] = "secret-private-material"
	badKey := publicJWK(f.key)
	badKey["n"] = "invalid!"
	for _, tc := range []struct {
		name   string
		body   any
		status int
	}{
		{"private key", map[string]any{"keys": []any{private}}, 200},
		{"private EC key", map[string]any{"keys": []any{publicJWK(f.key), map[string]string{"kty": "EC", "kid": "private", "d": "secret-private-material"}}}, 200},
		{"symmetric secret key", map[string]any{"keys": []any{publicJWK(f.key), map[string]string{"kty": "oct", "kid": "secret", "k": "symmetric-private-material"}}}, 200},
		{"malformed key", map[string]any{"keys": []any{badKey}}, 200},
		{"duplicate kid", map[string]any{"keys": []any{publicJWK(f.key), publicJWK(f.key)}}, 200},
		{"empty set", map[string]any{"keys": []any{}}, 200},
		{"upstream unavailable", map[string]string{"secret": "upstream-secret"}, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(tc.body)
			}))
			defer server.Close()
			v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
			w := declarativeRequest(v, "read:boot", "Bearer "+token)
			require.Equal(t, 503, w.Code)
			for _, text := range []string{token, "secret-private-material", "upstream-secret"} {
				require.NotContains(t, fmt.Sprintf("%+v %#v %s", v, v, w.Body.String()), text)
			}
		})
	}
}

func TestDeclarativeAuthRecoversAndRefreshesConcurrently(t *testing.T) {
	f := newDeclarativeFixture(t)
	var available atomic.Bool
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "" || !available.Load() {
			w.WriteHeader(503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{publicJWK(f.key)}})
	}))
	defer server.Close()
	v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
	token := f.signed(t, "read:boot", nil)
	require.Equal(t, 503, declarativeRequest(v, "read:boot", "Bearer "+token).Code)
	available.Store(true)
	var wg sync.WaitGroup
	results := make(chan int, 24)
	for range 24 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- declarativeRequest(v, "read:boot", "Bearer "+token).Code }()
	}
	wg.Wait()
	close(results)
	for status := range results {
		require.Equal(t, 204, status)
	}
	require.GreaterOrEqual(t, requests.Load(), int64(25))
}

func TestDeclarativeAuthCancelledFetch(t *testing.T) {
	f := newDeclarativeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	token := f.signed(t, "read:boot", nil)
	_, err := f.verifier.Verify(ctx, token)
	require.ErrorIs(t, err, ErrDeclarativeUnavailable)
	require.False(t, strings.Contains(fmt.Sprintf("%+v %#v", err, err), token))
}
