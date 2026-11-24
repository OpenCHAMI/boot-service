// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/client"
)

func TestDeclarativeLocalProviderReadsExistingNodeSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nodes" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewEncoder(w).Encode([]v1.Node{{Spec: v1.NodeSpec{XName: "x0c0s0b0n0", NID: 42, Groups: []string{"compute"}}}}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	c, err := client.NewClient(server.URL, server.Client(), client.DefaultLogger())
	if err != nil {
		t.Fatal(err)
	}
	node, err := NewDeclarativeLocalNodeProvider(*c).ResolveNodeByIdentifier(context.Background(), "42")
	if err != nil || node.Spec.XName != "x0c0s0b0n0" || len(node.Spec.Groups) != 1 || node.Spec.Groups[0] != "compute" {
		t.Fatalf("node=%#v err=%v", node, err)
	}
}
