package asn_test

import (
	"testing"

	"github.com/binsarjr/regionchecker/internal/asn"
)

func TestBrandCountry(t *testing.T) {
	cases := []struct {
		org     string
		wantCC  string
		wantHit bool
	}{
		{"PT Telkom Indonesia", "ID", true},
		{"Biznet Networks", "ID", true},
		{"PT INDIHOME TBK", "ID", true},
		{"LINKNET", "ID", true},
		{"CBN Nusantara", "ID", true},
		{"TOKOPEDIA", "ID", true},
		{"PT. Tokopedia", "ID", true},
		{"BUKALAPAK-AS-ID", "ID", true},
		{"GOJEK-AS", "ID", true},
		{"Traveloka", "ID", true},
		{"Blibli.com", "ID", true},
		{"PT. First Media, Tbk", "ID", true},
		{"GOOGLE-LLC", "", false},
		{"Cloudflare, Inc.", "", false},
		{"Alibaba US LLC", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		cc, hit := asn.BrandCountry(tc.org)
		if cc != tc.wantCC || hit != tc.wantHit {
			t.Errorf("BrandCountry(%q) = (%q, %v), want (%q, %v)", tc.org, cc, hit, tc.wantCC, tc.wantHit)
		}
	}
}

func TestBoostCountry_Alias(t *testing.T) {
	// Deprecated alias kept for API stability.
	cc, hit := asn.BoostCountry("TOKOPEDIA")
	if cc != "ID" || !hit {
		t.Errorf("BoostCountry = (%q, %v), want (ID, true)", cc, hit)
	}
}

func TestNormalizeOrg(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"  PT  Telkom  Indonesia  ", "PT Telkom Indonesia"},
		{"GOOGLE, LLC", "GOOGLE"},
		{"ACME CORP", "ACME"},
		{"", ""},
	}
	for _, tc := range cases {
		got := asn.NormalizeOrg(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeOrg(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
