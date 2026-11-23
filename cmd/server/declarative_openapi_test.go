// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"testing"
)

func TestDeclarativeOpenAPIAdvertisesOnlyAvailableOperations(t *testing.T) {
	spec := GenerateOpenAPISpec()
	named := spec.Paths.Value("/bootassignments/by-name/{name}")
	if named == nil || named.Get == nil || named.Put == nil || named.Delete == nil {
		t.Fatal("named assignment operations missing")
	}
	if named.Put.Extensions["x-required-scope"] != "promote:boot" {
		t.Fatalf("wrong named apply scope: %v", named.Put.Extensions)
	}
	for _, path := range []string{"/bootassignments/{uid}/status", "/bootconfigs/{name}/status"} {
		status := spec.Paths.Value(path)
		if status == nil || status.Put == nil || status.Patch == nil || status.Put.Responses.Value("412") == nil || status.Put.Extensions["x-required-scope"] != "promote:boot" {
			t.Fatalf("guarded conditional status missing: %s", path)
		}
	}
	retire := spec.Paths.Value("/bootconfigs/{name}/revisions/by-uid/{revisionUID}")
	if retire == nil || retire.Delete == nil || retire.Delete.Responses.Value("409") == nil {
		t.Fatal("retirement denial missing")
	}
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("OpenAPI validation: %v", err)
	}
}
