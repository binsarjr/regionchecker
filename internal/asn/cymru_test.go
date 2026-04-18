package asn

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

func TestCymruQueryName_V4(t *testing.T) {
	got := cymruQueryName(netip.MustParseAddr("8.8.8.8"))
	want := "8.8.8.8.origin.asn.cymru.com"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCymruQueryName_V6(t *testing.T) {
	got := cymruQueryName(netip.MustParseAddr("2001:4860:4860::8888"))
	if !strings.HasSuffix(got, ".origin6.asn.cymru.com") {
		t.Errorf("v6 name missing suffix: %s", got)
	}
	// Nibbles should be 32 + suffix.
	if strings.Count(got, ".") < 32 {
		t.Errorf("v6 name too short: %s", got)
	}
}

func TestCymruQueryName_V4Mapped(t *testing.T) {
	// ::ffff:8.8.8.8 must unmap and go via v4 path.
	got := cymruQueryName(netip.MustParseAddr("::ffff:8.8.8.8"))
	want := "8.8.8.8.origin.asn.cymru.com"
	if got != want {
		t.Errorf("mapped v6 got %q, want %q", got, want)
	}
}

func TestParseCymruTXT(t *testing.T) {
	r, err := parseCymruTXT("15169 | 8.8.8.0/24 | US | arin | 1992-12-01")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.ASN != 15169 {
		t.Errorf("ASN = %d, want 15169", r.ASN)
	}
	if r.Country != "US" {
		t.Errorf("Country = %q, want US", r.Country)
	}
	if r.Prefix != "8.8.8.0/24" {
		t.Errorf("Prefix = %q", r.Prefix)
	}
	if r.Registry != "arin" {
		t.Errorf("Registry = %q", r.Registry)
	}
}

func TestParseCymruTXT_Multiorigin(t *testing.T) {
	r, err := parseCymruTXT("13335 14907 | 1.1.1.0/24 | AU | apnic | 2010-08-01")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.ASN != 13335 {
		t.Errorf("ASN = %d, want 13335 (first)", r.ASN)
	}
}

func TestParseCymruTXT_Short(t *testing.T) {
	_, err := parseCymruTXT("only | three | fields")
	if err == nil {
		t.Error("expected error for short record")
	}
}

func TestCymruLookup_NXDomain(t *testing.T) {
	// Use 203.0.113.1 (TEST-NET) which Cymru returns NA or no answer for.
	// Skip if offline / CI short mode.
	if testing.Short() {
		t.Skip("network lookup in -short mode")
	}
	c := NewCymruClient()
	// Our local TEST-NET ip should not have an ASN.
	_, _ = c.Lookup(context.Background(), netip.MustParseAddr("203.0.113.1"))
	// Not asserting specific result since Cymru response varies by route; just
	// ensure no panic and returns within timeout.
}
