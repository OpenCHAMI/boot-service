// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"net/http"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
)

// NamedAssignmentCondition supplies the creation or update precondition.
type NamedAssignmentCondition struct {
	IfMatch     string
	IfNoneMatch string
}

// ApplyBootAssignmentByName applies one named assignment using the supplied condition.
func (c *Client) ApplyBootAssignmentByName(ctx context.Context, name string, input CreateBootAssignmentRequest, condition NamedAssignmentCondition) (*v1.BootAssignment, string, error) {
	segment, err := escapePathSegment(name)
	if err != nil {
		return nil, "", err
	}
	headers := http.Header{}
	if condition.IfMatch != "" {
		headers.Set("If-Match", condition.IfMatch)
	}
	if condition.IfNoneMatch != "" {
		headers.Set("If-None-Match", condition.IfNoneMatch)
	}
	var result v1.BootAssignment
	response, err := c.doRequestWithResponse(ctx, http.MethodPut, "/bootassignments/by-name/"+segment, input, headers, &result)
	if err != nil {
		return nil, "", err
	}
	return &result, response.Get("ETag"), nil
}

// GetBootAssignmentByName reads the resource and its strong ETag.
func (c *Client) GetBootAssignmentByName(ctx context.Context, name string) (*v1.BootAssignment, string, error) {
	segment, err := escapePathSegment(name)
	if err != nil {
		return nil, "", err
	}
	var result v1.BootAssignment
	response, err := c.doRequestWithResponse(ctx, http.MethodGet, "/bootassignments/by-name/"+segment, nil, nil, &result)
	if err != nil {
		return nil, "", err
	}
	return &result, response.Get("ETag"), nil
}

func (c *Client) DeleteBootAssignmentByName(ctx context.Context, name, ifMatch string) error {
	segment, err := escapePathSegment(name)
	if err != nil {
		return err
	}
	return c.doRequestWithHeaders(ctx, http.MethodDelete, "/bootassignments/by-name/"+segment, nil, http.Header{"If-Match": []string{ifMatch}}, nil)
}
