// Package asn provides optional ASN enrichment lookups gated behind
// --online / --mmdb flags.
package asn

import (
	"regexp"
	"strings"
)

// idBoosters matches ASN org-name substrings that strongly indicate an
// Indonesian network regardless of RIR country assignment.
var idBoosters = regexp.MustCompile(`(?i)\b(TELKOM|BIZNET|INDIHOME|LINKNET|CBN)\b`)

// BoostCountry returns the boosted country code for an ASN org name.
// If the org matches a known Indonesian carrier pattern, returns ("ID", true).
// Otherwise returns ("", false).
func BoostCountry(org string) (string, bool) {
	if org == "" {
		return "", false
	}
	if idBoosters.MatchString(org) {
		return "ID", true
	}
	return "", false
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
