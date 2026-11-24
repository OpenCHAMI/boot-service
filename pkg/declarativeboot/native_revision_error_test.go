// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestRevisionValidationAndCancellation(t *testing.T) {
	ctx := context.Background()
	svc, _ := revisionServiceFixture(t)
	caps := Capabilities{Propose: true}
	for _, name := range []string{"", "bad/name", ".", "a..b"} {
		_, err := svc.CreateSeries(ctx, revisionRequest("config", name, "/a", caps))
		if name == "" {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("revision name %q: %v", name, err)
		}
	}
	_, err := svc.EnsureRevision(ctx, revisionRequest("config", "", "/a", caps))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty named ensure: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = svc.CreateRevision(cancelled, revisionRequest("config", "", "/b", caps))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
	state, _, err := svc.GetSeries(ctx, "config")
	if err != nil || state.RevisionCount != 1 {
		t.Fatalf("state %#v error %v", state, err)
	}
}

func TestNativeResolverCorruptionIsOperational(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := storage.InitFileBackend(dir); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ctx, GeneratedStore(), NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewRevisionService(manager)
	_, err = svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "revisions", "bootconfig", "series", fmt.Sprintf("sha256.%x.json", sha256.Sum256([]byte("config"))))
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = (NativeReferenceResolver{}).ResolveReference(ctx, revision.Reference{Name: "config"})
	if err == nil || errors.Is(err, ErrInvalid) {
		t.Fatalf("corruption classified as invalid reference: %v", err)
	}
}
