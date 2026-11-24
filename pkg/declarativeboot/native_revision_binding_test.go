// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestNativeRevisionMissingNamedBindingDoesNotResolveDefault(t *testing.T) {
	// Given a restored series whose named binding was lost but whose default
	// record still contains the same revision name.
	ctx := context.Background()
	dir := t.TempDir()
	store, err := revision.NewFileStore[*v1.BootConfig](dir, "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	input := &v1.BootConfig{Spec: v1.BootConfigSpec{Kernel: "/kernel"}}
	created, _, err := store.CreateSeries(ctx, "config", "A", "digest", input)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "revisions", "bootconfig", "series", fmt.Sprintf("sha256.%x.json", sha256.Sum256([]byte("config"))))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var series map[string]json.RawMessage
	if err := json.Unmarshal(raw, &series); err != nil {
		t.Fatal(err)
	}
	delete(series, "revisionNames")
	raw, err = json.Marshal(series)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	// When resolving the missing binding through the native public API.
	got, err := store.Resolve(ctx, revision.Reference{Name: "config", RevisionName: "A"})

	// Then the missing named reference must not silently resolve to default.
	if !errors.Is(err, revision.ErrRevisionNotFound) {
		t.Fatalf("missing named binding resolved to UID %q (default UID %q), error %v", got.Metadata.UID, created.Metadata.UID, err)
	}
}
