package domain

import (
	"strings"

	"golang.org/x/net/idna"
)

// idnTLDMap maps Punycode (ACE form) IDN ccTLDs to ISO 3166-1 alpha-2.
var idnTLDMap = map[string]string{
	"xn--p1ai":               "RU",
	"xn--90a3ac":             "RS",
	"xn--j1amh":              "UA",
	"xn--fiqs8s":             "CN",
	"xn--fiqz9s":             "CN",
	"xn--kpry57d":            "TW",
	"xn--kprw13d":            "TW",
	"xn--3e0b707e":           "KR",
	"xn--mgbaam7a8h":         "AE",
	"xn--mgberp4a5d4ar":      "SA",
	"xn--mgba3a4f16a":        "IR",
	"xn--wgbh1c":             "EG",
	"xn--wgbl6a":             "QA",
	"xn--mgbbh1a71e":         "IN",
	"xn--h2brj9c":            "IN",
	"xn--fpcrj9c3d":          "IN",
	"xn--gecrj9c":            "IN",
	"xn--s9brj9c":            "IN",
	"xn--xkc2dl3a5ee0h":      "IN",
	"xn--45brj9c":            "IN",
	"xn--2scrj9c":            "IN",
	"xn--rvc1e0am3e":         "IN",
	"xn--o3cw4h":             "TH",
	"xn--q7ce6a":             "LA",
	"xn--mix891f":            "MO",
	"xn--node":               "GE",
	"xn--qxa6a":              "EU",
	"xn--ygbi2ammx":          "PS",
	"xn--d1alf":              "MK",
	"xn--clchc0ea0b2g2a9gcd": "SG",
	"xn--lgbbat1ad8j":        "DZ",
	"xn--mgb9awbf":           "OM",
	"xn--mgbai9azgqp6j":      "PK",
	"xn--mgbtx2b":            "IQ",
	"xn--mgbx4cd0ab":         "MY",
	"xn--pgbs0dh":            "TN",
	"xn--ygbi2ammj":          "JO",
	"xn--54b7fta0cc":         "BD",
	"xn--mgbc0a9azcg":        "MA",
	"xn--l1acc":              "MN",
}

// IDNCountry returns the country code for a Punycode-encoded IDN ccTLD.
func IDNCountry(tld string) (string, bool) {
	cc, ok := idnTLDMap[strings.ToLower(tld)]
	return cc, ok
}

// ToASCII normalizes a hostname to its Punycode ACE form.
func ToASCII(host string) (string, error) {
	return idna.Lookup.ToASCII(host)
}
