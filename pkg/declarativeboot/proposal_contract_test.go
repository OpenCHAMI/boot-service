// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"sync"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func revisionServiceFixture(t *testing.T) (*RevisionService, *Manager) {
	t.Helper()
	if err := storage.InitFileBackend(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(context.Background(), GeneratedStore(), NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	return NewRevisionService(manager), manager
}

func revisionRequest(name, revisionName, kernel string, caps Capabilities) ProposalRequest {
	return ProposalRequest{Name: name, RevisionName: revisionName, Resource: v1.BootConfig{Spec: v1.BootConfigSpec{Kernel: kernel}}, Capabilities: caps}
}

func TestRevisionLifecycleAndIndependentPermissions(t *testing.T) {
	ctx := context.Background()
	svc, _ := revisionServiceFixture(t)
	proposer := Capabilities{Propose: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", proposer))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", proposer))
	if err != nil || !b.Created {
		t.Fatalf("new revision: %#v, %v", b, err)
	}
	replay, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", proposer))
	if err != nil || replay.Created || replay.Record.Metadata.UID != b.Record.Metadata.UID {
		t.Fatalf("replay: %#v, %v", replay, err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "B", "/different", proposer))
	if !errors.Is(err, revision.ErrRevisionNameConflict) {
		t.Fatalf("conflict: %v", err)
	}
	status, etag, err := svc.GetSeries(ctx, "config")
	if err != nil || status.DefaultRevisionUID != a.Record.Metadata.UID || status.LatestRevisionUID != b.Record.Metadata.UID {
		t.Fatalf("pointers: %#v, %v", status, err)
	}
	_, err = svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: b.Record.Metadata.UID, IfMatch: etag, Capabilities: proposer})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("proposer promotion: %v", err)
	}
	result, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: b.Record.Metadata.UID, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
	if err != nil || !result.Changed || result.Status.DefaultRevisionUID != b.Record.Metadata.UID {
		t.Fatalf("promoter promotion: %#v, %v", result, err)
	}
}

func TestLatestFollowerBlocksNewProposalButNotReplay(t *testing.T) {
	ctx := context.Background()
	svc, manager := revisionServiceFixture(t)
	caps := Capabilities{Propose: true}
	_, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "follower"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "config", Selector: revision.SelectorLatest}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", caps))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("new binding without promote: %v", err)
	}
	replay, err := svc.EnsureRevision(ctx, revisionRequest("config", "A", "/a", caps))
	if err != nil || replay.Created {
		t.Fatalf("existing binding replay: %#v, %v", replay, err)
	}
	_, err = svc.CreateRevision(ctx, revisionRequest("config", "", "/b", caps))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("unnamed proposal without promote: %v", err)
	}
}

func TestRevisionPromotionConcurrentPreconditions(t *testing.T) {
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
	c, err := svc.EnsureRevision(ctx, revisionRequest("config", "C", "/c", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, etag, err := svc.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{b.Record.Metadata.UID, c.Record.Metadata.UID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.PromoteDefault(ctx, PromotionRequest{Name: "config", UID: id, IfMatch: etag, Capabilities: Capabilities{Promote: true}})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	wins, stale := 0, 0
	for err := range errs {
		if err == nil {
			wins++
		} else if errors.Is(err, revision.ErrPreconditionFailed) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("wins=%d stale=%d", wins, stale)
	}
}
