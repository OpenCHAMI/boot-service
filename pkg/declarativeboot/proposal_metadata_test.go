// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/uid"
)

func TestRevisionCanonicalMetadataAndDigest(t *testing.T) {
	ctx := context.Background()
	svc, _ := revisionServiceFixture(t)
	req := revisionRequest("config", "A", "/a", Capabilities{Propose: true})
	req.Resource.APIVersion, req.Resource.Kind = "ignored", "ignored"
	req.Resource.Metadata.UID = "ignored"
	req.Resource.Metadata.CreatedAt = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	req.Resource.Metadata.Labels = map[string]string{"role": "test"}
	result, err := svc.CreateSeries(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	stored := result.Record.Resource
	parsed, err := (uid.Codec{}).Parse(stored.Metadata.UID)
	if err != nil || parsed.Prefix != "bootconfig" || stored.APIVersion != "v1" || stored.Kind != "BootConfig" || stored.Metadata.Name != "config" || stored.Metadata.CreatedAt.IsZero() {
		t.Fatalf("resource %#v UID error %v", stored, err)
	}
	expected := v1.BootConfig{APIVersion: "v1", Kind: "BootConfig", Spec: v1.BootConfigSpec{Kernel: "/a"}}
	expected.Metadata.Name = "config"
	expected.Metadata.Labels = map[string]string{"role": "test"}
	digest, err := storage.BootConfigRevisionDigest(&expected)
	if err != nil || digest != result.Record.Metadata.Digest {
		t.Fatalf("digest %q got %q error %v", digest, result.Record.Metadata.Digest, err)
	}
}

func TestLatestFollowerAllowsPromoterAndConflictWithoutMutation(t *testing.T) {
	ctx := context.Background()
	svc, manager := revisionServiceFixture(t)
	proposer := Capabilities{Propose: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", proposer))
	if err != nil {
		t.Fatal(err)
	}
	input := assignment("follower")
	input.Spec.ConfigRef = revision.Reference{Name: "config", Selector: revision.SelectorLatest}
	if _, err := manager.Create(ctx, input); err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "A", "/different", proposer))
	if !errors.Is(err, revision.ErrRevisionNameConflict) {
		t.Fatalf("existing name conflict: %v", err)
	}
	before, _, err := svc.GetSeries(ctx, "config")
	if err != nil || before.LatestRevisionUID != a.Record.Metadata.UID {
		t.Fatalf("before %#v: %v", before, err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", Capabilities{Propose: true, Promote: true}))
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := svc.GetSeries(ctx, "config")
	if err != nil || after.LatestRevisionUID != b.Record.Metadata.UID || after.DefaultRevisionUID != a.Record.Metadata.UID {
		t.Fatalf("after %#v: %v", after, err)
	}
}

func TestProposalWaitsForManagerGate(t *testing.T) {
	ctx := context.Background()
	svc, manager := revisionServiceFixture(t)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- manager.WithMutation(ctx, func(LockedView) error { close(entered); <-release; return nil })
	}()
	<-entered
	requestDone := make(chan error, 1)
	go func() {
		_, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", Capabilities{Propose: true}))
		requestDone <- err
	}()
	select {
	case err := <-requestDone:
		t.Fatalf("returned while gate held: %v", err)
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.GetBootConfigSeries(ctx, "config"); err != nil {
		t.Fatal(err)
	}
}
