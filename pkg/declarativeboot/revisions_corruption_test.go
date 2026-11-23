// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestRevisionServiceMissingCommittedRecordIsCorrupt(t *testing.T) {
	// Given a real series with A as default, B as latest, and B's file lost.
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
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", caps))
	if err != nil {
		t.Fatal(err)
	}
	before, etag, err := svc.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	if before.DefaultRevisionUID != a.Record.Metadata.UID || before.LatestRevisionUID != b.Record.Metadata.UID || before.RevisionCount != 2 {
		t.Fatalf("unexpected initial state: %#v", before)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", b.Record.Metadata.UID+".json")); err != nil {
		t.Fatal(err)
	}
	ref := revision.Reference{Name: "config", UID: b.Record.Metadata.UID}
	checks := []struct {
		name string
		run  func() error
	}{
		{"list", func() error { _, err := svc.ListRevisions(ctx, "config"); return err }},
		{"get", func() error { _, err := svc.GetRevision(ctx, ref); return err }},
		{"resolve uid", func() error { _, err := svc.ResolveRevision(ctx, ref); return err }},
		{"resolve named", func() error {
			_, err := svc.ResolveRevision(ctx, revision.Reference{Name: "config", RevisionName: "B"})
			return err
		}},
		{"resolve latest", func() error {
			_, err := svc.ResolveRevision(ctx, revision.Reference{Name: "config", Selector: revision.SelectorLatest})
			return err
		}},
		{"promote", func() error {
			_, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: b.Record.Metadata.UID, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			// When the service accesses the missing committed record.
			err := tc.run()
			// Then it reports operational corruption, not user not-found.
			if !errors.Is(err, ErrRevisionCorrupt) || errors.Is(err, revision.ErrRevisionNotFound) {
				t.Errorf("got %v, want only ErrRevisionCorrupt", err)
			}
		})
	}
	// Series pointers remain readable and unchanged even in corrupt storage.
	after, afterETag, err := svc.GetSeries(ctx, "config")
	if err != nil || after != before || afterETag != etag {
		t.Fatalf("series changed: before=%#v after=%#v etag=%q error=%v", before, after, afterETag, err)
	}
	for _, condition := range []string{"", `"stale"`} {
		result, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: b.Record.Metadata.UID, IfMatch: condition, Capabilities: Capabilities{Promote: true}})
		var precondition *revision.PreconditionError
		if !errors.As(err, &precondition) || precondition.CurrentETag != etag || errors.Is(err, ErrRevisionCorrupt) || result.ETag != etag {
			t.Errorf("condition %q result %#v error %v", condition, result, err)
		}
	}
	final, finalETag, err := svc.GetSeries(ctx, "config")
	if err != nil || final != before || finalETag != etag {
		t.Fatalf("series changed after precondition failures: %#v %q %v", final, finalETag, err)
	}
}

func TestRevisionServiceHealthyNotFoundRetiredAndCancellation(t *testing.T) {
	// Given two healthy series and a retired revision.
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
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "C", "/c", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateSeries(ctx, revisionRequest("other", "A", "/x", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, otherETag, err := svc.GetSeries(ctx, "other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.RetireBootConfigRevision(ctx, "config", b.Record.Metadata.UID); err != nil {
		t.Fatal(err)
	}
	_, etag, err := svc.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	checks := []struct {
		name string
		want error
		run  func() error
	}{
		{"unknown uid get", revision.ErrRevisionNotFound, func() error {
			_, err := svc.GetRevision(ctx, revision.Reference{Name: "config", UID: revision.UIDExample})
			return err
		}},
		{"wrong series get", revision.ErrRevisionNotFound, func() error {
			_, err := svc.GetRevision(ctx, revision.Reference{Name: "other", UID: a.Record.Metadata.UID})
			return err
		}},
		{"wrong series resolve", revision.ErrRevisionNotFound, func() error {
			_, err := svc.ResolveRevision(ctx, revision.Reference{Name: "other", UID: a.Record.Metadata.UID})
			return err
		}},
		{"wrong series promote", revision.ErrRevisionNotFound, func() error {
			_, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "other", UID: a.Record.Metadata.UID, IfMatch: otherETag, Capabilities: Capabilities{Promote: true}})
			return err
		}},
		{"unknown name resolve", revision.ErrRevisionNotFound, func() error {
			_, err := svc.ResolveRevision(ctx, revision.Reference{Name: "config", RevisionName: "missing"})
			return err
		}},
		{"unknown uid resolve", revision.ErrRevisionNotFound, func() error {
			_, err := svc.ResolveRevision(ctx, revision.Reference{Name: "config", UID: revision.UIDExample})
			return err
		}},
		{"unknown uid promote", revision.ErrRevisionNotFound, func() error {
			_, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: revision.UIDExample, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
			return err
		}},
		{"retired get", revision.ErrRevisionRetired, func() error {
			_, err := svc.GetRevision(ctx, revision.Reference{Name: "config", UID: b.Record.Metadata.UID})
			return err
		}},
		{"retired resolve", revision.ErrRevisionRetired, func() error {
			_, err := svc.ResolveRevision(ctx, revision.Reference{Name: "config", RevisionName: "B"})
			return err
		}},
		{"retired promote", revision.ErrRevisionRetired, func() error {
			_, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: b.Record.Metadata.UID, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
			return err
		}},
		{"cancelled list", context.Canceled, func() error { _, err := svc.ListRevisions(cancelled, "config"); return err }},
		{"cancelled get", context.Canceled, func() error {
			_, err := svc.GetRevision(cancelled, revision.Reference{Name: "config", UID: a.Record.Metadata.UID})
			return err
		}},
		{"cancelled resolve", context.Canceled, func() error { _, err := svc.ResolveRevision(cancelled, revision.Reference{Name: "config"}); return err }},
		{"cancelled promote", context.Canceled, func() error {
			_, err := svc.PromoteDefault(cancelled, PromotionRequest{Name: "config", UID: a.Record.Metadata.UID, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			// When the service is called with the specified reference or context.
			err := tc.run()
			// Then the original native or context error remains distinguishable.
			if !errors.Is(err, tc.want) || errors.Is(err, ErrRevisionCorrupt) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRevisionServicePropagatesNativeReadIOError(t *testing.T) {
	// Given an unreadable committed record encountered by native listing.
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
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	before, etag, err := svc.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "revisions", "bootconfig", "records", a.Record.Metadata.UID+".json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		run  func() error
	}{
		{"list", func() error { _, err := svc.ListRevisions(ctx, "config"); return err }},
		{"get unknown", func() error {
			_, err := svc.GetRevision(ctx, revision.Reference{Name: "config", UID: revision.UIDExample})
			return err
		}},
		{"resolve unknown", func() error {
			_, err := svc.ResolveRevision(ctx, revision.Reference{Name: "config", RevisionName: "missing"})
			return err
		}},
		{"promote unknown", func() error {
			_, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: revision.UIDExample, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			// When an operation reaches the native list read.
			err := tc.run()
			// Then the filesystem error remains available to callers.
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || errors.Is(err, ErrRevisionCorrupt) || errors.Is(err, revision.ErrRevisionNotFound) {
				t.Errorf("got %v, want native filesystem error", err)
			}
		})
	}
	after, afterETag, err := svc.GetSeries(ctx, "config")
	if err != nil || after != before || afterETag != etag {
		t.Fatalf("series changed: before=%#v after=%#v etag=%q error=%v", before, after, afterETag, err)
	}
}
