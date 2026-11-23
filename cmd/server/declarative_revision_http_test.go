// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/revision"
)

func TestDeclarativeHTTPRevisionLifecycleAndCorruption(t *testing.T) {
	router, issuer, dir := declarativeTestRouterWithData(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	a := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-a"}}`
	b := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-b"}}`
	created := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", admin, a, "", "")
	if created.Code != http.StatusCreated {
		t.Fatalf("series: %d %s", created.Code, created.Body)
	}
	var aRecord struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &aRecord); err != nil || aRecord.Metadata.UID == "" {
		t.Fatalf("A decode: %v", err)
	}
	path := "/bootconfigs/os/revisions/B"
	bCreated := declarativeRequest(router, "PUT", path, admin, b, "", "")
	if bCreated.Code != http.StatusCreated {
		t.Fatalf("ensure B: %d %s", bCreated.Code, bCreated.Body)
	}
	var bRecord struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(bCreated.Body.Bytes(), &bRecord); err != nil || bRecord.Metadata.UID == "" {
		t.Fatalf("B decode: %v", err)
	}
	if replay := declarativeRequest(router, "PUT", path, admin, b, "", ""); replay.Code != 200 || replay.Header().Get("ETag") != bCreated.Header().Get("ETag") {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body)
	}
	if conflict := declarativeRequest(router, "PUT", path, admin, a, "", ""); conflict.Code != http.StatusConflict {
		t.Fatalf("conflict: %d", conflict.Code)
	}
	promote := fmt.Sprintf(`{"revisionUid":%q}`, bRecord.Metadata.UID)
	if response := declarativeRequest(router, "PUT", "/bootconfigs/os/default", admin, promote, "", ""); response.Code != http.StatusPreconditionFailed {
		t.Fatalf("missing promotion condition: %d", response.Code)
	}
	series := declarativeRequest(router, "GET", "/bootconfigs/os", admin, "", "", "")
	if response := declarativeRequest(router, "PUT", "/bootconfigs/os/default", admin, promote, "If-Match", series.Header().Get("ETag")); response.Code != 200 {
		t.Fatalf("promotion: %d %s", response.Code, response.Body)
	}
	retirePath := "/bootconfigs/os/revisions/by-uid/" + aRecord.Metadata.UID
	retire := declarativeRequest(router, "DELETE", retirePath, admin, "", "", "")
	if retire.Code != http.StatusConflict || !strings.Contains(retire.Body.String(), "boot_config_retirement_disabled") {
		t.Fatalf("retirement: %d %s", retire.Code, retire.Body)
	}
	if response := declarativeRequest(router, "GET", retirePath, admin, "", "", ""); response.Code != 200 {
		t.Fatalf("denied retirement changed record: %d", response.Code)
	}
	unknown, err := revision.NewUID()
	if err != nil {
		t.Fatal(err)
	}
	if response := declarativeRequest(router, "GET", "/bootconfigs/os/revisions/by-uid/"+unknown, admin, "", "", ""); response.Code != 404 {
		t.Fatalf("unknown revision: %d", response.Code)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", aRecord.Metadata.UID+".json")); err != nil {
		t.Fatal(err)
	}
	if response := declarativeRequest(router, "GET", retirePath, admin, "", "", ""); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("corrupt committed revision: %d", response.Code)
	}
}
