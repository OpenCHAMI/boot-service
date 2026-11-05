// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openchami/fabrica/pkg/versioning"

	"github.com/openchami/boot-service/pkg/client"
)

// internalAPIBaseURL is never dialed: handlerTransport serves every request in
// process. The generated client only needs it to build request URLs.
const internalAPIBaseURL = "http://boot-service.internal"

// newInternalAPIClient returns a client for the service's own resource API.
// Boot handlers and background sync reach resources through the generated
// client, and those callers have no caller identity to present, so requests are
// served in process instead of through the authenticated listener.
func newInternalAPIClient() (*client.Client, error) {
	router := chi.NewRouter()
	// Background workers have no outer Recoverer to catch a handler panic.
	router.Use(middleware.Recoverer)
	// Generated create handlers read the negotiated version from the context.
	router.Use(versioning.VersionNegotiationMiddleware(versioning.GlobalVersionRegistry, nil))
	RegisterGeneratedRoutes(router)

	return client.NewClient(internalAPIBaseURL,
		&http.Client{Transport: handlerTransport{handler: router}}, client.DefaultLogger())
}

// handlerTransport is an http.RoundTripper that serves requests with handler.
type handlerTransport struct {
	handler http.Handler
}

// RoundTrip implements http.RoundTripper.
func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		defer req.Body.Close() //nolint:errcheck
	}

	// RoundTrip must not modify the caller's request. Calls made while handling
	// a request inherit its context, and chi reuses any routing context it finds
	// there: the inner router would overwrite the outer request's route pattern
	// and leave state behind that fails the next call with a 404.
	inbound := req.Clone(context.WithValue(req.Context(), chi.RouteCtxKey, nil))
	// Handlers expect the non-nil body that a server guarantees.
	if inbound.Body == nil {
		inbound.Body = http.NoBody
	}

	recorded := &recordedResponse{header: make(http.Header)}
	t.handler.ServeHTTP(recorded, inbound)
	return recorded.response(req), nil
}

// recordedResponse is an http.ResponseWriter that buffers a handler's response.
type recordedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

// Header implements http.ResponseWriter.
func (r *recordedResponse) Header() http.Header {
	return r.header
}

// WriteHeader implements http.ResponseWriter. Only the first status counts.
func (r *recordedResponse) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

// Write implements http.ResponseWriter.
func (r *recordedResponse) Write(data []byte) (int, error) {
	r.WriteHeader(http.StatusOK)
	return r.body.Write(data)
}

func (r *recordedResponse) response(req *http.Request) *http.Response {
	r.WriteHeader(http.StatusOK) // A handler that wrote nothing returned 200.
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", r.status, http.StatusText(r.status)),
		StatusCode:    r.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        r.header,
		Body:          io.NopCloser(bytes.NewReader(r.body.Bytes())),
		ContentLength: int64(r.body.Len()),
		Request:       req,
	}
}
