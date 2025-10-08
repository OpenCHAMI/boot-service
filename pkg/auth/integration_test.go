// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Integration tests for authentication with locally created JWT tokens
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openchami/tokensmith/pkg/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationIntegration(t *testing.T) {
	// Generate test key pair
	keyPair, err := GenerateTestKeyPair()
	require.NoError(t, err)

	t.Run("NonEnforcingMode", func(t *testing.T) {
		// Non-enforcing mode logs errors but doesn't block requests
		config := NonEnforcingConfig()
		middleware := config.CreateMiddleware(nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		})

		// Test without token - should succeed in non-enforcing mode
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("ValidTokenWithStaticKey", func(t *testing.T) {
		// Create config with static public key
		config := CreateStaticKeyConfig(keyPair.PublicKeyPEM)
		config.NonEnforcing = false        // Enforce authentication
		config.RequiredScopes = []string{} // Don't require any specific scopes for basic auth test
		middleware := config.CreateMiddleware(nil)

		// Create a valid test token
		testToken, err := CreateTestToken(keyPair, nil)
		require.NoError(t, err)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract claims to verify they're available
			claims, err := GetClaimsFromRequest(r)
			assert.NoError(t, err)
			assert.Equal(t, "test-user", claims.Subject)
			assert.Contains(t, claims.Scope, "read") // Check for simple "read" scope

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("authenticated"))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "authenticated", w.Body.String())
	})

	t.Run("ScopeBasedAuthorization", func(t *testing.T) {
		// Test scope-based authorization with custom scopes
		keyPair, err := GenerateTestKeyPair()
		require.NoError(t, err)

		// Create token with specific scopes - use simple scope names
		tokenWithReadScope, err := CreateTestTokenWithScopes(keyPair, []string{"read"})
		require.NoError(t, err)

		tokenWithWriteScope, err := CreateTestTokenWithScopes(keyPair, []string{"write"})
		require.NoError(t, err)

		config := CreateStaticKeyConfig(keyPair.PublicKeyPEM)
		config.NonEnforcing = false
		authMiddleware := config.CreateMiddleware(nil)

		// Create scope middleware
		readScopeMiddleware := CreateScopeMiddleware("read")
		writeScopeMiddleware := CreateScopeMiddleware("write")

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("authorized"))
		})

		// Test read scope access
		t.Run("ReadScopeAccess", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenWithReadScope)
			w := httptest.NewRecorder()

			// Apply both auth and scope middleware
			combinedHandler := authMiddleware(readScopeMiddleware(handler))
			combinedHandler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})

		// Test write scope access with read token (should fail)
		t.Run("WriteScopeAccessWithReadToken", func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenWithReadScope)
			w := httptest.NewRecorder()

			// Apply both auth and scope middleware
			combinedHandler := authMiddleware(writeScopeMiddleware(handler))
			combinedHandler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
		})

		// Test write scope access with write token (should succeed)
		t.Run("WriteScopeAccessWithWriteToken", func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenWithWriteScope)
			w := httptest.NewRecorder()

			// Apply both auth and scope middleware
			combinedHandler := authMiddleware(writeScopeMiddleware(handler))
			combinedHandler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	})

	t.Run("ServiceToServiceAuthentication", func(t *testing.T) {
		keyPair, err := GenerateTestKeyPair()
		require.NoError(t, err)

		// Create service token
		serviceToken, err := CreateServiceToken(keyPair, "test-service", "boot-service", []string{"service:boot"})
		require.NoError(t, err)

		config := CreateStaticKeyConfig(keyPair.PublicKeyPEM)
		config.JWTIssuer = "test-tokensmith"
		config.JWTAudience = "boot-service"
		config.ValidateIssuer = true
		config.ValidateAudience = true
		authMiddleware := config.CreateMiddleware(nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := GetClaimsFromRequest(r)
			assert.NoError(t, err)
			assert.Equal(t, "test-service", claims.Subject)
			assert.Contains(t, claims.Scope, "service:boot")

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("service authenticated"))
		})

		req := httptest.NewRequest("GET", "/internal/stats", nil)
		req.Header.Set("Authorization", "Bearer "+serviceToken)
		w := httptest.NewRecorder()

		authMiddleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "service authenticated", w.Body.String())
	})

	t.Run("ExpiredTokenHandling", func(t *testing.T) {
		keyPair, err := GenerateTestKeyPair()
		require.NoError(t, err)

		// Create expired token
		now := time.Now()
		expiredClaims := &token.TSClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "test-issuer",
				Subject:   "test-user",
				Audience:  []string{"boot-service"},
				ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)), // Expired 1 hour ago
				NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			},
			Scope: []string{"boot:read"},
		}

		expiredToken, err := CreateTestToken(keyPair, expiredClaims)
		require.NoError(t, err)

		// Test with expiration validation enabled
		config := CreateStaticKeyConfig(keyPair.PublicKeyPEM)
		config.ValidateExpiration = true
		config.NonEnforcing = false
		middleware := config.CreateMiddleware(nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		// Should be unauthorized due to expired token
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("InvalidTokenHandling", func(t *testing.T) {
		config := CreateStaticKeyConfig(keyPair.PublicKeyPEM)
		config.NonEnforcing = false
		middleware := config.CreateMiddleware(nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		testCases := []struct {
			name         string
			token        string
			expectedCode int
		}{
			{
				name:         "InvalidJWT",
				token:        "invalid.jwt.token",
				expectedCode: http.StatusUnauthorized,
			},
			{
				name:         "MalformedHeader",
				token:        "NotABearerToken",
				expectedCode: http.StatusUnauthorized,
			},
			{
				name:         "EmptyToken",
				token:        "",
				expectedCode: http.StatusUnauthorized,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/test", nil)
				if tc.token != "" {
					if tc.name == "MalformedHeader" {
						req.Header.Set("Authorization", tc.token)
					} else {
						req.Header.Set("Authorization", "Bearer "+tc.token)
					}
				}
				w := httptest.NewRecorder()

				middleware(handler).ServeHTTP(w, req)

				assert.Equal(t, tc.expectedCode, w.Code)
			})
		}
	})
}
