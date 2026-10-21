// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package boot

// BuildInfo identifies the boot-service binary, independently of its generator.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// HandlerOption configures a boot API handler.
type HandlerOption func(*Handler)

// WithBuildInfo supplies the binary's build metadata to service endpoints.
// Unset versions use dev; unavailable commit and date fields are omitted.
func WithBuildInfo(info BuildInfo) HandlerOption {
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "none" {
		info.Commit = ""
	}
	if info.Date == "unknown" {
		info.Date = ""
	}
	return func(h *Handler) {
		h.buildInfo = info
	}
}
