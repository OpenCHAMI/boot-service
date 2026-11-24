// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"fmt"
	"sync"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestSelectionSnapshotCoherentDuringAssignmentChanges(t *testing.T) {
	ctx := context.Background()
	svc, manager := revisionServiceFixture(t)
	caps := Capabilities{Propose: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("os", "A", "/kernel-a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("os", "B", "/kernel-b", caps))
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "all"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os", UID: a.Record.Metadata.UID}}})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		current := created
		for i := 0; i < 80; i++ {
			id := a.Record.Metadata.UID
			if i%2 == 0 {
				id = b.Record.Metadata.UID
			}
			input := current.Assignment
			input.Spec.ConfigRef.UID = id
			var updateErr error
			current, updateErr = manager.PutByUID(ctx, input.Metadata.UID, input, Conditions{IfMatch: []string{current.ETag}})
			if updateErr != nil {
				errs <- updateErr
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 80; i++ {
			selected, matched, selectErr := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
			if selectErr != nil || !matched {
				errs <- fmt.Errorf("selection: %v", selectErr)
				return
			}
			id, kernel := selected.Assignment.Assignment.Spec.ConfigRef.UID, selected.Content.Kernel
			if selected.RevisionUID != id || !((id == a.Record.Metadata.UID && kernel == "/kernel-a") || (id == b.Record.Metadata.UID && kernel == "/kernel-b")) {
				errs <- fmt.Errorf("mixed selection: uid=%s resolved=%s kernel=%s", id, selected.RevisionUID, kernel)
				return
			}
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
