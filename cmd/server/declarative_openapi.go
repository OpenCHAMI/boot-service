// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func registerDeclarativeOpenAPI(spec *openapi3.T) {
	item := spec.Paths.Value("/bootassignments/{uid}")
	if item != nil {
		copyOperation := func(source *openapi3.Operation, id string) *openapi3.Operation {
			if source == nil {
				return nil
			}
			copy := *source
			copy.OperationID = id
			return &copy
		}
		spec.Paths.Set("/bootassignments/by-name/{name}", &openapi3.PathItem{
			Parameters: []*openapi3.ParameterRef{{Value: openapi3.NewPathParameter("name").WithRequired(true).WithSchema(openapi3.NewStringSchema())}},
			Get:        copyOperation(item.Get, "getBootAssignmentByName"), Put: copyOperation(item.Put, "putBootAssignmentByName"),
			Patch: copyOperation(item.Patch, "patchBootAssignmentByName"), Delete: copyOperation(item.Delete, "deleteBootAssignmentByName"),
		})
	}
	for path, entry := range spec.Paths.Map() {
		if !declarativePath(path) {
			continue
		}
		for method, op := range entry.Operations() {
			scope := "promote:boot"
			if method == "GET" {
				scope = "read:boot"
			} else if (path == "/bootconfigs" || strings.HasPrefix(path, "/bootconfigs/")) && (method == "POST" || strings.Contains(path, "/revisions/{revisionName}")) {
				scope = "propose:boot"
			}
			if op.Extensions == nil {
				op.Extensions = map[string]any{}
			}
			op.Extensions["x-required-scope"] = scope
			if strings.HasPrefix(path, "/bootassignments/") && (method == "PUT" || method == "PATCH" || method == "DELETE") || strings.HasSuffix(path, "/default") || strings.HasSuffix(path, "/status") {
				found := false
				for _, parameter := range op.Parameters {
					if parameter.Value != nil && parameter.Value.In == "header" && strings.EqualFold(parameter.Value.Name, "If-Match") {
						found = true
					}
				}
				if !found {
					required := !strings.Contains(path, "/by-name/") || method != "PUT"
					op.Parameters = append(op.Parameters, &openapi3.ParameterRef{Value: openapi3.NewHeaderParameter("If-Match").WithRequired(required).WithSchema(openapi3.NewStringSchema())})
				}
				op.Responses.Set("412", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithDescription("Missing or stale precondition")})
			}
			for _, status := range []string{"401", "403", "503"} {
				op.Responses.Set(status, &openapi3.ResponseRef{Value: openapi3.NewResponse().WithDescription("Authentication, authorization or service unavailable")})
			}
			if strings.HasPrefix(path, "/bootassignments/") && method == "DELETE" {
				op.Responses.Delete("200")
				op.Responses.Set("204", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithDescription("Assignment deleted")})
			}
		}
	}
	if named := spec.Paths.Value("/bootassignments/by-name/{name}"); named != nil && named.Put != nil {
		named.Put.Parameters = append(named.Put.Parameters, &openapi3.ParameterRef{Value: openapi3.NewHeaderParameter("If-None-Match").WithSchema(openapi3.NewStringSchema())})
		named.Put.Description = "Create with If-None-Match: *; update with the current strong If-Match."
		named.Put.Responses.Set("201", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithDescription("Assignment created")})
	}
	if revisions := spec.Paths.Value("/bootconfigs/{name}/revisions/by-uid/{revisionUID}"); revisions != nil && revisions.Delete != nil {
		revisions.Delete.Description = "Retirement is disabled and always returns 409."
		revisions.Delete.Responses.Set("409", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithDescription("boot_config_retirement_disabled")})
	}
}
