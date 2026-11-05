// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package boot

import (
	"net/http"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
)

func routesOf(t *testing.T, router chi.Router) []string {
	t.Helper()
	var routes []string
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(routes)
	return routes
}

// The split decides what stays reachable without a token, so pin it exactly.
func TestRouteVisibility(t *testing.T) {
	handler := &Handler{}
	tests := []struct {
		name     string
		register func(chi.Router)
		want     []string
	}{
		{
			name:     "modern public",
			register: handler.RegisterModernPublicRoutes,
			want:     []string{"GET /bootscript", "GET /service/version"},
		},
		{
			name:     "legacy public",
			register: handler.RegisterLegacyPublicRoutes,
			want:     []string{"GET /boot/v1/bootscript", "GET /boot/v1/service/status", "GET /boot/v1/service/version"},
		},
		{
			name:     "modern protected",
			register: handler.RegisterModernProtectedRoutes,
			want:     []string{"DELETE /bootparameters", "GET /bootparameters", "POST /bootparameters", "PUT /bootparameters"},
		},
		{
			name:     "legacy protected",
			register: handler.RegisterLegacyProtectedRoutes,
			want:     []string{"DELETE /boot/v1/bootparameters", "GET /boot/v1/bootparameters", "POST /boot/v1/bootparameters", "PUT /boot/v1/bootparameters"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			tc.register(router)
			got := routesOf(t, router)
			if len(got) != len(tc.want) {
				t.Fatalf("got routes %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got routes %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestPublicAndProtectedRoutesRegisterInSeparateGroups(t *testing.T) {
	// chi panics if /boot/v1 is mounted in both groups.
	handler := &Handler{}
	router := chi.NewRouter()
	router.Group(func(public chi.Router) {
		handler.RegisterModernPublicRoutes(public)
		handler.RegisterLegacyPublicRoutes(public)
	})
	router.Group(func(protected chi.Router) {
		handler.RegisterModernProtectedRoutes(protected)
		handler.RegisterLegacyProtectedRoutes(protected)
	})
}
