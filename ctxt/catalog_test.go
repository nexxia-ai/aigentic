package ctxt

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCatalogAppendAndList(t *testing.T) {
	base := t.TempDir()
	entry := catalogEntry{
		ID:        "20250604-abc12345",
		Name:      "n",
		Summary:   "s",
		Path:      filepath.Join(base, "runs", "20250604", "20250604-abc12345"),
		TurnCount: 2,
		UpdatedAt: time.Now().UTC(),
	}
	if err := appendCatalogEvent(base, entry); err != nil {
		t.Fatal(err)
	}
	list, err := materializeCatalog(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != entry.ID {
		t.Fatalf("list=%+v", list)
	}
}

func TestCatalogEndTurnAppendsOnly(t *testing.T) {
	base := t.TempDir()
	for i := 0; i < 50; i++ {
		id := NewRunID(time.Now().UTC().Add(time.Duration(i) * time.Second))
		e := catalogEntry{ID: id, Name: id, Path: RunDir(base, id), UpdatedAt: time.Now().UTC()}
		if err := appendCatalogEvent(base, e); err != nil {
			t.Fatal(err)
		}
	}
	snapBefore, _ := os.Stat(catalogSnapshotPath(base))
	logBefore, _ := os.Stat(catalogLogPath(base))
	target := catalogEntry{ID: "20250604-target01", Name: "target", UpdatedAt: time.Now().UTC()}
	if err := appendCatalogEvent(base, target); err != nil {
		t.Fatal(err)
	}
	snapAfter, _ := os.Stat(catalogSnapshotPath(base))
	logAfter, err := os.Stat(catalogLogPath(base))
	if err != nil {
		t.Fatal(err)
	}
	if snapBefore != nil && snapAfter != nil && snapAfter.ModTime().After(snapBefore.ModTime()) {
		t.Fatal("snapshot should not be rewritten on normal append")
	}
	if logBefore != nil && logAfter.Size() <= logBefore.Size() {
		t.Fatal("catalog log should grow")
	}
}

func TestCatalogCacheReusesUntilFileChanges(t *testing.T) {
	base := t.TempDir()
	entry := catalogEntry{ID: "20250604-cache01", Name: "c", UpdatedAt: time.Now().UTC()}
	if err := appendCatalogEvent(base, entry); err != nil {
		t.Fatal(err)
	}
	a, err := materializeCatalog(base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := materializeCatalog(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatal("cache mismatch")
	}
	if err := appendCatalogEvent(base, catalogEntry{ID: "20250604-cache02", Name: "d", UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	c, err := materializeCatalog(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 2 {
		t.Fatalf("len=%d", len(c))
	}
}

func TestCatalogCompactionCrashDuplicateLogSafe(t *testing.T) {
	base := t.TempDir()
	id := "20250604-duprun01"
	stale := catalogEntry{
		ID: id, Name: "n", TurnCount: 1, UpdatedAt: time.Now().UTC().Add(-time.Hour),
	}
	fresh := catalogEntry{
		ID: id, Name: "n", TurnCount: 5, UpdatedAt: time.Now().UTC(),
	}
	snap := catalogSnapshot{Version: catalogStorageVersion, Runs: []catalogEntry{stale}}
	if err := writeAtomicJSON(catalogSnapshotPath(base), snap); err != nil {
		t.Fatal(err)
	}
	if err := appendCatalogEvent(base, fresh); err != nil {
		t.Fatal(err)
	}
	list, err := materializeCatalog(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].TurnCount != 5 {
		t.Fatalf("list=%+v", list)
	}
}
