// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import "errors"

// Capabilities must be supplied by the verified authentication boundary.
type Capabilities struct {
	Propose bool
	Promote bool
}

// ErrForbidden reports a missing verified capability.
var ErrForbidden = errors.New("boot revision operation forbidden")
