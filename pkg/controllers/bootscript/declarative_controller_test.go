// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/boot-service/pkg/clients/nodeidentity"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

type deliveryLegacy struct{ profile string }

func (l *deliveryLegacy) GenerateBootScript(_ context.Context, _, profile string) (string, error) {
	l.profile = profile
	return "legacy-script", nil
}

type deliverySecretError struct{ detail string }

func (e deliverySecretError) Error() string { return e.detail }

func TestDeclarativeDeliveryFailureFormattingRedactsCause(t *testing.T) {
	failure := deliveryFailure(deliverySecretError{detail: "sensitive-provider-detail"})
	var typed *DeliveryFailure
	if !errors.As(failure, &typed) {
		t.Fatal("expected delivery failure")
	}
	for _, value := range []string{fmt.Sprintf("%v", failure), fmt.Sprintf("%#v", typed), fmt.Sprintf("%#v", *typed)} {
		if strings.Contains(value, "sensitive-provider-detail") {
			t.Fatalf("cause leaked: %q", value)
		}
	}
}

type deliveryProvider struct {
	node  *v1.Node
	err   error
	calls int
}

func (p *deliveryProvider) ResolveNodeByIdentifier(context.Context, string) (*v1.Node, error) {
	p.calls++
	return p.node, p.err
}

func TestDeclarativeDeliveryAndProfile(t *testing.T) {
	ctx := context.Background()
	if err := storage.InitFileBackend(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	manager, err := declarativeboot.NewManager(ctx, declarativeboot.GeneratedStore(), declarativeboot.NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	svc := declarativeboot.NewRevisionService(manager)
	_, err = svc.CreateSeries(ctx, declarativeboot.ProposalRequest{Name: "os", RevisionName: "A", Resource: v1.BootConfig{Spec: v1.BootConfigSpec{Kernel: "/kernel-a", Initrd: "/initrd-a", Params: "alpha=1"}}, Capabilities: declarativeboot.Capabilities{Propose: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "canary"}, Spec: v1.BootAssignmentSpec{Profile: "canary", ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	legacy := &deliveryLegacy{}
	provider := &deliveryProvider{node: &v1.Node{Spec: v1.NodeSpec{XName: "x0c0s0b0n0", BootMAC: "aa:bb:cc:dd:ee:ff"}}}
	controller := NewDeclarativeController(DeclarativeOptions{Legacy: legacy, Nodes: provider, Revisions: svc})
	script, err := controller.GenerateBootScript(WithRequestProfile(ctx, "canary"), "node", "")
	if err != nil || !strings.Contains(script, "/kernel-a") || !strings.Contains(script, "/initrd-a") || !strings.Contains(script, "alpha=1 BOOTIF=01-aa-bb-cc-dd-ee-ff") {
		t.Fatalf("selected script %q, %v", script, err)
	}
	script, err = controller.GenerateBootScript(WithRequestProfile(ctx, "other"), "node", "original")
	if err != nil || script != "legacy-script" || legacy.profile != "original" {
		t.Fatalf("legacy %q profile %q err %v", script, legacy.profile, err)
	}
	provider.err = fmt.Errorf("successful inventory: %w", nodeidentity.ErrNotFound)
	script, err = controller.GenerateBootScript(ctx, "unknown", "original")
	if err != nil || script != "legacy-script" || legacy.profile != "original" {
		t.Fatalf("authoritative unknown node did not preserve legacy: %q, %v", script, err)
	}
	provider.err = errors.New("unavailable")
	_, err = controller.GenerateBootScript(ctx, "node", "")
	var failure *DeliveryFailure
	if !errors.As(err, &failure) {
		t.Fatalf("expected typed refusal: %v", err)
	}
}

func TestDeclarativeEmptyStoreDoesNotConsultProvider(t *testing.T) {
	ctx := context.Background()
	if err := storage.InitFileBackend(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	m, err := declarativeboot.NewManager(ctx, declarativeboot.GeneratedStore(), declarativeboot.NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	p := &deliveryProvider{err: errors.New("offline")}
	c := NewDeclarativeController(DeclarativeOptions{Legacy: &deliveryLegacy{}, Nodes: p, Revisions: declarativeboot.NewRevisionService(m)})
	script, err := c.GenerateBootScript(ctx, "node", "original")
	if err != nil || script != "legacy-script" || p.calls != 0 {
		t.Fatalf("script %q, calls %d, err %v", script, p.calls, err)
	}
}
