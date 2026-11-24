// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeclarativeDeliveryRealRouterCorruptSelectedRevision(t *testing.T) {
	router, issuer, dir := declarativeTestRouterWithData(t)
	legacy := issuer.valid(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	node := `{"spec":{"xname":"x0c0s0b0n0","bootMac":"aa:bb:cc:dd:ee:ff","nid":42}}`
	if response := requestForTest(router, "POST", "/nodes", node, legacy); response.Code != 201 {
		t.Fatalf("node: %d", response.Code)
	}
	config := `{"metadata":{"name":"os"},"spec":{"kernel":"/secret-kernel","params":"secret-params"}}`
	created := requestForTest(router, "POST", "/bootconfigs?revisionName=A", config, admin)
	if created.Code != 201 {
		t.Fatalf("config: %d %s", created.Code, created.Body)
	}
	var record struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &record); err != nil || record.Metadata.UID == "" {
		t.Fatalf("revision decode: %v", err)
	}
	assignment := `{"metadata":{"name":"canary"},"spec":{"selector":{"macs":["aa:bb:cc:dd:ee:ff"]},"configRef":{"name":"os"}}}`
	if response := requestForTest(router, "POST", "/bootassignments", assignment, admin); response.Code != 201 {
		t.Fatalf("assignment: %d %s", response.Code, response.Body)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", record.Metadata.UID+".json")); err != nil {
		t.Fatal(err)
	}
	response := requestForTest(router, http.MethodGet, "/bootscript?mac=aa:bb:cc:dd:ee:ff", "", "")
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Content-Type") != "application/ipxe" {
		t.Fatalf("delivery: %d %s", response.Code, response.Body)
	}
	body := response.Body.String()
	if !strings.Contains(body, "canary") || !strings.Contains(body, "os") || !strings.Contains(body, "exit") || strings.Contains(body, "secret") {
		t.Fatalf("unsafe refusal: %q", body)
	}
}
