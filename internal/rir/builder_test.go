package rir

import (
	"strings"
	"testing"
)

// minimalStats builds a minimal delegated-stats stream for testing.
func minimalStats(lines ...string) string {
	header := "2|apnic|20240101|4|19700101|20240101|+0000\n" +
		"apnic|*|*|*|summary|*|*\n"
	return header + strings.Join(lines, "\n") + "\n"
}

func TestBuild_BasicV4V6ASN(t *testing.T) {
	data := minimalStats(
		"apnic|AU|ipv4|1.0.0.0|256|20110811|allocated",
		"apnic|CN|ipv4|1.0.1.0|256|20110811|allocated",
		"apnic|ID|ipv6|2001:e10::|32|20090612|allocated",
		"apnic|JP|asn|4608|1|19930101|allocated",
	)
	db, err := Build(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if len(db.V4) != 2 {
		t.Errorf("V4 len = %d, want 2", len(db.V4))
	}
	if len(db.V6) != 1 {
		t.Errorf("V6 len = %d, want 1", len(db.V6))
	}
	if len(db.ASN) != 1 {
		t.Errorf("ASN len = %d, want 1", len(db.ASN))
	}
	if cc := string(db.V4[0].CC[:]); cc != "AU" {
		t.Errorf("V4[0].CC = %q, want AU (sorted by start)", cc)
	}
	if cc := string(db.V6[0].CC[:]); cc != "ID" {
		t.Errorf("V6[0].CC = %q, want ID", cc)
	}
}

func TestBuild_SkipsInvalidCC(t *testing.T) {
	data := minimalStats(
		"apnic||ipv4|1.0.0.0|256|20110811|reserved",
		"apnic|X|ipv4|1.0.1.0|256|20110811|available",
		"apnic|AU|ipv4|1.0.2.0|256|20110811|allocated",
	)
	db, err := Build(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if len(db.V4) != 1 {
		t.Errorf("V4 len = %d, want 1 (invalid CCs skipped)", len(db.V4))
	}
}

func TestBuild_SkipsZeroValueV4(t *testing.T) {
	data := minimalStats(
		"apnic|AU|ipv4|1.0.0.0|0|20110811|allocated",
		"apnic|AU|ipv4|1.0.1.0|256|20110811|allocated",
	)
	db, err := Build(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if len(db.V4) != 1 {
		t.Errorf("V4 len = %d, want 1 (zero-value skipped)", len(db.V4))
	}
}

func TestBuild_V4SortOrder(t *testing.T) {
	data := minimalStats(
		"apnic|CN|ipv4|8.0.0.0|256|20110811|allocated",
		"apnic|AU|ipv4|1.0.0.0|256|20110811|allocated",
		"apnic|JP|ipv4|4.0.0.0|256|20110811|allocated",
	)
	db, err := Build(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	for i := 1; i < len(db.V4); i++ {
		if db.V4[i].Start < db.V4[i-1].Start {
			t.Errorf("V4 not sorted at index %d: %d < %d", i, db.V4[i].Start, db.V4[i-1].Start)
		}
	}
}

var prefixEndTests = []struct {
	name    string
	hi, lo  uint64
	bits    uint
	wantHi  uint64
	wantLo  uint64
}{
	{
		name:   "bits=128 no change",
		hi:     0x20010db800000000,
		lo:     0,
		bits:   128,
		wantHi: 0x20010db800000000,
		wantLo: 0,
	},
	{
		name:   "bits=32 /32 in v6 space",
		hi:     0x20010db800000000,
		lo:     0,
		bits:   32,
		wantHi: 0x20010db8ffffffff,
		wantLo: ^uint64(0),
	},
	{
		name:   "bits=64 boundary",
		hi:     0,
		lo:     0,
		bits:   64,
		wantHi: 0,
		wantLo: ^uint64(0),
	},
	{
		name:   "bits=96",
		hi:     0x20010db800000000,
		lo:     0,
		bits:   96,
		wantHi: 0x20010db800000000,
		wantLo: 0x00000000ffffffff,
	},
	{
		name:   "bits=127",
		hi:     0,
		lo:     0,
		bits:   127,
		wantHi: 0,
		wantLo: 1,
	},
}

func TestPrefixEnd128(t *testing.T) {
	for _, tc := range prefixEndTests {
		t.Run(tc.name, func(t *testing.T) {
			gotHi, gotLo := prefixEnd128(tc.hi, tc.lo, tc.bits)
			if gotHi != tc.wantHi || gotLo != tc.wantLo {
				t.Errorf("prefixEnd128(hi=%x, lo=%x, bits=%d) = (%x, %x), want (%x, %x)",
					tc.hi, tc.lo, tc.bits, gotHi, gotLo, tc.wantHi, tc.wantLo)
			}
		})
	}
}

// TestBuild_V6PrefixEnd verifies that the end address of a parsed v6 range
// covers exactly the prefix (non-power-of-2 gotcha is IPv4-only; v6 uses prefix length).
func TestBuild_V6PrefixEnd(t *testing.T) {
	// 2001:db8::/32 — end should be 2001:db8:ffff:ffff:ffff:ffff:ffff:ffff
	data := minimalStats("apnic|AU|ipv6|2001:db8::|32|20240101|allocated")
	db, err := Build(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if len(db.V6) != 1 {
		t.Fatalf("V6 len = %d, want 1", len(db.V6))
	}
	r := db.V6[0]
	wantEndHi := uint64(0x20010db8ffffffff)
	wantEndLo := ^uint64(0)
	if r.EndHi != wantEndHi || r.EndLo != wantEndLo {
		t.Errorf("V6 end = (%x, %x), want (%x, %x)", r.EndHi, r.EndLo, wantEndHi, wantEndLo)
	}
}
