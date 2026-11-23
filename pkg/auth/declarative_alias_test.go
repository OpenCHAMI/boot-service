// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestDeclarativeAuthRejectsSignedSecurityClaimAliases(t *testing.T) {
	f := newDeclarativeFixture(t)
	now := time.Now().Unix()
	fields := []struct{ name, value, altered string }{
		{"scope", `"read:boot"`, `"promote:boot"`},
		{"iss", fmt.Sprintf("%q", testDeclarativeIssuer), `"http://other.invalid"`},
		{"aud", `"boot-service"`, `"other"`},
		{"exp", fmt.Sprint(now + 3600), fmt.Sprint(now - 3600)},
		{"nbf", fmt.Sprint(now - 3600), fmt.Sprint(now + 3600)},
		{"iat", fmt.Sprint(now - 3600), fmt.Sprint(now + 3600)},
		{"sub", `"reader"`, `"other"`},
		{"jti", `"id-one"`, `"id-two"`},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			var base []string
			for _, other := range fields {
				if other.name != field.name {
					base = append(base, fmt.Sprintf("%q:%s", other.name, other.value))
				}
			}
			canonical := fmt.Sprintf("%q:%s", field.name, field.value)
			alias := fmt.Sprintf("%q:%s", strings.ToUpper(field.name), field.altered)
			for _, tc := range []struct {
				name   string
				claims []string
			}{
				{"alias alone", append(append([]string{}, base...), fmt.Sprintf("%q:%s", strings.ToUpper(field.name), field.value))},
				{"canonical then alias", append(append([]string{}, base...), canonical, alias)},
				{"alias then canonical", append(append([]string{}, base...), alias, canonical)},
			} {
				t.Run(tc.name, func(t *testing.T) {
					payload := "{" + strings.Join(tc.claims, ",") + "}"
					input := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"key-1"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(payload))
					signature, err := jwt.SigningMethodRS256.Sign(input, f.key)
					require.NoError(t, err)
					token := input + "." + base64.RawURLEncoding.EncodeToString(signature)
					required := ScopeReadBoot
					if field.name == "scope" && tc.name == "canonical then alias" {
						required = ScopePromoteBoot
					}
					require.Equal(t, http.StatusUnauthorized, declarativeRequest(f.verifier, required, "Bearer "+token).Code)
				})
			}
		})
	}
}

func TestDeclarativeAuthRejectsSignedMixedCaseScope(t *testing.T) {
	f := newDeclarativeFixture(t)
	for _, alias := range []string{"Scope", "sCoPe", "ſcope"} {
		t.Run(alias, func(t *testing.T) {
			token := f.signed(t, "read:boot", map[string]any{alias: "promote:boot"})
			require.Equal(t, http.StatusUnauthorized, declarativeRequest(f.verifier, ScopePromoteBoot, "Bearer "+token).Code)
		})
	}
}

func TestDeclarativeAuthAllowsUnknownExtensionClaims(t *testing.T) {
	f := newDeclarativeFixture(t)
	token := f.signed(t, []string{ScopeReadBoot}, map[string]any{"ScopeExtension": "ignored"})
	require.Equal(t, http.StatusNoContent, declarativeRequest(f.verifier, ScopeReadBoot, "Bearer "+token).Code)
}
