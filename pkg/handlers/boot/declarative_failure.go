// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package boot

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/openchami/boot-service/pkg/controllers/bootscript"
	"github.com/openchami/boot-service/pkg/declarativeboot"
)

func writeDeclarativeFailure(w http.ResponseWriter, err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var delivery *bootscript.DeliveryFailure
	var reference *declarativeboot.ReferenceFailure
	if !errors.As(err, &delivery) && !errors.As(err, &reference) {
		return false
	}
	w.Header().Set("Content-Type", "application/ipxe")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprint(w, "#!ipxe\n")
	if errors.As(err, &reference) {
		_, _ = fmt.Fprintf(w, "# assignment=%q reference=%q uid=%q revision=%q selector=%q resolved_uid=%q\n", reference.AssignmentName, reference.Reference.Name, reference.Reference.UID, reference.Reference.RevisionName, reference.Reference.Selector, reference.ResolvedUID)
	}
	_, _ = fmt.Fprint(w, "echo Boot unavailable\nexit 1\n")
	return true
}
