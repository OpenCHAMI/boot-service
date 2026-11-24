// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openchami/boot-service/pkg/declarativeboot"
)

func declarativeTestRouter(t *testing.T) (http.Handler, *testIssuer) {
	router, issuer, _ := declarativeTestRouterWithData(t)
	return router, issuer
}

func declarativeTestRouterWithData(t *testing.T) (http.Handler, *testIssuer, string) {
	t.Helper()
	issuer := newTestIssuer(t)
	t.Setenv(jwksURLEnv, issuer.jwksURL)
	registerResourcePrefixesForTest(t)
	cfg := DefaultConfig()
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	cfg.AuthEnabled, cfg.TokenSmithURL = true, issuer.jwksURL
	cfg.EnableLegacyAPI = true
	features, err := loadGeneratedFeatures(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(features.Close)
	manager, err := declarativeboot.NewManager(context.Background(), declarativeboot.GeneratedStore(), declarativeboot.NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	boot, err := newBootRoutes(*cfg, nil, context.Background(), manager)
	if err != nil {
		t.Fatal(err)
	}
	return newRouter(*cfg, features, boot), issuer, cfg.DataDir
}

func declarativeTestToken(t *testing.T, issuer *testIssuer, scope string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": issuer.jwksURL, "aud": "boot-service", "exp": time.Now().Add(time.Hour).Unix(), "scope": scope,
	})
	token.Header["kid"] = issuer.kid
	signed, err := token.SignedString(issuer.key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestDeclarativeHTTPIndependentCapabilities(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	proposer := declarativeTestToken(t, issuer, "propose:boot")
	reader := declarativeTestToken(t, issuer, "read:boot")
	promoter := declarativeTestToken(t, issuer, "promote:boot")
	config := `{"metadata":{"name":"alpha"},"spec":{"kernel":"/boot/kernel-a"}}`
	response := requestForTest(router, http.MethodPost, "/bootconfigs?revisionName=A", config, proposer)
	if response.Code != http.StatusCreated {
		t.Fatalf("proposer create: HTTP %d: %s", response.Code, response.Body)
	}
	response = requestForTest(router, http.MethodGet, "/bootconfigs/alpha", "", reader)
	if response.Code != http.StatusOK {
		t.Fatalf("reader get: HTTP %d: %s", response.Code, response.Body)
	}
	assignment := `{"metadata":{"name":"test-node"},"spec":{"configRef":{"name":"alpha"}}}`
	response = requestForTest(router, http.MethodPost, "/bootassignments", assignment, promoter)
	if response.Code != http.StatusCreated {
		t.Fatalf("promoter assignment: HTTP %d: %s", response.Code, response.Body)
	}
	response = requestForTest(router, http.MethodPost, "/bootassignments", assignment, proposer)
	if response.Code != http.StatusForbidden {
		t.Fatalf("proposer assignment: HTTP %d, want 403", response.Code)
	}
}
