// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"testing"

	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestNativeReferenceResolverSelectorsAndErrors(t *testing.T) {
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
	if _, err := storage.RetireBootConfigRevision(ctx, "config", b.Record.Metadata.UID); err != nil {
		t.Fatal(err)
	}
	resolver := NativeReferenceResolver{}
	tests := []struct {
		name    string
		ref     revision.Reference
		invalid bool
	}{
		{"default", revision.Reference{Name: "config"}, false},
		{"latest", revision.Reference{Name: "config", Selector: revision.SelectorLatest}, false},
		{"named", revision.Reference{Name: "config", RevisionName: "A"}, false},
		{"uid", revision.Reference{Name: "config", UID: a.Record.Metadata.UID}, false},
		{"missing series", revision.Reference{Name: "missing"}, true},
		{"missing name", revision.Reference{Name: "config", RevisionName: "missing"}, true},
		{"wrong series", revision.Reference{Name: "other", UID: a.Record.Metadata.UID}, true},
		{"retired", revision.Reference{Name: "config", UID: b.Record.Metadata.UID}, true},
		{"invalid name", revision.Reference{Name: "config", RevisionName: "bad/name"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := resolver.ResolveReference(ctx, tc.ref)
			if tc.invalid && !errors.Is(err, ErrInvalid) || !tc.invalid && err != nil {
				t.Fatalf("error: %v", err)
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := resolver.ResolveReference(cancelled, revision.Reference{Name: "config"}); !errors.Is(err, context.Canceled) || errors.Is(err, ErrInvalid) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestRevisionReadMethodsUseNativeState(t *testing.T) {
	ctx := context.Background()
	svc, _ := revisionServiceFixture(t)
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListRevisions(ctx, "config")
	if err != nil || len(list) != 1 || list[0].Metadata.UID != a.Record.Metadata.UID {
		t.Fatalf("list %#v: %v", list, err)
	}
	got, err := svc.GetRevision(ctx, revision.Reference{Name: "config", UID: a.Record.Metadata.UID})
	if err != nil || got.Metadata.UID != a.Record.Metadata.UID {
		t.Fatalf("get %#v: %v", got, err)
	}
	resolved, err := svc.ResolveRevision(ctx, revision.Reference{Name: "config", RevisionName: "A"})
	if err != nil || resolved.Metadata.UID != a.Record.Metadata.UID {
		t.Fatalf("resolve %#v: %v", resolved, err)
	}
}
