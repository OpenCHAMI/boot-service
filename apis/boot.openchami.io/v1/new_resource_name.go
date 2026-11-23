// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"errors"
	"regexp"
	"strings"
)

var newResourceNameLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)

// ValidateRevisionName validates a nonempty, case-sensitive revision name.
func ValidateRevisionName(name string) error {
	return validateNewResourceName(name)
}

func validateNewResourceName(name string) error {
	if len(name) == 0 || len(name) > 253 {
		return errors.New("name must contain 1 to 253 bytes")
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) > 63 || !newResourceNameLabel.MatchString(label) {
			return errors.New("name must use DNS-subdomain-style labels")
		}
	}
	return nil
}
