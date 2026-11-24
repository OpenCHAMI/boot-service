// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package boot

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

type failingDeclarativeController struct{ err error }

func (c failingDeclarativeController) GenerateBootScript(context.Context, string, string) (string, error) {
	return "", c.err
}

func TestDeclarativeFailureReturnsNonbootingScript(t *testing.T) {
	assignment := v1.BootAssignment{Metadata: resource.Metadata{Name: "canary"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "alpha"}}}
	failure := declarativeboot.WrapSelectedReferenceFailure(assignment, errors.New("private-kernel-params"))
	failure.(*declarativeboot.ReferenceFailure).ResolvedUID = "rev_diagnostic"
	handler := NewHandlerWithController(client.Client{}, failingDeclarativeController{err: failure}, log.New(io.Discard, "", 0))
	recorder := httptest.NewRecorder()
	handler.GetBootScript(recorder, httptest.NewRequest(http.MethodGet, "/bootscript?mac=aa:bb:cc:dd:ee:ff", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Content-Type") != "application/ipxe" {
		t.Fatalf("status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	body := recorder.Body.String()
	if !strings.HasPrefix(body, "#!ipxe\n") || !strings.Contains(body, "canary") || !strings.Contains(body, "alpha") || !strings.Contains(body, "rev_diagnostic") || !strings.Contains(body, "exit 1\n") || strings.Contains(body, "private-kernel-params") {
		t.Fatalf("unsafe refusal: %q", body)
	}
}
