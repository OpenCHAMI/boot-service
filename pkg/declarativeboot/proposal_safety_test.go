// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

type permissiveRevisionResolver struct{}

func (permissiveRevisionResolver) ResolveReference(context.Context, revision.Reference) error {
	return nil
}

func TestProposalRepairOfMissingReferencesRequiresPromotion(t *testing.T) {
	ctx := context.Background()
	if err := storage.InitFileBackend(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ctx, GeneratedStore(), permissiveRevisionResolver{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "broken"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "config", RevisionName: "B"}}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err = NewManager(ctx, GeneratedStore(), NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewRevisionService(manager)
	proposer := Capabilities{Propose: true}
	_, err = svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", proposer))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing series repair: %v", err)
	}
	admin := Capabilities{Propose: true, Promote: true}
	_, err = svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", admin))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", proposer))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing named binding repair: %v", err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", admin))
	if err != nil {
		t.Fatal(err)
	}
}

func TestProposalMissingCommittedRecordFailsClosed(t *testing.T) {
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
	caps := Capabilities{Propose: true, Promote: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", caps))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", a.Record.Metadata.UID+".json")); err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "A", "/a", caps))
	if !errors.Is(err, ErrRevisionCorrupt) {
		t.Fatalf("missing record: %v", err)
	}
}

func TestLatestFollowerAndProposalRaceIsOrdered(t *testing.T) {
	ctx := context.Background()
	svc, manager := revisionServiceFixture(t)
	proposer := Capabilities{Propose: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", proposer))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var proposal ProposalResult
	var proposalErr, assignmentErr error
	go func() {
		defer wg.Done()
		<-start
		proposal, proposalErr = svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", proposer))
	}()
	go func() {
		defer wg.Done()
		<-start
		_, assignmentErr = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "follower"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "config", Selector: revision.SelectorLatest}}})
	}()
	close(start)
	wg.Wait()
	if assignmentErr != nil {
		t.Fatal(assignmentErr)
	}
	state, _, err := svc.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	if proposalErr == nil {
		if !proposal.Created || state.LatestRevisionUID != proposal.Record.Metadata.UID {
			t.Fatalf("proposal outcome %#v state %#v", proposal, state)
		}
	} else if !errors.Is(proposalErr, ErrForbidden) || state.LatestRevisionUID != a.Record.Metadata.UID {
		t.Fatalf("proposal error %v state %#v", proposalErr, state)
	}
}
