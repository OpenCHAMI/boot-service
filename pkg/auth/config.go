// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package auth handles authentication configuration for OpenCHAMI boot service using tokensmith
package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"net/http"
	"time"

	tsmiddleware "github.com/openchami/tokensmith/middleware"
	"github.com/openchami/tokensmith/pkg/token"
)

// Config holds authentication configuration for the boot service
type Config struct {
	// Enable/disable authentication entirely
	Enabled bool `json:"enabled"`

	// JWT Configuration
	JWTPublicKey string `json:"jwtPublicKey,omitempty"`
	JWTIssuer    string `json:"jwtIssuer,omitempty"`
	JWTAudience  string `json:"jwtAudience,omitempty"`

	// JWKS Configuration (alternative to static public key)
	JWKSURL             string        `json:"jwksUrl,omitempty"`
	JWKSRefreshInterval time.Duration `json:"jwksRefreshInterval,omitempty"`

	// Validation Options
	ValidateExpiration bool     `json:"validateExpiration"`
	ValidateIssuer     bool     `json:"validateIssuer"`
	ValidateAudience   bool     `json:"validateAudience"`
	RequiredClaims     []string `json:"requiredClaims,omitempty"`
	RequiredScopes     []string `json:"requiredScopes,omitempty"`

	// Development/Testing
	AllowEmptyToken bool `json:"allowEmptyToken"` // For development only
	NonEnforcing    bool `json:"nonEnforcing"`    // Log errors but don't block
}

// DefaultConfig returns sensible defaults for authentication
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		ValidateExpiration:  true,
		ValidateIssuer:      false,           // Often not needed in internal services
		ValidateAudience:    false,           // Often not needed in internal services
		RequiredClaims:      []string{"sub"}, // At minimum require a subject
		RequiredScopes:      []string{},      // No required scopes by default
		JWKSRefreshInterval: 1 * time.Hour,
		AllowEmptyToken:     false,
		NonEnforcing:        false,
	}
}

// DevConfig returns a permissive configuration for development
func DevConfig() Config {
	config := DefaultConfig()
	config.Enabled = false // Disable auth entirely in dev
	config.AllowEmptyToken = true
	config.NonEnforcing = true
	config.ValidateExpiration = false
	config.ValidateIssuer = false
	config.ValidateAudience = false
	config.RequiredClaims = []string{}
	return config
}

// CreateMiddleware creates an HTTP middleware using tokensmith
func (c Config) CreateMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.New(log.Writer(), "auth: ", log.LstdFlags)
	}

	// If auth is disabled, return a pass-through middleware
	if !c.Enabled {
		logger.Printf("Authentication disabled")
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Create tokensmith middleware options
	opts := &tsmiddleware.MiddlewareOptions{
		AllowEmptyToken:     c.AllowEmptyToken,
		ValidateExpiration:  c.ValidateExpiration,
		ValidateIssuer:      c.ValidateIssuer,
		ValidateAudience:    c.ValidateAudience,
		RequiredClaims:      c.RequiredClaims,
		JWKSURL:             c.JWKSURL,
		JWKSRefreshInterval: c.JWKSRefreshInterval,
		NonEnforcing:        c.NonEnforcing,
	}

	// Determine key source
	var key interface{}
	if c.JWTPublicKey != "" {
		// Parse RSA public key from PEM
		keyPem, _ := pem.Decode([]byte(c.JWTPublicKey))
		if keyPem == nil {
			logger.Printf("Failed to decode PEM public key")
		} else {
			pubKey, err := x509.ParsePKIXPublicKey(keyPem.Bytes)
			if err != nil {
				logger.Printf("Failed to parse public key: %v", err)
			} else if rsaKey, ok := pubKey.(*rsa.PublicKey); ok {
				key = rsaKey
				logger.Printf("Using static RSA public key")
			} else {
				logger.Printf("Public key is not RSA type")
			}
		}
	}

	if c.JWKSURL != "" {
		logger.Printf("Using JWKS URL: %s", c.JWKSURL)
		key = nil // Let tokensmith fetch from JWKS
	}

	// Create the JWT middleware
	jwtMiddleware := tsmiddleware.JWTMiddleware(key, opts)

	// If scopes are required, chain with scope middleware
	if len(c.RequiredScopes) > 0 {
		scopeMiddleware := tsmiddleware.RequireScopes(c.RequiredScopes)
		return func(next http.Handler) http.Handler {
			return jwtMiddleware(scopeMiddleware(next))
		}
	}

	return jwtMiddleware
}

// CreateScopeMiddleware creates a middleware that requires specific scopes.
// Returns 403 Forbidden when a valid token lacks the required scope, since the
// request is authenticated but not authorized.
//
// NOTE: This is implemented locally rather than delegating to
// tsmiddleware.RequireScope / tsmiddleware.RequireScopes because those
// functions incorrectly return 401 Unauthorized for missing scopes in all
// current releases of github.com/openchami/tokensmith/middleware. Once that
// bug is fixed upstream, this function body can be replaced with:
//
//	if len(scopes) == 1 { return tsmiddleware.RequireScope(scopes[0]) }
//	return tsmiddleware.RequireScopes(scopes)
func CreateScopeMiddleware(scopes ...string) func(http.Handler) http.Handler {
	if len(scopes) == 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := tsmiddleware.GetClaimsFromContext(r.Context())
			if err != nil {
				// No claims in context means the auth middleware did not run or
				// the token was invalid — treat as unauthenticated.
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			for _, required := range scopes {
				found := false
				for _, s := range claims.Scope {
					if s == required {
						found = true
						break
					}
				}
				if !found {
					// Token is valid but lacks the required scope — authenticated,
					// not authorized.
					http.Error(w, "insufficient scope", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CreateServiceTokenMiddleware creates middleware for service-to-service authentication
func CreateServiceTokenMiddleware(requiredService string) func(http.Handler) http.Handler {
	return tsmiddleware.RequireServiceToken(requiredService)
}

// GetClaimsFromRequest is a convenience function to extract claims from request context
func GetClaimsFromRequest(r *http.Request) (*token.TSClaims, error) {
	return tsmiddleware.GetClaimsFromContext(r.Context())
}

// GetRawClaimsFromRequest is a convenience function to extract raw claims from request context
func GetRawClaimsFromRequest(r *http.Request) (*token.TSClaims, error) {
	return tsmiddleware.GetRawClaimsFromContext(r.Context())
}
