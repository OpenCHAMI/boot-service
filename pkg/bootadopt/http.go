// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootadopt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Requests records actual HTTP completion, including ambiguous transport errors.
type Requests struct {
	Started   uint64 `json:"started"`
	Completed uint64 `json:"completed"`
	Ambiguous bool   `json:"ambiguous"`
}

// Receipt allows callers to distinguish completed errors from pending writes.
type Receipt struct {
	Operation string   `json:"operation"`
	Phase     string   `json:"phase"`
	Requests  Requests `json:"requests"`
	Plan      *Plan    `json:"plan,omitempty"`
	Error     string   `json:"error,omitempty"`
}

type api struct {
	base    *url.URL
	token   string
	client  *http.Client
	receipt *Receipt
}

func (a *api) request(ctx context.Context, method, path string, body any, headers http.Header, output any) (int, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return 0, err
		}
	}
	u := a.base.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	for name, values := range headers {
		req.Header[name] = values
	}
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	a.receipt.Requests.Started++
	response, err := a.client.Do(req)
	if err != nil {
		a.receipt.Phase, a.receipt.Requests.Ambiguous = "transport", true
		return 0, fmt.Errorf("%s request failed; retry the same adoption to reconcile", method)
	}
	defer response.Body.Close()
	data, err = io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		a.receipt.Phase, a.receipt.Requests.Ambiguous = "transport", true
		return response.StatusCode, fmt.Errorf("incomplete %s response; retry the same adoption to reconcile", method)
	}
	a.receipt.Requests.Completed++
	a.receipt.Phase = "response"
	if response.StatusCode >= 400 {
		return response.StatusCode, fmt.Errorf("%s returned HTTP %d", method, response.StatusCode)
	}
	if output != nil && json.Unmarshal(data, output) != nil {
		return response.StatusCode, fmt.Errorf("invalid %s response", method)
	}
	return response.StatusCode, nil
}
