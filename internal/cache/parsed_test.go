package cache

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/binsarjr/regionchecker/internal/rir"
)

func TestLoadSnapshot(t *testing.T) {
	db := &rir.DB{
		V4: []rir.V4Range{
			{Start: 0x08080800, End: 0x080808FF, CC: [2]byte{'U', 'S'}, Registry: rir.RegARIN, Status: rir.StatusAllocated, Date: 12345},
			{Start: 0x72727272, End: 0x727272FF, CC: [2]byte{'C', 'N'}, Registry: rir.RegAPNIC, Status: rir.StatusAllocated, Date: 23456},
		},
		V6: []rir.V6Range{
			{StartHi: 0x2001486048600000, StartLo: 0, EndHi: 0x2001486048600000, EndLo: 0xffffffffffffffff, CC: [2]byte{'U', 'S'}, Registry: rir.RegARIN, Status: rir.StatusAllocated},
		},
		ASN: []rir.ASNRange{
			{Start: 15169, End: 15169, CC: [2]byte{'U', 'S'}, Registry: rir.RegARIN, Status: rir.StatusAllocated},
		},
	}
	var buf bytes.Buffer
	var sha [32]byte
	copy(sha[:], sha256.New().Sum(nil))
	if err := rir.Snapshot(db, sha, &buf); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snap.bin")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	snap, err := LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	if snap.DB == nil {
		t.Fatal("DB nil")
	}
	if got, want := len(snap.DB.V4), 2; got != want {
		t.Fatalf("v4 count: %d want %d", got, want)
	}
	if got, want := len(snap.DB.V6), 1; got != want {
		t.Fatalf("v6 count: %d want %d", got, want)
	}
	if got, want := len(snap.DB.ASN), 1; got != want {
		t.Fatalf("asn count: %d want %d", got, want)
	}
	if snap.Size != int64(buf.Len()) {
		t.Fatalf("size: %d want %d", snap.Size, buf.Len())
	}
}

func TestVerifySHA256(t *testing.T) {
	raw := []byte("something")
	sum := sha256.Sum256(raw)
	if !VerifySHA256(raw, sum) {
		t.Fatal("expected match")
	}
	var bad [32]byte
	if VerifySHA256(raw, bad) {
		t.Fatal("unexpected match with zero hash")
	}
}
