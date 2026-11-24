// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package hsm

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openchami/boot-service/pkg/clients/nodeidentity"
)

func TestDeclarativeUnknownNodeRequiresCompleteInventory(t *testing.T) {
	for _, tc := range []struct {
		name, components string
		status           int
		unknown          bool
	}{
		{"confirmed absent", `{"Components":[]}`, 200, true},
		{"missing field", `{}`, 200, false},
		{"missing NID", `{"Components":[{"ID":"x0c0s0b0n0","Type":"Node"}]}`, 200, false},
		{"unavailable", `{}`, 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/hsm/v2/State/Components" {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.components)
					return
				}
				_, _ = io.WriteString(w, `[]`)
			}))
			t.Cleanup(server.Close)
			config := DefaultHSMConfig()
			config.BaseURL = server.URL
			client, err := NewHSMClient(config, log.New(io.Discard, "", 0))
			if err != nil {
				t.Fatal(err)
			}
			_, err = NewDeclarativeNodeProvider(client).ResolveNodeByIdentifier(context.Background(), "x0c0s0b0n0")
			if err == nil || errors.Is(err, nodeidentity.ErrNotFound) != tc.unknown {
				t.Fatalf("lookup error = %v, want confirmed absent %v", err, tc.unknown)
			}
		})
	}
}
