package domain_test

import (
	"testing"

	"github.com/binsarjr/regionchecker/internal/domain"
)

var countryTests = []struct {
	host     string
	wantCC   string
	wantType string
	wantConf string
}{
	// Standard ccTLD
	{"google.co.id", "ID", "cctld", "high"},
	{"www.unpad.ac.id", "ID", "cctld", "high"},
	{"bbc.co.uk", "GB", "cctld", "high"},
	{"example.de", "DE", "cctld", "high"},
	{"example.fr", "FR", "cctld", "high"},
	{"example.jp", "JP", "cctld", "high"},
	{"example.au", "AU", "cctld", "high"},
	{"example.cn", "CN", "cctld", "high"},
	{"example.ru", "RU", "cctld", "high"},
	{"example.br", "BR", "cctld", "high"},
	// Special exceptions
	{"example.eu", "EU", "cctld", "low"},
	{"old.su", "SU", "cctld", "medium"},
	// IDN ccTLD (Punycode) — xn--p1ai is in idnTLDMap → type "idn"
	{"xn--e1afmkfd.xn--p1ai", "RU", "idn", "high"},
	// Geo-gTLD
	{"techcrunch.berlin", "DE", "geo-gtld", "medium"},
	{"brand.tokyo", "JP", "geo-gtld", "medium"},
	// Generic
	{"example.com", "", "generic", "low"},
	{"example.org", "", "generic", "low"},
	{"example.net", "", "generic", "low"},
	{"example.io", "IO", "cctld", "high"}, // British Indian Ocean Territory ccTLD
	// Raw IP
	{"8.8.8.8", "", "", "unknown"},
	{"2001:db8::1", "", "", "unknown"},
	// Trailing dot stripped
	{"example.id.", "ID", "cctld", "high"},
	// Single label
	{"localhost", "", "", "unknown"},
	// Empty
	{"", "", "", "unknown"},
}

func TestCountry(t *testing.T) {
	for _, tc := range countryTests {
		t.Run(tc.host, func(t *testing.T) {
			cc, typ, conf := domain.Country(tc.host)
			if cc != tc.wantCC || typ != tc.wantType || conf != tc.wantConf {
				t.Errorf("Country(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.host, cc, typ, conf, tc.wantCC, tc.wantType, tc.wantConf)
			}
		})
	}
}

func TestIDNCountry(t *testing.T) {
	cases := []struct {
		tld  string
		want string
	}{
		{"xn--p1ai", "RU"},
		{"xn--90a3ac", "RS"},
		{"xn--3e0b707e", "KR"},
		{"xn--wgbh1c", "EG"},
		{"xn--o3cw4h", "TH"},
	}
	for _, tc := range cases {
		cc, ok := domain.IDNCountry(tc.tld)
		if !ok || cc != tc.want {
			t.Errorf("IDNCountry(%q) = (%q, %v), want (%q, true)", tc.tld, cc, ok, tc.want)
		}
	}
}

func TestGeoCountry(t *testing.T) {
	cases := []struct {
		tld  string
		want string
	}{
		{"berlin", "DE"},
		{"tokyo", "JP"},
		{"london", "GB"},
		{"paris", "FR"},
		{"nyc", "US"},
		{"moscow", "RU"},
	}
	for _, tc := range cases {
		cc, ok := domain.GeoCountry(tc.tld)
		if !ok || cc != tc.want {
			t.Errorf("GeoCountry(%q) = (%q, %v), want (%q, true)", tc.tld, cc, ok, tc.want)
		}
	}
}

func TestEffectiveTLD(t *testing.T) {
	cases := []struct {
		host     string
		wantETLD string
	}{
		{"google.co.id", "co.id"},
		{"example.com", "com"},
		{"bbc.co.uk", "co.uk"},
		{"www.unpad.ac.id", "ac.id"},
	}
	for _, tc := range cases {
		etld, _ := domain.EffectiveTLD(tc.host)
		if etld != tc.wantETLD {
			t.Errorf("EffectiveTLD(%q) = %q, want %q", tc.host, etld, tc.wantETLD)
		}
	}
}
