// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ScopeReadBoot    = "read:boot"
	ScopeProposeBoot = "propose:boot"
	ScopePromoteBoot = "promote:boot"
)

var (
	ErrDeclarativeInvalid     = errors.New("invalid authentication")
	ErrDeclarativeUnavailable = errors.New("authentication unavailable")
)

// DeclarativeConfig is independent of the legacy authentication configuration.
type DeclarativeConfig struct {
	Enabled    bool
	Issuer     string
	JWKSURL    string
	HTTPClient *http.Client
}

// DeclarativeVerifier verifies only new management requests; it never starts background work.
type DeclarativeVerifier struct {
	issuer  string
	jwksURL string
	client  *http.Client
	cache   declarativeKeyCache
	ready   bool
}

// NewDeclarativeVerifier always returns a fail-closed verifier, even for invalid settings.
func NewDeclarativeVerifier(cfg DeclarativeConfig) *DeclarativeVerifier {
	v := &DeclarativeVerifier{}
	if !cfg.Enabled || !validDeclarativeURL(cfg.Issuer) || !validDeclarativeURL(cfg.JWKSURL) {
		return v
	}
	client := &http.Client{}
	if cfg.HTTPClient != nil {
		*client = *cfg.HTTPClient
	}
	client.Timeout = 3 * time.Second
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	v.issuer, v.jwksURL, v.client, v.ready = cfg.Issuer, cfg.JWKSURL, client, true
	v.cache.slots = make(chan struct{}, 8)
	return v
}

func validDeclarativeURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && u.Fragment == "" && u.RawQuery == ""
}

// GoString and String prevent accidental disclosure of configuration in debug output.
func (*DeclarativeVerifier) String() string   { return "declarative verifier" }
func (*DeclarativeVerifier) GoString() string { return "declarative verifier" }

// DeclarativePrincipal contains only claims from a successfully verified token.
type DeclarativePrincipal struct {
	subject string
	scopes  []string
}

// Subject returns the verified subject, when present.
func (p DeclarativePrincipal) Subject() string { return p.subject }

// HasScope checks exact capability membership without implied roles or inheritance.
func (p DeclarativePrincipal) HasScope(required string) bool {
	if required == "" {
		return false
	}
	for _, scope := range p.scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func (DeclarativePrincipal) String() string   { return "verified principal" }
func (DeclarativePrincipal) GoString() string { return "verified principal" }

type declarativePrincipalKey struct{}

// DeclarativePrincipalFromContext retrieves the principal placed by Require.
func DeclarativePrincipalFromContext(ctx context.Context) (DeclarativePrincipal, bool) {
	p, ok := ctx.Value(declarativePrincipalKey{}).(DeclarativePrincipal)
	return p, ok
}

// Verify validates a bearer token once and returns a principal or a safe sentinel error.
func (v *DeclarativeVerifier) Verify(ctx context.Context, token string) (DeclarativePrincipal, error) {
	if v == nil || !v.ready {
		return DeclarativePrincipal{}, ErrDeclarativeUnavailable
	}
	if len(token) > 16384 || !validJWTObjects(token) {
		return DeclarativePrincipal{}, ErrDeclarativeInvalid
	}
	claims := &declarativeClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" || len(kid) > 256 {
			return nil, ErrDeclarativeInvalid
		}
		return v.lookupKey(ctx, kid, t.Method.Alg())
	}, jwt.WithValidMethods(declarativeAlgorithms), jwt.WithIssuer(v.issuer), jwt.WithAudience("boot-service"), jwt.WithExpirationRequired(), jwt.WithStrictDecoding())
	if errors.Is(err, ErrDeclarativeUnavailable) {
		return DeclarativePrincipal{}, ErrDeclarativeUnavailable
	}
	if err != nil || parsed == nil || !parsed.Valid {
		return DeclarativePrincipal{}, ErrDeclarativeInvalid
	}
	return DeclarativePrincipal{subject: claims.Subject, scopes: claims.Scope}, nil
}

// Require authenticates once and requires the supplied independent capability.
func (v *DeclarativeVerifier) Require(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v == nil || !v.ready {
				http.Error(w, "authentication unavailable", http.StatusServiceUnavailable)
				return
			}
			headers := r.Header.Values("Authorization")
			if len(headers) != 1 {
				http.Error(w, "invalid authentication", http.StatusUnauthorized)
				return
			}
			scheme, credential, found := strings.Cut(headers[0], " ")
			credential = strings.TrimLeft(credential, " ")
			if !found || len(scheme) != 6 || !strings.EqualFold(scheme, "Bearer") || credential == "" {
				http.Error(w, "invalid authentication", http.StatusUnauthorized)
				return
			}
			for _, c := range []byte(credential) {
				if c <= 0x20 || c >= 0x7f {
					http.Error(w, "invalid authentication", http.StatusUnauthorized)
					return
				}
			}
			p, err := v.Verify(r.Context(), credential)
			if err != nil {
				status := http.StatusUnauthorized
				if errors.Is(err, ErrDeclarativeUnavailable) {
					status = http.StatusServiceUnavailable
				}
				http.Error(w, http.StatusText(status), status)
				return
			}
			if !p.HasScope(scope) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), declarativePrincipalKey{}, p)))
		})
	}
}
