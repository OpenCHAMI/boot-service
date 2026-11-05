// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/openchami/tokensmith/pkg/authz"
	"github.com/openchami/tokensmith/pkg/authz/engine"
	"github.com/openchami/tokensmith/pkg/keys"
	"github.com/openchami/tokensmith/pkg/token"
)

// testIssuer signs tokens and serves the matching JWKS, like TokenSmith.
type testIssuer struct {
	key     *rsa.PrivateKey
	kid     string
	jwksURL string
}

func newTestIssuer(t *testing.T) *testIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	kid, err := keys.RFC7638Thumbprint(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid,
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	t.Cleanup(server.Close)
	return &testIssuer{key: key, kid: kid, jwksURL: server.URL}
}

func (i *testIssuer) token(t *testing.T, key *rsa.PrivateKey, subject string, expires time.Time) string {
	t.Helper()
	return i.tokenFor(t, key, subject, "test-issuer", "boot-service", expires)
}

// tokenFor carries the full claim set TokenSmith's middleware requires.
func (i *testIssuer) tokenFor(t *testing.T, key *rsa.PrivateKey, subject, issuer, audience string, expires time.Time) string {
	t.Helper()
	now := time.Now()
	claims := &token.TSClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			Audience:  []string{audience},
			ExpiresAt: jwt.NewNumericDate(expires),
			NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		},
		AuthLevel:   "IAL2",
		AuthFactors: 2,
		AuthMethods: []string{"password", "mfa"},
		SessionID:   "test-session",
		SessionExp:  expires.Unix(),
		AuthEvents:  []string{"login"},
	}
	unsigned := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	unsigned.Header["kid"] = i.kid
	signed, err := unsigned.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func (i *testIssuer) valid(t *testing.T) string {
	t.Helper()
	return i.token(t, i.key, "operator", time.Now().Add(time.Hour))
}

