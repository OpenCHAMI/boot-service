// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/openchami/boot-service/internal/storage"
)

func TestDeclarativeHTTPRejectsRestoredRetiredReference(t *testing.T) {
	router, issuer := declarativeTestRouter(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	a := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-a"}}`
	b := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-b"}}`
	first := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", admin, a, "", "")
	second := declarativeRequest(router, "PUT", "/bootconfigs/os/revisions/B", admin, b, "", "")
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("create revisions: %d %d", first.Code, second.Code)
	}
	var firstRecord, secondRecord struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstRecord); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondRecord); err != nil {
		t.Fatal(err)
	}
	series := declarativeRequest(router, "GET", "/bootconfigs/os", admin, "", "", "")
	promote := fmt.Sprintf(`{"revisionUid":%q}`, secondRecord.Metadata.UID)
	if result := declarativeRequest(router, "PUT", "/bootconfigs/os/default", admin, promote, "If-Match", series.Header().Get("ETag")); result.Code != http.StatusOK {
		t.Fatalf("promote: %d", result.Code)
	}
	if _, err := storage.RetireBootConfigRevision(context.Background(), "os", firstRecord.Metadata.UID); err != nil {
		t.Fatal(err)
	}
	path := "/bootconfigs/os/revisions/by-uid/" + firstRecord.Metadata.UID
	if result := declarativeRequest(router, "GET", path, admin, "", "", ""); result.Code != http.StatusGone {
		t.Fatalf("retired read: %d", result.Code)
	}
	body := fmt.Sprintf(`{"metadata":{"name":"pin"},"spec":{"configRef":{"name":"os","uid":%q}}}`, firstRecord.Metadata.UID)
	if result := declarativeRequest(router, "POST", "/bootassignments", admin, body, "", ""); result.Code != http.StatusUnprocessableEntity {
		t.Fatalf("retired pin: %d", result.Code)
	}
}
