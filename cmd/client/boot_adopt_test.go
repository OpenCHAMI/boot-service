// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/openchami/boot-service/pkg/bootadopt"
)

func TestAdoptionConflictingFlagsCertifyNoPendingRequests(t *testing.T) {
	cmd := newBootAdoptCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--dry-run", "--execute"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("conflicting flags succeeded")
	}
	var receipt bootadopt.Receipt
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Operation != "boot-adopt" || receipt.Phase != "preflight" || receipt.Requests.Started != 0 || receipt.Requests.Completed != 0 || receipt.Requests.Ambiguous {
		t.Fatalf("invalid preflight receipt: %#v", receipt)
	}
}