// newServerRouterForTest assembles the router as runServe does, with the
// generated feature loader and real token validation.
func newServerRouterForTest(t *testing.T, authEnabled bool, jwksURL string) http.Handler {
	t.Helper()
	registerResourcePrefixesForTest(t)
	t.Setenv(jwksURLEnv, jwksURL)

	cfg := DefaultConfig()
	cfg.AuthEnabled = authEnabled
	cfg.EnableLegacyAPI = true
	cfg.DataDir = filepath.Join(t.TempDir(), "data")

	features, err := loadGeneratedFeatures(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(features.Close)

	boot, err := newBootRoutes(*cfg, nil, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return newRouter(*cfg, features, boot)
}

func requestForTest(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

// publicRoutes lists everything reachable without a token. Adding to it exposes
// a route to anonymous callers, so each entry needs a reason.
var publicRoutes = map[string]bool{
	"GET /health":                   true, // liveness probes
	"GET /openapi.json":             true, // API description
	"GET /docs":                     true, // API description
	"GET /metrics":                  true, // scrapers; selected by .fabrica.yaml
	"GET /service/status":           true, // generated public status
	"GET /.well-known/api-catalog":  true, // RFC 9727 discovery
	"HEAD /.well-known/api-catalog": true, // RFC 9727 discovery
	"GET /service/version":          true, // service information
	"GET /bootscript":               true, // nodes boot before they can hold a token
	"GET /boot/v1/bootscript":       true, // legacy clients of the same
	"GET /boot/v1/service/status":   true, // legacy service information
	"GET /boot/v1/service/version":  true, // legacy service information
	"OPTIONS /*":                    true, // generated preflight, carries no data
}

// Walking the router rather than listing protected routes means a new route is
// protected by test as soon as it exists.
func TestEveryRouteOutsideThePublicListRequiresAToken(t *testing.T) {
	issuer := newTestIssuer(t)
	router := newServerRouterForTest(t, true, issuer.jwksURL)

	seen := map[string]bool{}
	err := chi.Walk(router.(chi.Router), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.ReplaceAll(strings.TrimSuffix(route, "/"), "/*", "")
		if route == "" {
			route = "/*"
		}
		key := method + " " + route
		if seen[key] {
			return nil
		}
		seen[key] = true

		path := strings.NewReplacer("{uid}", "x").Replace(route)
		response := requestForTest(router, method, path, "", "")
		// The generated /metrics route answers every method.
		if publicRoutes[key] || route == "/metrics" {
			if response.Code == http.StatusUnauthorized {
				t.Errorf("%s must be public, got 401", key)
			}
		} else if response.Code != http.StatusUnauthorized {
			t.Errorf("%s is not listed as public but returned HTTP %d without a token", key, response.Code)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"GET /nodes", "POST /bmcs", "DELETE /bootconfigurations/x", "PUT /nodes/x/status", "POST /bootparameters", "DELETE /boot/v1/bootparameters"} {
		if !seen[strings.Replace(want, "/x", "/{uid}", 1)] {
			t.Errorf("walk did not reach %s; routes seen: %v", want, seen)
		}
	}
}

func TestTokenValidation(t *testing.T) {
	issuer := newTestIssuer(t)
	router := newServerRouterForTest(t, true, issuer.jwksURL)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
		want  int
	}{
		{name: "no token", want: http.StatusUnauthorized},
		{name: "malformed", token: "not-a-token", want: http.StatusUnauthorized},
		{name: "wrong signing key", token: issuer.token(t, otherKey, "operator", time.Now().Add(time.Hour)), want: http.StatusUnauthorized},
		{name: "expired", token: issuer.token(t, issuer.key, "operator", time.Now().Add(-time.Hour)), want: http.StatusUnauthorized},
		{name: "valid", token: issuer.valid(t), want: http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if response := requestForTest(router, http.MethodGet, "/nodes", "", tc.token); response.Code != tc.want {
				t.Fatalf("got HTTP %d, want %d: %s", response.Code, tc.want, response.Body)
			}
		})
	}
}

// The generated authentication checks signatures and claims but not who a token
// was issued for or by, so a token minted for another service is accepted here.
// rejectUnenforcedAuthSettings exists because of this. If this test starts
// failing after a Fabrica upgrade, the checks work: configure them, remove the
// rejection, and delete this test.
func TestTokensForOtherAudiencesAreAcceptedKnownLimitation(t *testing.T) {
	issuer := newTestIssuer(t)
	router := newServerRouterForTest(t, true, issuer.jwksURL)

	foreign := issuer.tokenFor(t, issuer.key, "someone", "another-issuer", "another-service", time.Now().Add(time.Hour))
	if response := requestForTest(router, http.MethodGet, "/nodes", "", foreign); response.Code != http.StatusOK {
		t.Fatalf("got HTTP %d: %s", response.Code, response.Body)
	}
}

// A trailing slash must not route around the protected group.
func TestTrailingSlashDoesNotBypassAuthentication(t *testing.T) {
	issuer := newTestIssuer(t)
	router := newServerRouterForTest(t, true, issuer.jwksURL)

	for _, path := range []string{"/nodes/", "/bootparameters/", "/boot/v1/bootparameters/"} {
		response := requestForTest(router, http.MethodGet, path, "", "")
		if response.Code == http.StatusOK {
			t.Fatalf("GET %s returned data without a token", path)
		}
		if location := response.Header().Get("Location"); location != "" {
			redirected := requestForTest(router, http.MethodGet, location, "", "")
			if redirected.Code != http.StatusUnauthorized {
				t.Fatalf("GET %s after redirect returned HTTP %d", location, redirected.Code)
			}
		}
	}
}

// Nodes hold no token and boot handlers call the resource API internally, so
// the whole path must work with authentication enabled.
func TestBootFlowsWorkWithAuthenticationEnabled(t *testing.T) {
	issuer := newTestIssuer(t)
	router := newServerRouterForTest(t, true, issuer.jwksURL)
	token := issuer.valid(t)

	const mac = "aa:bb:cc:dd:ee:ff"
	created := requestForTest(router, http.MethodPost, "/bootparameters",
		`{"macs":["`+mac+`"],"kernel":"https://example.com/vmlinuz"}`, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("POST /bootparameters returned HTTP %d: %s", created.Code, created.Body)
	}
	if listed := requestForTest(router, http.MethodGet, "/boot/v1/bootparameters", "", token); listed.Code != http.StatusOK {
		t.Fatalf("GET /boot/v1/bootparameters returned HTTP %d: %s", listed.Code, listed.Body)
	}

	node := requestForTest(router, http.MethodPost, "/nodes",
		`{"spec":{"xname":"x0c0s0b0n0","bootMac":"`+mac+`"}}`, token)
	if node.Code != http.StatusCreated {
		t.Fatalf("POST /nodes returned HTTP %d: %s", node.Code, node.Body)
	}

	for _, path := range []string{"/bootscript?mac=" + mac, "/boot/v1/bootscript?mac=" + mac} {
		script := requestForTest(router, http.MethodGet, path, "", "")
		if script.Code != http.StatusOK || !strings.Contains(script.Body.String(), "https://example.com/vmlinuz") {
			t.Fatalf("GET %s returned HTTP %d: %s", path, script.Code, script.Body)
		}
	}
}

func TestStartupRefusesAuthenticationWithoutJWKSURL(t *testing.T) {
	t.Setenv(jwksURLEnv, "")
	cfg := DefaultConfig()
	cfg.AuthEnabled = true
	cfg.DataDir = filepath.Join(t.TempDir(), "data")

	features, err := loadGeneratedFeatures(context.Background(), cfg)
	if err == nil {
		features.Close()
		t.Fatal("expected authentication without a JWKS URL to fail closed")
	}
	if !strings.Contains(err.Error(), jwksURLEnv) {
		t.Fatalf("error should name %s: %v", jwksURLEnv, err)
	}
}

func TestAuthenticationDisabledLeavesRoutesOpen(t *testing.T) {
	router := newServerRouterForTest(t, false, "")
	for _, path := range []string{"/nodes", "/bootparameters", "/boot/v1/bootparameters"} {
		if response := requestForTest(router, http.MethodGet, path, "", ""); response.Code != http.StatusOK {
			t.Fatalf("GET %s returned HTTP %d: %s", path, response.Code, response.Body)
		}
	}
}

func TestMetricsAreServedOnBothListeners(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	features, err := loadGeneratedFeatures(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(features.Close)

	boot, err := newBootRoutes(*cfg, nil, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mainListener := newRouter(*cfg, features, boot)
	requestForTest(mainListener, http.MethodGet, "/health", "", "")

	for name, handler := range map[string]http.Handler{"main": mainListener, "dedicated": newMetricsHandler(features)} {
		response := requestForTest(handler, http.MethodGet, "/metrics", "", "")
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "boot_service_http_request_total") {
			t.Fatalf("%s listener returned HTTP %d: %.200s", name, response.Code, response.Body)
		}
	}
	if response := requestForTest(newMetricsHandler(features), http.MethodGet, "/nodes", "", ""); response.Code == http.StatusOK {
		t.Fatal("the dedicated listener must not serve the API")
	}
}

// The generated authentication maps only the token subject to a principal, and
// AuthZ evaluates roles alone, so enforce mode denies every token whatever the
// policy says. It fails closed and leaves booting alone. If this test starts
// failing after a Fabrica upgrade, roles reach AuthZ: extend it to cover real
// grants and update docs/AUTHENTICATION.md.
func TestAuthZEnforceModeDeniesEveryTokenKnownLimitation(t *testing.T) {
	issuer := newTestIssuer(t)
	t.Setenv("TOKENSMITH_AUTHZ_MODE", "enforce")
	t.Setenv("TOKENSMITH_CASBIN_MODEL", "../../authz/model.conf")
	t.Setenv("TOKENSMITH_CASBIN_POLICY", "../../authz/policy.csv")
	t.Setenv("TOKENSMITH_CASBIN_GROUPING", "../../authz/grouping.csv")
	router := newServerRouterForTest(t, true, issuer.jwksURL)

	for _, path := range []string{"/nodes", "/bootparameters", "/boot/v1/bootparameters"} {
		response := requestForTest(router, http.MethodGet, path, "", issuer.valid(t))
		if response.Code != http.StatusForbidden {
			t.Fatalf("GET %s returned HTTP %d: %s", path, response.Code, response.Body)
		}
		// Denials report the object checked, which must be what the policy names.
		var denial struct {
			Input struct {
				Object string `json:"object"`
			} `json:"input"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &denial); err != nil {
			t.Fatal(err)
		}
		if denial.Input.Object != path {
			t.Fatalf("GET %s was checked as object %q", path, denial.Input.Object)
		}
	}

	if response := requestForTest(router, http.MethodGet, "/bootscript?mac=aa:bb:cc:dd:ee:ff", "", ""); response.Code != http.StatusOK {
		t.Fatalf("booting must not depend on AuthZ, got HTTP %d", response.Code)
	}
}

// Enforce mode cannot admit anyone yet, so check the checked-in policy against
// the authorizer directly.
func TestCheckedInPolicyCoversBootParameters(t *testing.T) {
	authorizer, err := engine.NewBuilder().
		WithModelPath("../../authz/model.conf").
		WithPolicyPath("../../authz/policy.csv").
		WithGroupingPath("../../authz/grouping.csv").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		role, action string
		want         authz.Decision
	}{
		{"viewer", http.MethodGet, authz.DecisionAllow},
		{"viewer", http.MethodPost, authz.DecisionDeny},
		{"viewer", http.MethodDelete, authz.DecisionDeny},
		{"editor", http.MethodGet, authz.DecisionAllow},
		{"editor", http.MethodPut, authz.DecisionAllow},
		{"editor", http.MethodDelete, authz.DecisionDeny},
		{"admin", http.MethodDelete, authz.DecisionAllow},
	}
	for _, path := range []string{"/bootparameters", "/boot/v1/bootparameters"} {
		for _, tc := range tests {
			t.Run(tc.role+" "+tc.action+" "+path, func(t *testing.T) {
				principal := authz.Principal{ID: "someone", Roles: []string{tc.role}}
				if got, result := authorizer.Authorize(context.Background(), principal, path, tc.action); got != tc.want {
					t.Fatalf("got %v, want %v: %+v", got, tc.want, result)
				}
			})
		}
	}
}
