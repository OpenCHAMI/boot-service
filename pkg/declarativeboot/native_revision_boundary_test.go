// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestNativeRevisionLongCaseSensitiveAndLegacySeries(t *testing.T) {
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
	caps := Capabilities{Propose: true}
	long := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	for _, name := range []string{"aaG", "aaa", long} {
		result, err := svc.CreateSeries(ctx, revisionRequest(name, "A", "/"+name, caps))
		if err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		got, err := svc.ResolveRevision(ctx, revision.Reference{Name: name})
		if err != nil || got.Metadata.UID != result.Record.Metadata.UID {
			t.Fatalf("resolve %q: %#v %v", name, got, err)
		}
	}
	seriesDir := filepath.Join(dir, "revisions", "bootconfig", "series")
	canonical := filepath.Join(seriesDir, fmt.Sprintf("sha256.%x.json", sha256.Sum256([]byte("aaG"))))
	legacy := filepath.Join(seriesDir, base64.RawURLEncoding.EncodeToString([]byte("aaG"))+".json")
	if err := os.Rename(canonical, legacy); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ResolveRevision(ctx, revision.Reference{Name: "aaG", RevisionName: "A"})
	if err != nil || got.Metadata.SeriesName != "aaG" {
		t.Fatalf("legacy: %#v %v", got, err)
	}
}

func TestRevisionPromotionMissingAndStaleConditions(t *testing.T) {
	ctx := context.Background()
	svc, _ := revisionServiceFixture(t)
	caps := Capabilities{Propose: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, etag, err := svc.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	for _, condition := range []string{"", `"stale"`, "W/" + etag, "*"} {
		result, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: b.Record.Metadata.UID, IfMatch: condition, Capabilities: Capabilities{Promote: true}})
		if !errors.Is(err, revision.ErrPreconditionFailed) || result.ETag != etag {
			t.Fatalf("condition %q result %#v error %v", condition, result, err)
		}
	}
	state, after, err := svc.GetSeries(ctx, "config")
	if err != nil || after != etag || state.DefaultRevisionUID != a.Record.Metadata.UID {
		t.Fatalf("state %#v etag %q error %v", state, after, err)
	}
}

func TestProposalRequiresProposeCapability(t *testing.T) {
	ctx := context.Background()
	svc, _ := revisionServiceFixture(t)
	_, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", Capabilities{Promote: true}))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("create: %v", err)
	}
	_, err = svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", Capabilities{Promote: true}))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ensure: %v", err)
	}
	_, err = svc.CreateRevision(ctx, revisionRequest("config", "", "/b", Capabilities{Promote: true}))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("create revision: %v", err)
	}
}

func TestRevisionRetiredNameDifferentContentRemainsConflict(t *testing.T) {
	ctx := context.Background()
	svc, _ := revisionServiceFixture(t)
	caps := Capabilities{Propose: true}
	_, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "C", "/c", caps))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = storage.RetireBootConfigRevision(ctx, "config", b.Record.Metadata.UID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "B", "/different", caps))
	if !errors.Is(err, revision.ErrRevisionNameConflict) {
		t.Fatalf("conflict: %v", err)
	}
}
