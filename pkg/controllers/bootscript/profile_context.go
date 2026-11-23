// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

import "context"

type requestProfileKey struct{}

// WithRequestProfile carries the original HTTP query profile to declarative selection.
func WithRequestProfile(ctx context.Context, profile string) context.Context {
	return context.WithValue(ctx, requestProfileKey{}, profile)
}

// RequestProfile reports whether middleware supplied an HTTP query profile.
func RequestProfile(ctx context.Context) (string, bool) {
	profile, ok := ctx.Value(requestProfileKey{}).(string)
	return profile, ok
}
