package rir

import (
	"net/netip"
	"strings"
	"testing"
)

const dbSample = `2|apnic|20260417|6|19830613|20260417|+1000
apnic|*|ipv4|*|3|summary
apnic|*|ipv6|*|2|summary
apnic|*|asn|*|1|summary
apnic|CN|ipv4|1.0.1.0|256|20110414|allocated
apnic|JP|ipv4|1.0.16.0|4096|20110414|allocated
apnic|ID|ipv4|49.0.0.0|262144|20100101|allocated
apnic|AU|ipv6|2001:dc0::|32|20020801|allocated
apnic|ID|ipv6|2001:df0::|32|20030101|allocated
apnic|KR|asn|9318|1|20021219|allocated
`

func TestBuildAndLookup(t *testing.T) {
	db, err := Build(strings.NewReader(dbSample))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(db.V4) != 3 || len(db.V6) != 2 || len(db.ASN) != 1 {
		t.Fatalf("counts v4=%d v6=%d asn=%d", len(db.V4), len(db.V6), len(db.ASN))
	}
	cases := []struct {
		ip, want string
	}{
		{"1.0.1.1", "CN"},
		{"1.0.1.255", "CN"},
		{"1.0.17.1", "JP"},
		{"49.0.109.161", "ID"},
		{"8.8.8.8", ""},       // not in sample
		{"2001:dc0::1", "AU"},
		{"2001:df0::cafe", "ID"},
		{"2001:4860::1", ""},
	}
	for _, c := range cases {
		t.Run(c.ip, func(t *testing.T) {
			addr := netip.MustParseAddr(c.ip)
			cc, _, ok := db.LookupIP(addr)
			if c.want == "" {
				if ok {
					t.Errorf("want miss got %s", cc)
				}
				return
			}
			if !ok || cc != c.want {
				t.Errorf("got %q want %q", cc, c.want)
			}
		})
	}
	// ASN
	if cc, _, ok := db.LookupASN(9318); !ok || cc != "KR" {
		t.Errorf("asn: got %q ok=%v", cc, ok)
	}
	if _, _, ok := db.LookupASN(99999); ok {
		t.Error("asn miss expected")
	}
}

func BenchmarkLookupIP(b *testing.B) {
	db, err := Build(strings.NewReader(dbSample))
	if err != nil {
		b.Fatal(err)
	}
	ip := netip.MustParseAddr("49.0.109.161")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = db.LookupIP(ip)
	}
}
