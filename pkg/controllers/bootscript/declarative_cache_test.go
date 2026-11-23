// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"
	"errors"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

type cacheRevisionReader struct {
	content v1.BootConfigSpec
	err     error
	uid     string
}

func (*cacheRevisionReader) HasAssignments(context.Context) (bool, error) { return true, nil }
func (r *cacheRevisionReader) SelectForDelivery(context.Context, v1.NodeSpec, string) (v1.BootConfigSpec, v1.BootAssignment, revision.Metadata, bool, error) {
	return r.content, v1.BootAssignment{Metadata: resource.Metadata{UID: "assignment"}}, revision.Metadata{UID: r.uid, Digest: r.uid}, true, r.err
}

func TestDeclarativeCacheFreshnessAndRefusal(t *testing.T) {
	ctx := context.Background()
	reader := &cacheRevisionReader{content: v1.BootConfigSpec{Kernel: "/kernel-a"}, uid: "a"}
	nodes := &deliveryProvider{node: &v1.Node{Spec: v1.NodeSpec{XName: "x0c0s0b0n0", BootMAC: "aa:bb:cc:dd:ee:ff"}}}
	c := NewDeclarativeController(DeclarativeOptions{Legacy: &deliveryLegacy{}, Nodes: nodes, Revisions: reader})
	generate := func(want string) {
		t.Helper()
		script, err := c.GenerateBootScript(ctx, "node", "")
		if err != nil || !strings.Contains(script, want) {
			t.Fatalf("script %q: %v", script, err)
		}
	}
	generate("/kernel-a")
	generate("/kernel-a")
	if c.CacheHits() != 1 {
		t.Fatal("warm script did not hit the cache")
	}
	reader.content.Kernel, reader.uid = "/kernel-b", "b"
	generate("/kernel-b")
	if c.CacheHits() != 1 {
		t.Fatal("promotion reused a stale cache entry")
	}
	nodes.node.Spec.BootMAC = "aa:bb:cc:dd:ee:01"
	generate("BOOTIF=01-aa-bb-cc-dd-ee-01")
	if c.CacheHits() != 1 {
		t.Fatal("identity change reused a stale cache entry")
	}
	reader.err = errors.New("broken selected reference")
	if script, err := c.GenerateBootScript(ctx, "node", ""); err == nil || script != "" {
		t.Fatal("broken reference served cached content")
	}
	reader.err, nodes.err = nil, errors.New("SMD unavailable")
	if script, err := c.GenerateBootScript(ctx, "node", ""); err == nil || script != "" {
		t.Fatal("failed identity refresh served cached content")
	}
}
