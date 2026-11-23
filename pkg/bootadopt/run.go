// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootadopt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"time"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

// desiredInput matches the management write contract; response-only kind,
// apiVersion, UID, timestamps, and status are deliberately excluded.
type desiredInput[T any] struct {
	Metadata desiredMetadata `json:"metadata"`
	Spec     T               `json:"spec"`
}
type desiredMetadata struct {
	Name string `json:"name"`
}

// Options identifies one explicit source and target. Execute defaults to false.
type Options struct {
	Server, Token, SourceUID, Series, Assignment string
	Execute                                      bool
	HTTPClient                                   *http.Client
}

// Run copies one source using idempotent named writes. A partial failure never
// deletes data; retry the same mapping. Redirects are disabled to contain JWTs.
func Run(ctx context.Context, options Options) (receipt Receipt, resultErr error) {
	receipt = Receipt{Operation: "boot-adopt", Phase: "preflight"}
	defer func() {
		if resultErr != nil {
			receipt.Error = resultErr.Error()
		}
	}()
	base, err := url.Parse(options.Server)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return receipt, fmt.Errorf("invalid server URL")
	}
	for _, value := range []string{options.SourceUID, options.Series, options.Assignment} {
		if value == "" || value == "." || value == ".." {
			return receipt, fmt.Errorf("explicit source UID, series and assignment are required")
		}
	}
	transport := http.Client{Timeout: 30 * time.Second}
	if options.HTTPClient != nil {
		transport = *options.HTTPClient
	}
	transport.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	a := api{base: base, token: options.Token, client: &transport, receipt: &receipt}
	var source v1.BootConfiguration
	if _, err := a.request(ctx, http.MethodGet, "/bootconfigurations/"+url.PathEscape(options.SourceUID), nil, nil, &source); err != nil {
		return receipt, err
	}
	if source.Metadata.UID != options.SourceUID {
		return receipt, fmt.Errorf("source UID mismatch")
	}
	receipt.Phase = "preflight"
	plan, err := BuildPlan(source, options.Series, options.Assignment)
	if err != nil {
		return receipt, err
	}
	receipt.Plan = &plan
	if !options.Execute {
		receipt.Phase = "complete"
		return receipt, nil
	}
	if err := a.execute(ctx, plan); err != nil {
		return receipt, err
	}
	receipt.Phase = "complete"
	return receipt, nil
}

func (a *api) execute(ctx context.Context, plan Plan) error {
	seriesPath := "/bootconfigs/" + url.PathEscape(plan.Config.Metadata.Name)
	var state revision.SeriesStatus
	status, err := a.request(ctx, http.MethodGet, seriesPath, nil, nil, &state)
	if err != nil && status != http.StatusNotFound {
		return err
	}
	configInput := desiredInput[v1.BootConfigSpec]{Metadata: desiredMetadata{Name: plan.Config.Metadata.Name}, Spec: plan.Config.Spec}
	var record revision.Record[*v1.BootConfig]
	if status == http.StatusNotFound {
		// Query construction is separate from path escaping: the named create is
		// needed so a lost response can be reconciled without a duplicate revision.
		u := *a.base
		u.RawQuery = "revisionName=" + url.QueryEscape(plan.RevisionName)
		original := a.base
		a.base = &u
		_, err = a.request(ctx, http.MethodPost, "/bootconfigs", configInput, nil, &record)
		a.base = original
		if err != nil {
			return err
		}
	} else {
		_, err = a.request(ctx, http.MethodPut, seriesPath+"/revisions/"+url.PathEscape(plan.RevisionName), configInput, nil, &record)
		if err != nil {
			return err
		}
	}
	if record.Metadata.UID == "" || record.Resource == nil || record.Resource.Spec != plan.Config.Spec || record.Metadata.RevisionName != plan.RevisionName {
		return fmt.Errorf("revision response does not match adoption plan")
	}
	assignment := plan.Assignment
	assignment.Spec.ConfigRef = revision.Reference{Name: plan.Config.Metadata.Name, UID: record.Metadata.UID}
	path := "/bootassignments/by-name/" + url.PathEscape(assignment.Metadata.Name)
	var current v1.BootAssignment
	status, err = a.request(ctx, http.MethodGet, path, nil, nil, &current)
	if err != nil && status != http.StatusNotFound {
		return err
	}
	if status != http.StatusNotFound {
		if !reflect.DeepEqual(current.Spec, assignment.Spec) {
			return fmt.Errorf("target assignment already has different desired content")
		}
		return nil
	}
	_, err = a.request(ctx, http.MethodPut, path, desiredInput[v1.BootAssignmentSpec]{Metadata: desiredMetadata{Name: assignment.Metadata.Name}, Spec: assignment.Spec}, http.Header{"If-None-Match": {"*"}}, &current)
	return err
}
