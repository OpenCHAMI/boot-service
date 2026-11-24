// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/go-chi/chi/v5"
)

// Never mount generated mutations for resources owned by the shared manager.
type legacyGeneratedRouter struct{ chi.Router }

func (r legacyGeneratedRouter) Route(pattern string, fn func(chi.Router)) chi.Router {
	if pattern == "/bootconfigs" || pattern == "/bootassignments" {
		return chi.NewRouter()
	}
	return r.Router.Route(pattern, fn)
}
