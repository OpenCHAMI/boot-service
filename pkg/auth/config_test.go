// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Test demonstrating tokensmith middleware integration
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()

		assert.True(t, config.Enabled)
		assert.True(t, config.ValidateExpiration)
		assert.False(t, config.ValidateIssuer)
		assert.False(t, config.ValidateAudience)
		assert.Equal(t, []string{"sub"}, config.RequiredClaims)
		assert.Equal(t, 1*time.Hour, config.JWKSRefreshInterval)
		assert.False(t, config.AllowEmptyToken)
		assert.False(t, config.NonEnforcing)
	})

	t.Run("DevConfig", func(t *testing.T) {
		config := DevConfig()

		assert.False(t, config.Enabled) // Auth disabled in dev
		assert.True(t, config.AllowEmptyToken)
		assert.True(t, config.NonEnforcing)
		assert.False(t, config.ValidateExpiration)
		assert.False(t, config.ValidateIssuer)
		assert.False(t, config.ValidateAudience)
		assert.Empty(t, config.RequiredClaims)
	})
}

func TestCreateMiddleware(t *testing.T) {
	t.Run("DisabledAuth", func(t *testing.T) {
		config := Config{Enabled: false}
		middleware := config.CreateMiddleware(nil)

		// Test that disabled auth allows all requests through
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("EnabledAuthWithoutToken", func(t *testing.T) {
		config := DefaultConfig()
		middleware := config.CreateMiddleware(nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		// Should get 401 unauthorized without token
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestScopeMiddleware(t *testing.T) {
	t.Run("NoScopesRequired", func(t *testing.T) {
		middleware := CreateScopeMiddleware()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		// Pass-through middleware should not modify the request
		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SingleScopeRequired", func(t *testing.T) {
		middleware := CreateScopeMiddleware("read")
		require.NotNil(t, middleware)

		// The middleware is created but testing actual scope validation
		// would require setting up a full JWT token with claims
		// That's beyond the scope of this basic integration test
	})

	t.Run("MultipleScopesRequired", func(t *testing.T) {
		middleware := CreateScopeMiddleware("read", "write")
		require.NotNil(t, middleware)
	})
}

func TestServiceTokenMiddleware(t *testing.T) {
	t.Run("CreateServiceTokenMiddleware", func(t *testing.T) {
		middleware := CreateServiceTokenMiddleware("boot-service")
		require.NotNil(t, middleware)

		// Again, actual validation would require proper JWT tokens
		// This just tests that the middleware is created successfully
	})
}

func TestConvenienceFunctions(t *testing.T) {
	t.Run("GetClaimsFromRequest", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		// Without JWT middleware, there should be no claims
		claims, err := GetClaimsFromRequest(req)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "claims not found in context")
	})

	t.Run("GetRawClaimsFromRequest", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		// Without JWT middleware, there should be no raw claims
		claims, err := GetRawClaimsFromRequest(req)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "raw claims not found in context")
	})
}
