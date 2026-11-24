// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthBoundsJWKSBody(t *testing.T) {
	f := newDeclarativeFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", declarativeJWKSMaxBytes+1)))
	}))
	defer server.Close()
	v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
	token := f.signed(t, "read:boot", nil)
	require.Equal(t, http.StatusServiceUnavailable, declarativeRequest(v, "read:boot", "Bearer "+token).Code)
}

func TestDeclarativeAuthCancelsInFlightFetch(t *testing.T) {
	f := newDeclarativeFixture(t)
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()
	v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
	token := f.signed(t, "read:boot", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := v.Verify(ctx, token); result <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("JWKS request did not start")
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrDeclarativeUnavailable)
	case <-time.After(2 * time.Second):
		t.Fatal("JWKS request did not cancel")
	}
}

func TestDeclarativeAuthLimitsConcurrentFetchAndCancelsWaiter(t *testing.T) {
	f := newDeclarativeFixture(t)
	entered := make(chan struct{}, 16)
	release := make(chan struct{})
	var active, peak atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		entered <- struct{}{}
		select {
		case <-release:
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{publicJWK(f.key)}})
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	v := NewDeclarativeVerifier(DeclarativeConfig{Enabled: true, Issuer: testDeclarativeIssuer, JWKSURL: server.URL, HTTPClient: server.Client()})
	token := f.signed(t, "read:boot", nil)
	var wg sync.WaitGroup
	results := make(chan int, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- declarativeRequest(v, "read:boot", "Bearer "+token).Code }()
	}
	for range 8 {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			close(release)
			wg.Wait()
			t.Fatal("concurrent requests did not reach JWKS")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := v.Verify(ctx, token)
	close(release)
	wg.Wait()
	close(results)
	require.ErrorIs(t, err, ErrDeclarativeUnavailable)
	for status := range results {
		require.Equal(t, http.StatusNoContent, status)
	}
	require.LessOrEqual(t, peak.Load(), int64(8))
}
