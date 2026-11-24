// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/controllers/bootscript"
	"github.com/openchami/fabrica/pkg/resource"
)

func TestLegacySelectionRemainsAdditive(t *testing.T) {
	node := v1.Node{Spec: v1.NodeSpec{XName: "x0c0s0b0n1", BootMAC: "aa:bb:cc:dd:ee:ff", Groups: []string{"a", "b", "c", "d", "e"}}}
	configs := []v1.BootConfiguration{
		{Metadata: resource.Metadata{Name: "mac"}, Spec: v1.BootConfigurationSpec{MACs: []string{node.Spec.BootMAC}, Kernel: "/mac-kernel", Priority: 100}},
		{Metadata: resource.Metadata{Name: "groups"}, Spec: v1.BootConfigurationSpec{Groups: node.Spec.Groups, Kernel: "/groups-kernel", Priority: 0}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/nodes":
			if err := json.NewEncoder(w).Encode([]v1.Node{node}); err != nil {
				t.Error(err)
			}
		case "/bootconfigurations":
			if err := json.NewEncoder(w).Encode(configs); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	bootClient, err := client.NewClient(server.URL, server.Client(), client.DefaultLogger())
	if err != nil {
		t.Fatal(err)
	}
	controller := bootscript.NewBootScriptController(*bootClient, log.New(io.Discard, "", 0))
	script, err := controller.GenerateBootScript(context.Background(), node.Spec.XName, "")
	if err != nil || !strings.Contains(script, "/groups-kernel") || strings.Contains(script, "/mac-kernel") {
		t.Fatalf("legacy additive winner not rendered: script=%q err=%v", script, err)
	}
}
