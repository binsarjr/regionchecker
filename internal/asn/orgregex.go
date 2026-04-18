// Package asn provides optional ASN enrichment lookups gated behind
// --online / --mmdb flags.
package asn

import (
	"regexp"
	"strings"
)

// brandRule matches ASN org-name substrings for a country.
type brandRule struct {
	re *regexp.Regexp
	cc string
}

// defaultBrands maps ASN org patterns to country codes. Word-boundary
// anchored to avoid false positives. Order matters only for debugging;
// first match wins.
var defaultBrands = []brandRule{
	// Indonesian carriers, ISPs, hosting.
	{regexp.MustCompile(`(?i)\b(?:TELKOM|TELKOMSEL|BIZNET|INDIHOME|LINKNET|CBN|INDOSAT|AXIATA|SMARTFREN|FIRST[-\s]?MEDIA|HYPERNET|NIAGAHOSTER|DEWAWEB|IDWEBHOST|DATACIPTA|MYREPUBLIC[-\s]?INDONESIA|MORATELINDO)\b`), "ID"},
	// Indonesian internet companies (run own ASN).
	{regexp.MustCompile(`(?i)\b(?:TOKOPEDIA|BUKALAPAK|GOJEK|TRAVELOKA|BLIBLI|HALODOC|JNE|DETIK|KOMPAS[-\s]?GRAMEDIA)\b`), "ID"},
}

// BrandCountry returns the country code for an ASN org name when it
// matches a known brand pattern. Primary signal for catching Indonesian
// companies hosted on foreign clouds (e.g. tokopedia.com on Alibaba US).
func BrandCountry(org string) (string, bool) {
	if org == "" {
		return "", false
	}
	for _, r := range defaultBrands {
		if r.re.MatchString(org) {
			return r.cc, true
		}
	}
	return "", false
}

// BoostCountry is the legacy name for BrandCountry. Kept for API stability.
//
// Deprecated: use BrandCountry.
func BoostCountry(org string) (string, bool) {
	return BrandCountry(org)
}

// NormalizeOrg trims trailing legal suffixes and collapses whitespace.
// Useful for display and ensuring consistent matching.
func NormalizeOrg(org string) string {
	org = strings.TrimSpace(org)
	for _, suffix := range []string{", LTD", ", INC", ", LLC", " PT", " CORP"} {
		org = strings.TrimSuffix(org, suffix)
	}
	return strings.Join(strings.Fields(org), " ")
}
