// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestRetireRevisionServicePreservesNativeRecordsAndPins(t *testing.T) {
	ctx := context.Background()
	svc, manager := revisionServiceFixture(t)
	a, err := svc.CreateSeries(ctx, revisionRequest("config", "A", "/a", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureRevision(ctx, revisionRequest("config", "B", "/b", Capabilities{Propose: true}))
	if err != nil {
		t.Fatal(err)
	}
	pin, err := manager.Create(ctx, v1.BootAssignment{
		Metadata: resource.Metadata{Name: "pin"},
		Spec:     v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "config", UID: a.Record.Metadata.UID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.ListRevisions(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	status, etag, err := svc.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}

	refs := []revision.Reference{
		{Name: "config", UID: a.Record.Metadata.UID},
		{Name: "config", RevisionName: "A"},
		{Name: "config", Selector: revision.SelectorDefault},
		{Name: "config", Selector: revision.SelectorLatest},
		{Name: "missing", UID: a.Record.Metadata.UID},
		{},
	}
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- svc.RetireRevision(ctx, refs[i%len(refs)])
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		var denial RetirementDisabledError
		if !errors.As(err, &denial) || !errors.Is(err, ErrRetirementDisabled) || err.Error() != "boot_config_retirement_disabled" {
			t.Fatalf("retirement was not denied: %v", err)
		}
	}
	after, err := svc.ListRevisions(ctx, "config")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("native records changed: %v", err)
	}
	statusAfter, etagAfter, err := svc.GetSeries(ctx, "config")
	if err != nil || statusAfter != status || etagAfter != etag {
		t.Fatalf("series pointers changed: %v", err)
	}
	stored, err := manager.GetByUID(ctx, pin.Assignment.Metadata.UID)
	if err != nil || stored.ETag != pin.ETag || stored.Assignment.Metadata.UID != pin.Assignment.Metadata.UID || stored.Assignment.Spec.ConfigRef != pin.Assignment.Spec.ConfigRef {
		t.Fatalf("assignment pin changed: %v", err)
	}
	resolved, err := svc.ResolveRevision(ctx, revision.Reference{Name: "config", RevisionName: "A"})
	if err != nil || resolved.Metadata.UID != a.Record.Metadata.UID {
		t.Fatalf("named revision no longer resolves: %v", err)
	}
}

func TestRetireRevisionNeverTouchesBackend(t *testing.T) {
	// A nil service cannot access a manager or native store; denial must still work.
	var svc *RevisionService
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := svc.RetireRevision(ctx, revision.Reference{Name: "config"})
	var denial RetirementDisabledError
	if !errors.As(fmt.Errorf("outer: %w", err), &denial) || !errors.Is(err, ErrRetirementDisabled) {
		t.Fatalf("expected unconditional typed denial: %v", err)
	}
}
