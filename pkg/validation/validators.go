// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package validation

import (
	"net"
	"net/url"
	"regexp"
	"strings"
)

// ValidateXName validates Cray XName format (e.g., x1000c0s0b0n0)
func ValidateXName(xname string) bool {
	if xname == "" {
		return false
	}

	// Basic XName pattern: x<cab>c<chassis>s<slot>b<blade>n<node>
	pattern := `^x\d+c\d+s\d+b\d+n\d+$`
	matched, _ := regexp.MatchString(pattern, xname)
	return matched
}

// ValidateXNameOrDefault validates XName format or allows wildcards and defaults
func ValidateXNameOrDefault(xname string) bool {
	if xname == "" {
		return true // Optional field
	}

	// Allow wildcards
	if strings.Contains(xname, "*") {
		// Basic pattern with wildcards
		pattern := `^x\d+c\d+s\d+b\d+n\*$|^x\d+c\d+s\d+b\*$|^x\d+c\d+s\*$|^x\d+c\*$|^x\*$`
		matched, _ := regexp.MatchString(pattern, xname)
		return matched
	}

	// Allow "default" keyword
	if xname == "default" {
		return true
	}

	// Standard XName validation
	return ValidateXName(xname)
}

// ValidateMAC validates MAC address format
func ValidateMAC(mac string) bool {
	if mac == "" {
		return true // Optional field
	}

	_, err := net.ParseMAC(mac)
	return err == nil
}

// ValidateURLOrPath validates URL format or file path
func ValidateURLOrPath(value string) bool {
	if value == "" {
		return false // Required field should not be empty
	}

	// Check if it's a valid URL
	if parsedURL, err := url.Parse(value); err == nil {
		// Must have scheme (http/https) for URLs
		if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
			return true
		}
	}

	// Check if it's a valid absolute file path
	if strings.HasPrefix(value, "/") {
		return len(value) > 1 // More than just "/"
	}

	return false
}

// ValidateURLOrPathOptional validates URL format or file path, allowing empty values
func ValidateURLOrPathOptional(value string) bool {
	if value == "" {
		return true // Optional field can be empty
	}

	return ValidateURLOrPath(value)
}
