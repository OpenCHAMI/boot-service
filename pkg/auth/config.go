// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package auth handles authentication configuration for OpenCHAMI boot service using tokensmith
package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openchami/tokensmith/pkg/authn"
	"github.com/openchami/tokensmith/pkg/token"
)

type claimsContextKey struct{}

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
	if c.NonEnforcing {
		logger.Printf("Authentication non-enforcing mode enabled")
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Determine key source
	var staticKey *rsa.PublicKey
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
				staticKey = rsaKey
				logger.Printf("Using static RSA public key")
			} else {
				logger.Printf("Public key is not RSA type")
			}
		}
	}

	var jwtMiddleware func(http.Handler) http.Handler
	if staticKey != nil {
		jwtMiddleware = c.createStaticKeyMiddleware(staticKey, logger)
	} else if c.JWKSURL != "" {
		logger.Printf("Using JWKS URL: %s", c.JWKSURL)
		opt := authn.Options{
			DisableIssuerValidation:   !c.ValidateIssuer,
			DisableAudienceValidation: !c.ValidateAudience,
			ClockSkew:                 2 * time.Minute,
			JWKSURLs:                  []string{c.JWKSURL},
		}
		if c.JWKSRefreshInterval > 0 {
			opt.JWKSCacheSoftTTL = c.JWKSRefreshInterval
			opt.JWKSCacheHardTTL = c.JWKSRefreshInterval * 2
		}
		if c.ValidateIssuer && c.JWTIssuer != "" {
			opt.Issuers = []string{c.JWTIssuer}
		}
		if c.ValidateAudience && c.JWTAudience != "" {
			opt.Audiences = []string{c.JWTAudience}
		}

		var err error
		jwtMiddleware, err = authn.Middleware(opt)
		if err != nil {
			logger.Printf("Failed to create auth middleware: %v", err)
			return func(_ http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					http.Error(w, "authentication configuration error", http.StatusInternalServerError)
				})
			}
		}
	} else {
		// Keep behavior fail-closed for auth-enabled configs with no verification key.
		return func(_ http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "invalid token", http.StatusUnauthorized)
			})
		}
	}

	// If scopes are required, chain with scope middleware
	if len(c.RequiredScopes) > 0 {
		scopeMiddleware := CreateScopeMiddleware(c.RequiredScopes...)
		return func(next http.Handler) http.Handler {
			return jwtMiddleware(scopeMiddleware(next))
		}
	}

	return jwtMiddleware
}

func (c Config) createStaticKeyMiddleware(staticKey *rsa.PublicKey, logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authzHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authzHeader == "" {
				if c.AllowEmptyToken {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authzHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			claims := &token.TSClaims{}
			tokenString := strings.TrimSpace(parts[1])
			parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (interface{}, error) {
				return staticKey, nil
			})
			if err != nil || !parsedToken.Valid {
				if c.NonEnforcing {
					logger.Printf("non-enforcing auth parse failure: %v", err)
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			if c.ValidateIssuer && c.JWTIssuer != "" && claims.Issuer != c.JWTIssuer {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if c.ValidateAudience && c.JWTAudience != "" {
				ok := false
				for _, aud := range claims.Audience {
					if aud == c.JWTAudience {
						ok = true
						break
					}
				}
				if !ok {
					http.Error(w, "invalid token", http.StatusUnauthorized)
					return
				}
			}
			for _, required := range c.RequiredClaims {
				switch required {
				case "sub":
					if strings.TrimSpace(claims.Subject) == "" {
						http.Error(w, "invalid token", http.StatusUnauthorized)
						return
					}
				case "iss":
					if strings.TrimSpace(claims.Issuer) == "" {
						http.Error(w, "invalid token", http.StatusUnauthorized)
						return
					}
				case "aud":
					if len(claims.Audience) == 0 {
						http.Error(w, "invalid token", http.StatusUnauthorized)
						return
					}
				}
			}

			ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CreateScopeMiddleware creates a middleware that requires specific scopes.
// Returns 403 Forbidden when a valid token lacks the required scope, since the
// request is authenticated but not authorized.
func CreateScopeMiddleware(scopes ...string) func(http.Handler) http.Handler {
	if len(scopes) == 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := GetClaimsFromRequest(r)
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
	requiredService = strings.TrimSpace(requiredService)
	if requiredService == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := GetClaimsFromRequest(r)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			for _, aud := range claims.Audience {
				if aud == requiredService {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "invalid service token audience", http.StatusForbidden)
		})
	}
}

// GetClaimsFromRequest is a convenience function to extract claims from request context
func GetClaimsFromRequest(r *http.Request) (*token.TSClaims, error) {
	if claims, ok := r.Context().Value(claimsContextKey{}).(*token.TSClaims); ok && claims != nil {
		return claims, nil
	}

	mapClaims, ok := authn.VerifiedClaimsFromContext(r.Context())
	if !ok {
		return nil, errors.New("claims not found in context")
	}

	b, err := json.Marshal(mapClaims)
	if err != nil {
		return nil, err
	}

	var claims token.TSClaims
	if err := json.Unmarshal(b, &claims); err != nil {
		return nil, err
	}

	return &claims, nil
}

// GetRawClaimsFromRequest is a compatibility helper that returns request claims.
//
// Deprecated: Prefer GetClaimsFromRequest and map claims into an authz principal.
// This function intentionally delegates to GetClaimsFromRequest to avoid depending
// on deprecated tokensmith raw-claims APIs.
func GetRawClaimsFromRequest(r *http.Request) (*token.TSClaims, error) {
	return GetClaimsFromRequest(r)
}
