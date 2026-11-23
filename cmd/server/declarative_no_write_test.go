// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func declarativeFileDigest(t *testing.T, root string) [32]byte {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(relative)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(relative))
		binary.BigEndian.PutUint64(size[:], uint64(len(data)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func TestDeclarativeHTTPDeniedWritesLeaveFilesUnchanged(t *testing.T) {
	router, issuer, dir := declarativeTestRouterWithData(t)
	admin := declarativeTestToken(t, issuer, "read:boot propose:boot promote:boot")
	proposer := declarativeTestToken(t, issuer, "propose:boot")
	config := `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-a"}}`
	if response := declarativeRequest(router, "POST", "/bootconfigs?revisionName=A", admin, config, "", ""); response.Code != 201 {
		t.Fatalf("series: %d", response.Code)
	}
	latest := `{"metadata":{"name":"latest"},"spec":{"configRef":{"name":"os","selector":"latest"}}}`
	if response := declarativeRequest(router, "POST", "/bootassignments", admin, latest, "", ""); response.Code != 201 {
		t.Fatalf("assignment: %d", response.Code)
	}
	before := declarativeFileDigest(t, dir)
	tests := []struct {
		method, path, token, body string
		status                    int
	}{
		{"POST", "/bootassignments", proposer, `{"metadata":{"name":"denied"},"spec":{"configRef":{"name":"os"}}}`, 403},
		{"PUT", "/bootconfigs/os/revisions/B", proposer, `{"metadata":{"name":"os"},"spec":{"kernel":"/kernel-b"}}`, 403},
		{"POST", "/bootassignments", admin, `{"metadata":{"name":"invalid"},"spec":{"configRef":{"name":"missing"}}}`, 422},
		{"POST", "/bootassignments", admin, `{"metadata":{"name":"invalid"},"spec":{"configRef":{"name":"os"},"latest":"bad"}}`, 400},
	}
	for _, tc := range tests {
		response := declarativeRequest(router, tc.method, tc.path, tc.token, tc.body, "", "")
		if response.Code != tc.status {
			t.Fatalf("%s %s: HTTP %d", tc.method, tc.path, response.Code)
		}
		if after := declarativeFileDigest(t, dir); after != before {
			t.Fatalf("denied %s %s changed file inventory", tc.method, tc.path)
		}
	}
}
