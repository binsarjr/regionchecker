package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/binsarjr/regionchecker/internal/cache"
	"github.com/binsarjr/regionchecker/internal/config"
	"github.com/binsarjr/regionchecker/internal/rir"
)

func TestDedup(t *testing.T) {
	in := []string{"a", "b", "a", "c", "b", "d"}
	got := dedup(in)
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOpenSnapshotMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := cache.New(dir); err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	_, err := openSnapshot(config.Config{CacheDir: dir})
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestOpenSnapshotPresent(t *testing.T) {
	dir := t.TempDir()
	store, err := cache.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	db := &rir.DB{
		V4: []rir.V4Range{{Start: 1, End: 10, CC: [2]byte{'A', 'U'}}},
	}
	var buf bytes.Buffer
	if err := rir.Snapshot(db, [32]byte{}, &buf); err != nil {
		t.Fatal(err)
	}
	path := store.ParsedPath(parsedSnapshotName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := openSnapshot(config.Config{CacheDir: dir})
	if err != nil {
		t.Fatalf("openSnapshot: %v", err)
	}
	defer snap.Close()
	if len(snap.DB.V4) != 1 {
		t.Errorf("V4 len = %d, want 1", len(snap.DB.V4))
	}
}

func TestSweepOrphansNoDir(t *testing.T) {
	if err := sweepOrphans(filepath.Join(t.TempDir(), "nope"), 0); err != nil {
		t.Errorf("sweepOrphans on missing dir: %v", err)
	}
}

func TestSweepOrphansRemovesOld(t *testing.T) {
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.tmp")
	if err := os.WriteFile(oldFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(oldFile, past, past); err != nil {
		t.Fatal(err)
	}
	if err := sweepOrphans(dir, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file should be removed")
	}
}
