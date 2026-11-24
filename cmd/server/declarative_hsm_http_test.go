// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/boot-service/pkg/clients/hsm"
	"github.com/openchami/boot-service/pkg/declarativeboot"
)

func TestDeclarativeDeliveryRealRouterUsesFreshSMD(t *testing.T) {
	smd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hsm/v2/State/Components":
			_, _ = io.WriteString(w, `{"Components":[{"ID":"x0c0s0b0n0","Type":"Node","NID":42}]}`)
		case "/hsm/v2/Inventory/EthernetInterfaces":
			_, _ = io.WriteString(w, `[{"ComponentID":"x0c0s0b0n0","MACAddress":"aa:bb:cc:dd:ee:ff"}]`)
		case "/hsm/v2/memberships":
			_, _ = io.WriteString(w, `[{"id":"x0c0s0b0n0","groupLabels":["compute"]}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer smd.Close()
	issuer := newTestIssuer(t)
	t.Setenv(jwksURLEnv, issuer.jwksURL)
	registerResourcePrefixesForTest(t)
	cfg := DefaultConfig()
	cfg.AuthEnabled, cfg.TokenSmithURL = true, issuer.jwksURL
	cfg.DataDir, cfg.HSMURL, cfg.HSMSyncEnabled = filepath.Join(t.TempDir(), "data"), smd.URL, false
	features, err := loadGeneratedFeatures(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(features.Close)
	manager, err := declarativeboot.NewManager(context.Background(), declarativeboot.GeneratedStore(), declarativeboot.NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	hsmConfig := hsm.DefaultHSMConfig()
	hsmConfig.BaseURL = smd.URL
	hsmClient, err := hsm.NewHSMClient(hsmConfig, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	boot, err := newBootRoutes(*cfg, hsmClient, context.Background(), manager)
	if err != nil {
		t.Fatal(err)
	}
	router := newRouter(*cfg, features, boot)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	config := `{"metadata":{"name":"os"},"spec":{"kernel":"https://images.example.invalid/smd-kernel"}}`
	if response := requestForTest(router, "POST", "/bootconfigs?revisionName=A", config, admin); response.Code != 201 {
		t.Fatalf("config: %d %s", response.Code, response.Body)
	}
	assignment := `{"metadata":{"name":"group"},"spec":{"selector":{"groups":["compute"]},"configRef":{"name":"os"}}}`
	if response := requestForTest(router, "POST", "/bootassignments", assignment, admin); response.Code != 201 {
		t.Fatalf("assignment: %d %s", response.Code, response.Body)
	}
	response := requestForTest(router, "GET", "/bootscript?mac=aa:bb:cc:dd:ee:ff", "", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), "https://images.example.invalid/smd-kernel") {
		t.Fatalf("SMD boot: %d %s", response.Code, response.Body)
	}
	smd.Close()
	response = requestForTest(router, "GET", "/bootscript?mac=aa:bb:cc:dd:ee:ff", "", "")
	if response.Code != http.StatusServiceUnavailable || !strings.HasPrefix(response.Body.String(), "#!ipxe\n") || strings.Contains(response.Body.String(), "smd-kernel") {
		t.Fatalf("failed SMD must refuse: %d %s", response.Code, response.Body)
	}
}
