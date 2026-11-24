// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/openchami/fabrica/pkg/revision"
	"github.com/openchami/fabrica/pkg/storage"
)

func TestRetireDenialPreservesPinnedRevision(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	backend, err := storage.NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	refs := newReferenceFixture(t, dir)
	m, err := NewManager(ctx, NewBackendStore(backend), refs)
	if err != nil {
		t.Fatal(err)
	}
	before, err := refs.store.Resolve(ctx, revision.Reference{Name: "config", RevisionName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	series, seriesETag, err := refs.store.GetSeries(ctx, "config")
	if err != nil {
		t.Fatal(err)
	}
	a := assignment("pin")
	a.Spec.ConfigRef.UID = before.Metadata.UID
	created, err := m.Create(ctx, a)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 3)
	wg.Add(3)
	go func() { defer wg.Done(); errs <- DenyBootConfigRetirement() }()
	go func() { defer wg.Done(); _, e := m.GetByUID(ctx, created.Assignment.Metadata.UID); errs <- e }()
	go func() {
		defer wg.Done()
		concurrent := a
		concurrent.Metadata.Name = "concurrent-pin"
		_, e := m.Create(ctx, concurrent)
		errs <- e
	}()
	wg.Wait()
	close(errs)
	denials := 0
	for err := range errs {
		if errors.Is(err, ErrRetirementDisabled) {
			denials++
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if denials != 1 || ErrRetirementDisabled.Error() != "boot_config_retirement_disabled" {
		t.Fatal("incorrect retirement denial")
	}
	var typed RetirementDisabledError
	if !errors.As(DenyBootConfigRetirement(), &typed) {
		t.Fatal("denial must be typed")
	}
	after, err := refs.store.Resolve(ctx, a.Spec.ConfigRef)
	if err != nil || after.Metadata != before.Metadata {
		t.Fatalf("pin changed: %#v, %v", after.Metadata, err)
	}
	named, err := refs.store.Resolve(ctx, revision.Reference{Name: "config", RevisionName: "A"})
	if err != nil || named.Metadata.UID != before.Metadata.UID {
		t.Fatalf("name changed: %#v, %v", named.Metadata, err)
	}
	seriesAfter, etagAfter, err := refs.store.GetSeries(ctx, "config")
	if err != nil || seriesAfter != series || etagAfter != seriesETag {
		t.Fatalf("series changed: %#v, %q, %v", seriesAfter, etagAfter, err)
	}
	pinAfter, err := m.GetByUID(ctx, created.Assignment.Metadata.UID)
	if err != nil || pinAfter.ETag != created.ETag || pinAfter.Assignment.Spec.ConfigRef != a.Spec.ConfigRef {
		t.Fatalf("assignment pin changed: %#v, %v", pinAfter, err)
	}
}
