// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/fabrica/pkg/resource"
	"github.com/openchami/fabrica/pkg/revision"
)

func TestSelectionSnapshotRejectsCorruptRevisionName(t *testing.T) {
	for _, selector := range []string{"default", "latest", "uid"} {
		for _, corruption := range []string{"invalid", "unbound", "other-binding", "missing-binding-record", "forged-bound-uid"} {
			t.Run(selector+"/"+corruption, func(t *testing.T) {
				ctx := context.Background()
				dir := t.TempDir()
				if err := storage.InitFileBackend(dir); err != nil {
					t.Fatal(err)
				}
				manager, err := NewManager(ctx, GeneratedStore(), NativeReferenceResolver{})
				if err != nil {
					t.Fatal(err)
				}
				svc := NewRevisionService(manager)
				caps := Capabilities{Propose: true}
				a, err := svc.CreateSeries(ctx, revisionRequest("os", "A", "/kernel-a", caps))
				if err != nil {
					t.Fatal(err)
				}
				kernelB := "/kernel-b"
				if corruption == "forged-bound-uid" {
					kernelB = "/kernel-a"
				}
				b, err := svc.EnsureRevision(ctx, revisionRequest("os", "B", kernelB, caps))
				if err != nil {
					t.Fatal(err)
				}
				ref, selected, otherName := revision.Reference{Name: "os"}, a, "B"
				switch selector {
				case "latest":
					ref.Selector, selected, otherName = revision.SelectorLatest, b, "A"
				case "uid":
					ref.UID = a.Record.Metadata.UID
				}
				_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "all"}, Spec: v1.BootAssignmentSpec{ConfigRef: ref}})
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "revisions", "bootconfig", "records", selected.Record.Metadata.UID+".json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var record revision.Record[*v1.BootConfig]
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				switch corruption {
				case "invalid":
					record.Metadata.RevisionName = "invalid\nboot"
				case "unbound":
					record.Metadata.RevisionName = "Unbound"
				case "other-binding":
					record.Metadata.RevisionName = otherName
				case "missing-binding-record":
					record.Metadata.RevisionName = otherName
					otherUID := b.Record.Metadata.UID
					if selector == "latest" {
						otherUID = a.Record.Metadata.UID
					}
					if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", otherUID+".json")); err != nil {
						t.Fatal(err)
					}
				case "forged-bound-uid":
					record.Metadata.RevisionName = otherName
					otherUID := b.Record.Metadata.UID
					if selector == "latest" {
						otherUID = a.Record.Metadata.UID
					}
					otherPath := filepath.Join(dir, "revisions", "bootconfig", "records", otherUID+".json")
					otherData, err := os.ReadFile(otherPath)
					if err != nil {
						t.Fatal(err)
					}
					var other revision.Record[*v1.BootConfig]
					if err := json.Unmarshal(otherData, &other); err != nil {
						t.Fatal(err)
					}
					other.Metadata.UID = selected.Record.Metadata.UID
					otherData, err = json.Marshal(other)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(otherPath, otherData, 0600); err != nil {
						t.Fatal(err)
					}
				}
				corrupt, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, corrupt, 0600); err != nil {
					t.Fatal(err)
				}
				seriesDir := filepath.Join(dir, "revisions", "bootconfig", "series")
				entries, err := os.ReadDir(seriesDir)
				if err != nil || len(entries) != 1 {
					t.Fatalf("series files: %v, %v", entries, err)
				}
				seriesPath := filepath.Join(seriesDir, entries[0].Name())
				seriesBefore, err := os.ReadFile(seriesPath)
				if err != nil {
					t.Fatal(err)
				}
				snapshot, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
				var failure *ReferenceFailure
				if !matched || !errors.As(err, &failure) || !errors.Is(err, ErrRevisionCorrupt) || snapshot.Content.Kernel != "" {
					t.Fatalf("matched=%v snapshot=%#v error=%v", matched, snapshot, err)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(after, corrupt) {
					t.Fatalf("GET changed record: %v", err)
				}
				seriesAfter, err := os.ReadFile(seriesPath)
				if err != nil || !bytes.Equal(seriesAfter, seriesBefore) {
					t.Fatalf("GET changed series: %v", err)
				}
			})
		}
	}
}

func TestSelectionSnapshotAcceptsUnnamedRevision(t *testing.T) {
	for _, selector := range []string{"latest", "uid"} {
		t.Run(selector, func(t *testing.T) {
			ctx := context.Background()
			svc, manager := revisionServiceFixture(t)
			caps := Capabilities{Propose: true}
			if _, err := svc.CreateSeries(ctx, revisionRequest("os", "A", "/kernel-a", caps)); err != nil {
				t.Fatal(err)
			}
			unnamed, err := svc.CreateRevision(ctx, revisionRequest("os", "", "/kernel-unnamed", caps))
			if err != nil {
				t.Fatal(err)
			}
			ref := revision.Reference{Name: "os", Selector: revision.SelectorLatest}
			if selector == "uid" {
				ref.Selector, ref.UID = "", unnamed.Record.Metadata.UID
			}
			if _, err := manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "all"}, Spec: v1.BootAssignmentSpec{ConfigRef: ref}}); err != nil {
				t.Fatal(err)
			}
			selected, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
			if err != nil || !matched || selected.RevisionName != "" || selected.RevisionUID != unnamed.Record.Metadata.UID || selected.Content.Kernel != "/kernel-unnamed" {
				t.Fatalf("selection=%#v matched=%v err=%v", selected, matched, err)
			}
		})
	}
}

func TestSelectionSnapshotMissingDefaultRecordPreservesSeriesPointers(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := storage.InitFileBackend(dir); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ctx, GeneratedStore(), NativeReferenceResolver{})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewRevisionService(manager)
	caps := Capabilities{Propose: true}
	a, err := svc.CreateSeries(ctx, revisionRequest("os", "A", "/kernel-a", caps))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.EnsureRevision(ctx, revisionRequest("os", "B", "/kernel-b", caps))
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(ctx, v1.BootAssignment{Metadata: resource.Metadata{Name: "all"}, Spec: v1.BootAssignmentSpec{ConfigRef: revision.Reference{Name: "os"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "revisions", "bootconfig", "records", a.Record.Metadata.UID+".json")); err != nil {
		t.Fatal(err)
	}
	_, matched, err := svc.SelectSnapshot(ctx, v1.NodeSpec{}, "")
	var failure *ReferenceFailure
	if !matched || !errors.As(err, &failure) || !errors.Is(err, ErrRevisionCorrupt) {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
	status, _, err := svc.GetSeries(ctx, "os")
	if err != nil || status.DefaultRevisionUID != a.Record.Metadata.UID || status.LatestRevisionUID != b.Record.Metadata.UID {
		t.Fatalf("series=%#v err=%v", status, err)
	}
	latest, err := svc.ResolveRevision(ctx, revision.Reference{Name: "os", Selector: revision.SelectorLatest})
	if err != nil || latest.Metadata.UID != b.Record.Metadata.UID {
		t.Fatalf("latest=%#v err=%v", latest.Metadata, err)
	}
}
