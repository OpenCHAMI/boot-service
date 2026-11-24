// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

// Package nodeidentity defines errors shared by authoritative node sources.
package nodeidentity

import "errors"

// ErrNotFound means a successful, complete inventory lookup found no node.
// Transport failures, incomplete identity, and ambiguous matches must not wrap it.
var ErrNotFound = errors.New("node not found")
