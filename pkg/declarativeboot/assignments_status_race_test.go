// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

func TestAssignmentStatusAndSpecRace(t *testing.T) {
	// Given one ETag shared by status and spec updates.
	ctx := context.Background()
	m, _ := testManager(t)
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	condition := Conditions{IfMatch: []string{created.ETag}}
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, err := m.PutStatusByUID(ctx, StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: condition}, v1.BootAssignmentStatus{Phase: "ready"})
		results <- err
	}()
	go func() {
		<-start
		next := assignment("target")
		next.Spec.Priority = 13
		_, err := m.PutByUID(ctx, created.Assignment.Metadata.UID, next, condition)
		results <- err
	}()
	// When both different writes race at the gate.
	close(start)
	// Then exactly one commits and the other sees a stale precondition.
	wins, stale := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			wins++
		} else if errors.Is(err, ErrPrecondition) {
			stale++
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("wins=%d stale=%d", wins, stale)
	}
}

func TestAssignmentStatusCancellationDuringTransform(t *testing.T) {
	// Given a valid status request and an operation cancelled inside its callback.
	m, backend := testManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	created, err := m.Create(ctx, assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	// When the callback cancels the context before returning a change.
	_, err = m.TransformStatusByUID(ctx, StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}, func(s v1.BootAssignmentStatus) (v1.BootAssignmentStatus, error) {
		cancel()
		s.Phase = "ready"
		return s, nil
	})
	// Then cancellation is returned and the committed file is unchanged.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	got, err := NewBackendStore(backend).Load(context.Background(), created.Assignment.Metadata.UID)
	if err != nil || got.Status.Phase != "" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestAssignmentStatusCancellationWaitingForGate(t *testing.T) {
	// Given a committed assignment and an occupied shared gate.
	m, _ := testManager(t)
	created, err := m.Create(context.Background(), assignment("target"))
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- m.WithMutation(context.Background(), func(LockedView) error { close(entered); <-release; return nil })
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// When a cancelled status update attempts to acquire the gate.
	_, err = m.PutStatusByUID(ctx, StatusTarget{UID: created.Assignment.Metadata.UID, Conditions: Conditions{IfMatch: []string{created.ETag}}}, v1.BootAssignmentStatus{Phase: "ready"})
	close(release)
	if gateErr := <-done; gateErr != nil {
		t.Fatal(gateErr)
	}
	// Then it returns cancellation and leaves the assignment unchanged.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	got, err := m.GetByUID(context.Background(), created.Assignment.Metadata.UID)
	if err != nil || got.ETag != created.ETag || got.Assignment.Status.Phase != "" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}
