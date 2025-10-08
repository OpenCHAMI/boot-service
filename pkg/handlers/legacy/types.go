// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Legacy BSS API types for backward compatibility
// This file defines the exact request/response formats expected by legacy BSS clients
package legacy

import (
	"time"
)

// BootParameters represents the legacy BSS boot parameters format
type BootParameters struct {
	Hosts     []string        `json:"hosts,omitempty"`
	Macs      []string        `json:"macs,omitempty"`
	Nids      []string        `json:"nids,omitempty"`
	Params    string          `json:"params,omitempty"`
	Kernel    string          `json:"kernel,omitempty"`
	Initrd    string          `json:"initrd,omitempty"`
	CloudInit CloudInitConfig `json:"cloud-init,omitempty"`
	Meta      MetaData        `json:"meta,omitempty"`
}

// CloudInitConfig represents cloud-init configuration in legacy format
type CloudInitConfig struct {
	MetaData     interface{} `json:"meta-data,omitempty"`
	UserData     interface{} `json:"user-data,omitempty"`
	VendorData   interface{} `json:"vendor-data,omitempty"`
	NetworkData  interface{} `json:"network-data,omitempty"`
	PhoneHomeURL string      `json:"phone-home-url,omitempty"`
}

// MetaData represents metadata in legacy BSS format
type MetaData struct {
	Comment    string    `json:"comment,omitempty"`
	CreatedBy  string    `json:"created-by,omitempty"`
	CreatedAt  time.Time `json:"created-at,omitempty"`
	ModifiedBy string    `json:"modified-by,omitempty"`
	ModifiedAt time.Time `json:"modified-at,omitempty"`
}

// BootParametersRequest represents a request to create/update boot parameters
type BootParametersRequest struct {
	Hosts     []string        `json:"hosts,omitempty"`
	Macs      []string        `json:"macs,omitempty"`
	Nids      []string        `json:"nids,omitempty"`
	Params    string          `json:"params,omitempty"`
	Kernel    string          `json:"kernel,omitempty"`
	Initrd    string          `json:"initrd,omitempty"`
	CloudInit CloudInitConfig `json:"cloud-init,omitempty"`
}

// BootParametersResponse represents the response from boot parameters operations
type BootParametersResponse struct {
	BootParameters []BootParameters `json:"boot-parameters"`
}

// BootScriptRequest represents a request for boot script generation
type BootScriptRequest struct {
	// Node identifiers - at least one must be provided
	Host string `json:"host,omitempty"`
	Mac  string `json:"mac,omitempty"`
	Nid  string `json:"nid,omitempty"`

	// Optional parameters
	Retry  bool   `json:"retry,omitempty"`
	Token  string `json:"token,omitempty"`
	Format string `json:"format,omitempty"` // defaults to "ipxe"
}

// ServiceStatus represents the legacy service status format
type ServiceStatus struct {
	ServiceName    string            `json:"service_name"`
	ServiceVersion string            `json:"service_version"`
	ServiceStatus  string            `json:"service_status"`
	Details        map[string]string `json:"details,omitempty"`
}

// ServiceVersion represents the legacy service version format
type ServiceVersion struct {
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	BuildDate      string `json:"build_date,omitempty"`
	GitCommit      string `json:"git_commit,omitempty"`
}

// ErrorResponse represents the legacy error response format
type ErrorResponse struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Status   int    `json:"status"`
	Instance string `json:"instance,omitempty"`
}

// QueryParameters represents common query parameters for legacy API
type QueryParameters struct {
	Host string `json:"host,omitempty"`
	Mac  string `json:"mac,omitempty"`
	Nid  string `json:"nid,omitempty"`
	Name string `json:"name,omitempty"`
}
