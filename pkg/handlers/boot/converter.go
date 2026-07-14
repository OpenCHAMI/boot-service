// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package boot

import (
	"strconv"
	"strings"
	"time"

	apiv1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

// ConvertNodeToLegacyIdentifiers extracts legacy identifiers from a modern Node resource
func ConvertNodeToLegacyIdentifiers(n *apiv1.Node) (hosts []string, macs []string, nids []string) {
	if n.Spec.XName != "" {
		hosts = append(hosts, n.Spec.XName)
	}

	if n.Spec.BootMAC != "" {
		macs = append(macs, n.Spec.BootMAC)
	}

	if n.Spec.NID != 0 {
		nids = append(nids, strconv.Itoa(int(n.Spec.NID)))
	}

	return hosts, macs, nids
}

// ConvertBootConfigurationToLegacy converts a modern BootConfiguration to legacy BootParameters
func ConvertBootConfigurationToLegacy(config *apiv1.BootConfiguration) BootParameters {
	// Extract target identifiers from boot configuration
	var hosts, macs, nids []string

	hosts = config.Spec.Hosts
	macs = config.Spec.MACs

	// Convert NIDs to strings
	for _, nid := range config.Spec.NIDs {
		nids = append(nids, strconv.Itoa(int(nid)))
	}

	// Include groups as hosts
	hosts = append(hosts, config.Spec.Groups...)

	// Create metadata from resource metadata
	meta := MetaData{
		Comment:    "Converted from modern BootConfiguration",
		CreatedAt:  config.Metadata.CreatedAt,
		ModifiedAt: config.Metadata.UpdatedAt,
	}

	return BootParameters{
		Hosts:     hosts,
		Macs:      macs,
		Nids:      nids,
		Params:    config.Spec.Params,
		Kernel:    config.Spec.Kernel,
		Initrd:    config.Spec.Initrd,
		CloudInit: CloudInitConfig{}, // Empty for now - will add if needed
		Meta:      meta,
	}
}

// ConvertLegacyToBootConfiguration converts legacy BootParameters to modern BootConfiguration
func ConvertLegacyToBootConfiguration(legacy BootParameters) *apiv1.BootConfiguration {
	// Convert string NIDs to int32
	var nids []int32
	for _, nidStr := range legacy.Nids {
		if nid, err := strconv.Atoi(nidStr); err == nil {
			nids = append(nids, int32(nid))
		}
	}

	return &apiv1.BootConfiguration{
		Spec: apiv1.BootConfigurationSpec{
			Hosts:  legacy.Hosts,
			MACs:   legacy.Macs,
			NIDs:   nids,
			Kernel: legacy.Kernel,
			Initrd: legacy.Initrd,
			Params: legacy.Params,
		},
	}
}

// ConvertLegacyRequestToBootConfiguration converts a legacy request to modern BootConfiguration
func ConvertLegacyRequestToBootConfiguration(req BootParametersRequest) *apiv1.BootConfiguration {
	// Convert string NIDs to int32
	var nids []int32
	for _, nidStr := range req.Nids {
		if nid, err := strconv.Atoi(nidStr); err == nil {
			nids = append(nids, int32(nid))
		}
	}

	return &apiv1.BootConfiguration{
		Spec: apiv1.BootConfigurationSpec{
			Hosts:  req.Hosts,
			MACs:   req.Macs,
			NIDs:   nids,
			Kernel: req.Kernel,
			Initrd: req.Initrd,
			Params: req.Params,
		},
	}
}

// ExtractNodeIdentifier extracts the best node identifier from a BootScriptRequest
func ExtractNodeIdentifier(req BootScriptRequest) string {
	// Prefer host (xname), then mac, then nid
	if req.Host != "" {
		return req.Host
	}
	if req.Mac != "" {
		return req.Mac
	}
	if req.Nid != "" {
		return req.Nid
	}
	return ""
}

// ParseNodeIdentifiersFromQuery parses legacy query parameters for node identifiers
func ParseNodeIdentifiersFromQuery(host, mac, nid, name string) []string {
	var identifiers []string

	if host != "" {
		// Handle comma-separated hosts
		identifiers = append(identifiers, strings.Split(host, ",")...)
	}
	if mac != "" {
		// Handle comma-separated macs
		identifiers = append(identifiers, strings.Split(mac, ",")...)
	}
	if nid != "" {
		// Handle comma-separated nids
		identifiers = append(identifiers, strings.Split(nid, ",")...)
	}
	if name != "" {
		// Handle comma-separated names
		identifiers = append(identifiers, strings.Split(name, ",")...)
	}

	return identifiers
}

// CreateErrorResponse creates a legacy-formatted error response
func CreateErrorResponse(status int, title, detail string) ErrorResponse {
	return ErrorResponse{
		Type:   "about:blank",
		Title:  title,
		Detail: detail,
		Status: status,
	}
}

// CreateServiceStatus creates a legacy service status response
func CreateServiceStatus(version string) ServiceStatus {
	return ServiceStatus{
		ServiceName:    "boot-script-service",
		ServiceVersion: version,
		ServiceStatus:  "running",
		Details: map[string]string{
			"framework": "fabrica",
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}
}

// CreateServiceVersion creates a legacy service version response
func CreateServiceVersion(version, buildDate, gitCommit string) ServiceVersion {
	return ServiceVersion{
		ServiceName:    "boot-script-service",
		ServiceVersion: version,
		BuildDate:      buildDate,
		GitCommit:      gitCommit,
	}
}
