// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
//
// This file contains the user-editable OpenAPI extension hook.
//
// ✅ This file is safe to edit: it will NOT be overwritten by regeneration.
//
// Add any routes that are not Fabrica-generated (legacy APIs, custom endpoints,
// WireGuard, cloud-init, etc.) to registerCustomOpenAPIPaths so they appear in
// the served OpenAPI spec and Swagger UI at /openapi.json and /docs.
//
// Example:
//
//	func registerCustomOpenAPIPaths(spec *openapi3.T) {
//	    metaDataOp := openapi3.NewOperation()
//	    metaDataOp.OperationID = "getMetaData"
//	    metaDataOp.Summary = "Cloud-init meta-data endpoint"
//	    metaDataOp.Tags = []string{"cloud-init"}
//	    metaDataOp.Responses = openapi3.NewResponses()
//	    metaDataOp.Responses.Set("200", &openapi3.ResponseRef{
//	        Value: openapi3.NewResponse().WithDescription("YAML metadata for the requesting node"),
//	    })
//	    spec.Paths.Set("/meta-data", &openapi3.PathItem{Get: metaDataOp})
//	}
package main

import "github.com/getkin/kin-openapi/openapi3"

// registerCustomOpenAPIPaths is called by GenerateOpenAPISpec after all
// Fabrica-generated resource paths have been registered.
// Add your custom / non-generated route definitions here.
func registerCustomOpenAPIPaths(spec *openapi3.T) {
	// Boot service owns /service/status through the boot API handler. Preserve
	// its existing response contract instead of documenting Fabrica's new
	// standardized status handler, which this server does not register.
	schema := openapi3.NewObjectSchema().
		WithProperty("service_name", openapi3.NewStringSchema()).
		WithProperty("service_version", openapi3.NewStringSchema()).
		WithProperty("service_status", openapi3.NewStringSchema()).
		WithProperty("details", openapi3.NewObjectSchema().WithAdditionalProperties(openapi3.NewStringSchema())).
		WithRequired([]string{"service_name", "service_version", "service_status"})
	spec.Components.Schemas["BootServiceStatusResponse"] = &openapi3.SchemaRef{Value: schema}
	op := openapi3.NewOperation()
	op.OperationID = "getServiceStatus"
	op.Summary = "Get boot service status"
	op.Tags = []string{"Service"}
	op.Responses = openapi3.NewResponses()
	op.Responses.Set("200", &openapi3.ResponseRef{
		Value: openapi3.NewResponse().WithDescription("Boot service status").
			WithJSONSchemaRef(componentSchemaRef(spec, "BootServiceStatusResponse")),
	})
	spec.Paths.Set("/service/status", &openapi3.PathItem{Get: op})
}
