package rir

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
)

func TestSnapshotRoundTrip(t *testing.T) {
	db, err := Build(strings.NewReader(dbSample))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	sha := SHA256([]byte(dbSample))
	if err := Snapshot(db, sha, &buf); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	loaded, err := LoadSnapshot(&buf, sha)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.V4) != len(db.V4) || len(loaded.V6) != len(db.V6) || len(loaded.ASN) != len(db.ASN) {
		t.Fatalf("count mismatch v4=%d/%d v6=%d/%d asn=%d/%d",
			len(loaded.V4), len(db.V4), len(loaded.V6), len(db.V6), len(loaded.ASN), len(db.ASN))
	}
	for i := range db.V4 {
		if db.V4[i] != loaded.V4[i] {
			t.Errorf("v4[%d] mismatch: %+v vs %+v", i, db.V4[i], loaded.V4[i])
		}
	}
	cc, _, ok := loaded.LookupIP(netip.MustParseAddr("49.0.109.161"))
	if !ok || cc != "ID" {
		t.Errorf("post-load lookup: got %q ok=%v", cc, ok)
	}
}

func TestSnapshotStaleDetection(t *testing.T) {
	db, err := Build(strings.NewReader(dbSample))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	originalSHA := SHA256([]byte(dbSample))
	if err := Snapshot(db, originalSHA, &buf); err != nil {
		t.Fatal(err)
	}
	differentSHA := SHA256([]byte("something else"))
	_, err = LoadSnapshot(&buf, differentSHA)
	if err != ErrSnapshotStale {
		t.Errorf("want ErrSnapshotStale, got %v", err)
	}
}

func TestSnapshotAnySHA(t *testing.T) {
	db, err := Build(strings.NewReader(dbSample))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	sha := SHA256([]byte(dbSample))
	if err := Snapshot(db, sha, &buf); err != nil {
		t.Fatal(err)
	}
	// Zero expectedSHA means accept any
	var zero [32]byte
	if _, err := LoadSnapshot(&buf, zero); err != nil {
		t.Errorf("zero expected should accept, got %v", err)
	}
}
