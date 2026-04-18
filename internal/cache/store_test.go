package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAtomicWriteRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("hello cache payload")
	if err := s.AtomicWrite("k1", want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read("k1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("roundtrip mismatch: got %q want %q", got, want)
	}
	entries, err := os.ReadDir(s.TmpDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("tmp should be empty after atomic write, got %d entries", len(entries))
	}
}

func TestStoreMetaRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	fetched := time.Now().UTC().Truncate(time.Second)
	m := Meta{
		ETag:         `"abc"`,
		LastModified: "Wed, 21 Oct 2015 07:28:00 GMT",
		SHA256:       "deadbeef",
		FetchedAt:    fetched,
		Bytes:        42,
	}
	if err := s.WriteMeta("k1", m); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadMeta("k1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ETag != m.ETag || got.LastModified != m.LastModified || got.SHA256 != m.SHA256 || got.Bytes != m.Bytes {
		t.Fatalf("meta mismatch: %+v vs %+v", got, m)
	}
	if !got.FetchedAt.Equal(m.FetchedAt) {
		t.Fatalf("fetched_at mismatch: %v vs %v", got.FetchedAt, m.FetchedAt)
	}
}

func TestStoreSweepTmp(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(s.TmpDir(), "orphan.old")
	fresh := filepath.Join(s.TmpDir(), "orphan.fresh")
	for _, p := range []string{old, fresh} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if err := s.SweepTmp(30 * time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old tmp survived sweep: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh tmp removed by sweep: %v", err)
	}
}

func TestStoreReadMissing(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read("nope"); !os.IsNotExist(err) {
		t.Fatalf("expected not exist, got %v", err)
	}
	if _, err := s.ReadMeta("nope"); !os.IsNotExist(err) {
		t.Fatalf("expected not exist, got %v", err)
	}
}
