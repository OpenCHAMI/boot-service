// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestDeclarativeDeliveryRealRouterProfileAndFallback(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	legacy := issuer.valid(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	node := `{"spec":{"xname":"x0c0s0b0n0","bootMac":"aa:bb:cc:dd:ee:ff","nid":42}}`
	if response := requestForTest(router, http.MethodPost, "/nodes", node, legacy); response.Code != http.StatusCreated {
		t.Fatalf("node HTTP %d: %s", response.Code, response.Body)
	}
	legacyConfig := `{"macs":["aa:bb:cc:dd:ee:ff"],"kernel":"https://example.com/legacy"}`
	if response := requestForTest(router, http.MethodPost, "/bootparameters", legacyConfig, legacy); response.Code != http.StatusCreated {
		t.Fatalf("legacy HTTP %d: %s", response.Code, response.Body)
	}
	config := `{"metadata":{"name":"alpha"},"spec":{"kernel":"https://example.com/new"}}`
	if response := requestForTest(router, http.MethodPost, "/bootconfigs?revisionName=A", config, admin); response.Code != http.StatusCreated {
		t.Fatalf("config HTTP %d: %s", response.Code, response.Body)
	}
	assignment := `{"metadata":{"name":"canary"},"spec":{"selector":{"macs":["aa:bb:cc:dd:ee:ff"]},"profile":"batch","configRef":{"name":"alpha"}}}`
	if response := requestForTest(router, http.MethodPost, "/bootassignments", assignment, admin); response.Code != http.StatusCreated {
		t.Fatalf("assignment HTTP %d: %s", response.Code, response.Body)
	}
	for _, prefix := range []string{"/bootscript", "/boot/v1/bootscript"} {
		for _, tc := range []struct{ profile, artifact string }{{"batch", "https://example.com/new"}, {"other", "https://example.com/legacy"}} {
			response := requestForTest(router, http.MethodGet, prefix+"?mac=aa:bb:cc:dd:ee:ff&profile="+tc.profile, "", "")
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), tc.artifact) {
				t.Fatalf("%s profile %s: HTTP %d %s", prefix, tc.profile, response.Code, response.Body)
			}
		}
	}
}
