// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestDeclarativeHTTPTokensAndAliases(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	reader := declarativeTestToken(t, issuer, "read:boot")
	for _, path := range []string{"/bootassignments/", "/%62ootassignments", "/bootassignments/by-name/test/", "/%62ootconfigs/os/revisions/A"} {
		method := http.MethodGet
		if path == "/%62ootconfigs/os/revisions/A" {
			method = http.MethodPut
		}
		response := requestForTest(router, method, path, "{}", reader)
		if method == http.MethodPut && response.Code != http.StatusForbidden {
			t.Errorf("%s %s: got %d", method, path, response.Code)
		}
		if response.Code == http.StatusMovedPermanently {
			t.Errorf("%s redirected before auth", path)
		}
	}
	for _, path := range []string{"/bootassignments", "/bootassignments/by-name/test/", "/bootassignments/fake/status/", "/%62ootconfigs/os/default", "/bootconfigs/os/revisions/by-uid/rev_fake"} {
		response := requestForTest(router, http.MethodPut, path, "{}", reader)
		if response.Code != http.StatusForbidden {
			t.Errorf("reader PUT %s: got %d", path, response.Code)
		}
	}
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		iss  string
		aud  string
		exp  time.Time
		key  *rsa.PrivateKey
	}{
		{"wrong issuer", "https://wrong.invalid", "boot-service", time.Now().Add(time.Hour), issuer.key},
		{"wrong audience", issuer.jwksURL, "other", time.Now().Add(time.Hour), issuer.key},
		{"expired", issuer.jwksURL, "boot-service", time.Now().Add(-time.Hour), issuer.key},
		{"wrong signature", issuer.jwksURL, "boot-service", time.Now().Add(time.Hour), wrongKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": tc.iss, "aud": tc.aud, "exp": tc.exp.Unix(), "scope": "read:boot"})
			token.Header["kid"] = issuer.kid
			signed, err := token.SignedString(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			response := requestForTest(router, http.MethodGet, "/bootassignments", "", signed)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("HTTP %d, want 401", response.Code)
			}
		})
	}
}

func TestDeclarativeHTTPNotVetoedByLegacyEnforceMode(t *testing.T) {
	t.Setenv("TOKENSMITH_AUTHZ_MODE", "enforce")
	t.Setenv("TOKENSMITH_CASBIN_MODEL", "../../authz/model.conf")
	t.Setenv("TOKENSMITH_CASBIN_POLICY", "../../authz/policy.csv")
	t.Setenv("TOKENSMITH_CASBIN_GROUPING", "../../authz/grouping.csv")
	router, issuer := declarativeTestRouter(t)
	proposer := declarativeTestToken(t, issuer, "propose:boot")
	response := requestForTest(router, http.MethodPost, "/bootconfigs?revisionName=A", `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel"}}`, proposer)
	if response.Code != http.StatusCreated {
		t.Fatalf("proposer denied by legacy middleware: HTTP %d %s", response.Code, response.Body)
	}
}
