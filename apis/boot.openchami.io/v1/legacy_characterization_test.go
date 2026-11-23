// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"encoding/json"
	"testing"
)

func TestLegacyBootConfigurationSchemaAndValidation(t *testing.T) {
	input := `{"metadata":{"name":"legacy"},"spec":{"hosts":["x0c0s0b0n0"],"macs":["aa:bb:cc:dd:ee:ff"],"nids":[1],"groups":["compute"],"profile":"debug","kernel":"https://example.com/kernel","initrd":"/boot/initrd","params":"quiet","priority":42}}`
	var config BootConfiguration
	if err := json.Unmarshal([]byte(input), &config); err != nil {
		t.Fatal(err)
	}
	if err := config.Validate(context.Background()); err != nil {
		t.Fatalf("legacy fixture rejected: %v", err)
	}
	data, err := json.Marshal(config.Spec)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"hosts", "macs", "nids", "groups", "profile", "kernel", "initrd", "params", "priority"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("legacy field %q missing from %s", key, data)
		}
	}
	if len(fields) != 9 {
		t.Fatalf("legacy schema gained fields: %s", data)
	}
}
