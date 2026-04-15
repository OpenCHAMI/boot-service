// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiv1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/fabrica/pkg/resource"
)

func TestGenerateBootScript_SelectsRequestedProfile(t *testing.T) {
	nodes := []apiv1.Node{
		{
			Spec: apiv1.NodeSpec{
				XName:    "x0c0s0b0n0",
				NID:      42,
				BootMAC:  "aa:bb:cc:dd:ee:ff",
				Hostname: "node-42",
			},
		},
	}

	configs := []apiv1.BootConfiguration{
		{
			Metadata: resource.Metadata{Name: "default-config"},
			Spec: apiv1.BootConfigurationSpec{
				Profile: "",
				Kernel:  "http://files.example.com/vmlinuz-default",
				Params:  "root=/dev/ram0 profile=default",
			},
		},
		{
			Metadata: resource.Metadata{Name: "compute-config"},
			Spec: apiv1.BootConfigurationSpec{
				Profile: "compute",
				Kernel:  "http://files.example.com/vmlinuz-compute",
				Params:  "root=/dev/ram0 profile=compute",
			},
		},
	}

	controller := newTestControllerWithData(t, nodes, configs)
	script, err := controller.GenerateBootScript(context.Background(), "x0c0s0b0n0", "compute")
	if err != nil {
		t.Fatalf("GenerateBootScript returned error: %v", err)
	}

	if !strings.Contains(script, "profile=compute") {
		t.Fatalf("expected compute profile script, got: %s", script)
	}
}

func TestGenerateBootScript_FallsBackToDefaultProfile(t *testing.T) {
	nodes := []apiv1.Node{
		{
			Spec: apiv1.NodeSpec{
				XName:    "x0c0s0b0n1",
				NID:      43,
				BootMAC:  "aa:bb:cc:dd:ee:01",
				Hostname: "node-43",
			},
		},
	}

	configs := []apiv1.BootConfiguration{
		{
			Metadata: resource.Metadata{Name: "service-config"},
			Spec: apiv1.BootConfigurationSpec{
				Profile: "service",
				Kernel:  "http://files.example.com/vmlinuz-service",
				Params:  "root=/dev/ram0 profile=service",
			},
		},
		{
			Metadata: resource.Metadata{Name: "default-config"},
			Spec: apiv1.BootConfigurationSpec{
				Profile: "",
				Kernel:  "http://files.example.com/vmlinuz-default",
				Params:  "root=/dev/ram0 profile=default",
			},
		},
	}

	controller := newTestControllerWithData(t, nodes, configs)
	script, err := controller.GenerateBootScript(context.Background(), "x0c0s0b0n1", "compute")
	if err != nil {
		t.Fatalf("GenerateBootScript returned error: %v", err)
	}

	if !strings.Contains(script, "profile=default") {
		t.Fatalf("expected fallback default profile script, got: %s", script)
	}
}

func TestGenerateBootScript_EmptyRequestedProfileUsesDefault(t *testing.T) {
	nodes := []apiv1.Node{
		{
			Spec: apiv1.NodeSpec{
				XName:    "x0c0s0b0n2",
				NID:      44,
				BootMAC:  "aa:bb:cc:dd:ee:02",
				Hostname: "node-44",
			},
		},
	}

	configs := []apiv1.BootConfiguration{
		{
			Metadata: resource.Metadata{Name: "compute-config"},
			Spec: apiv1.BootConfigurationSpec{
				Profile: "compute",
				Kernel:  "http://files.example.com/vmlinuz-compute",
				Params:  "root=/dev/ram0 profile=compute",
			},
		},
		{
			Metadata: resource.Metadata{Name: "default-config"},
			Spec: apiv1.BootConfigurationSpec{
				Profile: "",
				Kernel:  "http://files.example.com/vmlinuz-default",
				Params:  "root=/dev/ram0 profile=default",
			},
		},
	}

	controller := newTestControllerWithData(t, nodes, configs)
	script, err := controller.GenerateBootScript(context.Background(), "x0c0s0b0n2", "")
	if err != nil {
		t.Fatalf("GenerateBootScript returned error: %v", err)
	}

	if !strings.Contains(script, "profile=default") {
		t.Fatalf("expected default profile script for empty request profile, got: %s", script)
	}
}

func newTestControllerWithData(t *testing.T, nodes []apiv1.Node, configs []apiv1.BootConfiguration) *BootScriptController {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes":
			writeJSONResponse(t, w, nodes)
		case "/bootconfigurations":
			writeJSONResponse(t, w, configs)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	bootClient, err := client.NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	return NewBootScriptController(*bootClient, log.New(io.Discard, "", 0))
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, data interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		t.Fatalf("failed to encode JSON response: %v", err)
	}
}
