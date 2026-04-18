package asn_test

import (
	"testing"

	"github.com/binsarjr/regionchecker/internal/asn"
)

func TestBoostCountry(t *testing.T) {
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
		{"GOOGLE-LLC", "", false},
		{"Cloudflare, Inc.", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		cc, hit := asn.BoostCountry(tc.org)
		if cc != tc.wantCC || hit != tc.wantHit {
			t.Errorf("BoostCountry(%q) = (%q, %v), want (%q, %v)", tc.org, cc, hit, tc.wantCC, tc.wantHit)
		}
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
