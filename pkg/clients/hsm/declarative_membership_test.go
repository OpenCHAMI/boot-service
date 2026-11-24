// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package hsm

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestDeclarativeMembershipWireCompleteness(t *testing.T) {
	cases := []struct {
		name, membership string
		groups           []string
		wantError        bool
	}{
		{"missing", `{"id":"x0c0s0b0n0"}`, nil, true},
		{"null", `{"id":"x0c0s0b0n0","groupLabels":null}`, nil, true},
		{"wrong-type", `{"id":"x0c0s0b0n0","groupLabels":"compute"}`, nil, true},
		{"null-element", `{"id":"x0c0s0b0n0","groupLabels":[null]}`, nil, true},
		{"wrong-element", `{"id":"x0c0s0b0n0","groupLabels":[1]}`, nil, true},
		{"explicit-empty", `{"id":"x0c0s0b0n0","groupLabels":[]}`, []string{}, false},
		{"valid", `{"id":"x0c0s0b0n0","groupLabels":["compute","canary"]}`, []string{"compute", "canary"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := []byte("fixture-only-hmac-key")
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"aud": "smd"}).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				value := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				parsed, err := jwt.Parse(value, func(*jwt.Token) (interface{}, error) { return key, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithAudience("smd"))
				if err != nil || !parsed.Valid {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/hsm/v2/State/Components":
					_, _ = io.WriteString(w, `{"Components":[{"ID":"x0c0s0b0n0","Type":"Node","NID":42}]}`)
				case "/hsm/v2/Inventory/EthernetInterfaces":
					_, _ = io.WriteString(w, `[{"ComponentID":"x0c0s0b0n0","MACAddress":"aa:bb:cc:dd:ee:ff"}]`)
				case "/hsm/v2/memberships":
					if r.URL.Query().Get("type") != "Node" {
						http.Error(w, "invalid query", http.StatusBadRequest)
						return
					}
					_, _ = io.WriteString(w, "["+tc.membership+"]")
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			config := DefaultHSMConfig()
			config.BaseURL = server.URL
			config.AuthTokenProvider = func(context.Context) (string, error) { return token, nil }
			client, err := NewHSMClient(config, log.New(io.Discard, "", 0))
			if err != nil {
				t.Fatal(err)
			}
			node, err := NewDeclarativeNodeProvider(client).ResolveNodeByIdentifier(context.Background(), "42")
			if requests.Load() != 3 {
				t.Fatalf("wire requests = %d, want 3", requests.Load())
			}
			if tc.wantError {
				if err == nil || node != nil {
					t.Fatalf("incomplete membership accepted: %#v, %v", node, err)
				}
				return
			}
			if err != nil || node == nil {
				t.Fatalf("node unavailable: %v", err)
			}
			if len(tc.groups) == 0 && len(node.Spec.Groups) == 0 {
				return
			}
			if !reflect.DeepEqual(node.Spec.Groups, tc.groups) {
				t.Fatalf("groups = %v, want %v", node.Spec.Groups, tc.groups)
			}
		})
	}
}
