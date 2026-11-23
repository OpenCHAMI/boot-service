// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import "strings"

// Conditions holds the raw conditional header values, preserving duplicates.
type Conditions struct{ IfMatch, IfNoneMatch []string }

// Validate rejects ambiguous conditional headers, including on collection POST.
func (c Conditions) Validate() error {
	if len(c.IfMatch) > 1 || len(c.IfNoneMatch) > 1 || len(c.IfMatch) > 0 && len(c.IfNoneMatch) > 0 {
		return ErrBadCondition
	}
	if len(c.IfMatch) == 1 && strings.Contains(c.IfMatch[0], ",") || len(c.IfNoneMatch) == 1 && strings.Contains(c.IfNoneMatch[0], ",") {
		return ErrBadCondition
	}
	return nil
}

func (c Conditions) check(current string, exists bool) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if !exists {
		if len(c.IfNoneMatch) != 1 || strings.TrimSpace(c.IfNoneMatch[0]) != "*" {
			return ErrPrecondition
		}
		return nil
	}
	if len(c.IfMatch) != 1 || strings.TrimSpace(c.IfMatch[0]) != current {
		return ErrPrecondition
	}
	return nil
}
