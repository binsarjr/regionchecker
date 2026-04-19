package classifier

import "strings"

// Confidence tiers used across the ladder.
const (
	ConfHigh                   = "high"
	ConfHighASNBrand           = "high-asn-brand"
	ConfHighSSLCert            = "high-ssl-cert"
	ConfHighCTLog              = "high-ct-log"
	ConfHighContent            = "high-content-scan"
	ConfHighRDAPRegistrant     = "high-rdap-registrant"
	ConfMediumWayback          = "medium-wayback-snapshot"
	ConfMediumDomainIDOffshore = "medium-domain-id-offshore-host"
	ConfMediumGenericTLDIDHost = "medium-generic-tld-id-host"
	ConfMediumDomainCCMismatch = "medium-domain-cc-mismatch"
	ConfLowDNSFailed           = "low-dns-failed"
	ConfIPOnly                 = "ip-only"
	ConfUnknown                = "unknown"
)

// Signals is the input to Decide. All country codes use ISO 3166 alpha-2
// or "" when the signal is absent.
type Signals struct {
	DomainCC   string // from domain suffix (cctld/idn)
	DomainType string // "cctld" | "idn" | "geo-gtld" | "generic" | ""
	IPCC       string // from RIR lookup on first resolved IP
	ASNCC      string // from ASN org brand regex
	RDAPCC     string // from RDAP registrant vCard cc
	DNSFailed  bool
	IsIPInput  bool
}

// Decision holds the final country code, confidence tier, and human-readable reason.
type Decision struct {
	FinalCountry string
	Confidence   string
	Reason       string
}

// Decide merges all available signals into a single Decision.
// Precedence when multiple signals are available:
//  1. Majority vote (2+ signals agreeing on the same CC) → ConfHigh.
//  2. ASN brand alone → ConfHighASNBrand (company identity).
//  3. RDAP registrant alone → ConfHighRDAPRegistrant (domain ownership).
//  4. Fall back to existing domain+IP tiers.
func Decide(s Signals) Decision {
	// Raw IP with no enrichment.
	if s.IsIPInput && s.ASNCC == "" && s.RDAPCC == "" {
		if s.IPCC != "" {
			return Decision{FinalCountry: s.IPCC, Confidence: ConfIPOnly, Reason: "raw ip input; rir lookup"}
		}
		return Decision{Confidence: ConfUnknown, Reason: "no country signal"}
	}

	// Tally votes across all non-empty signals.
	votes := map[string]int{}
	for _, cc := range []string{s.DomainCC, s.IPCC, s.ASNCC, s.RDAPCC} {
		if cc != "" {
			votes[cc]++
		}
	}
	best, n := "", 0
	for cc, v := range votes {
		if v > n {
			best, n = cc, v
		}
	}

	// 2+ signals agree: high confidence.
	if n >= 2 {
		var parts []string
		if s.DomainCC == best {
			parts = append(parts, "domain "+s.DomainType)
		}
		if s.IPCC == best {
			parts = append(parts, "ip")
		}
		if s.ASNCC == best {
			parts = append(parts, "asn brand")
		}
		if s.RDAPCC == best {
			parts = append(parts, "rdap")
		}
		return Decision{
			FinalCountry: best,
			Confidence:   ConfHigh,
			Reason:       strings.Join(parts, "+") + " agree on " + best,
		}
	}

	// ASN brand alone: strong identity signal.
	if s.ASNCC != "" {
		reason := "asn brand " + s.ASNCC
		if s.IPCC != "" && s.IPCC != s.ASNCC {
			reason += " overrides ip " + s.IPCC
		}
		return Decision{FinalCountry: s.ASNCC, Confidence: ConfHighASNBrand, Reason: reason}
	}

	// RDAP registrant alone: domain ownership.
	if s.RDAPCC != "" {
		reason := "rdap registrant " + s.RDAPCC
		if s.IPCC != "" && s.IPCC != s.RDAPCC {
			reason += " overrides ip " + s.IPCC
		}
		return Decision{FinalCountry: s.RDAPCC, Confidence: ConfHighRDAPRegistrant, Reason: reason}
	}

	// DNS failed: fall back to domain cc if present.
	if s.DNSFailed {
		if s.DomainCC != "" {
			return Decision{FinalCountry: s.DomainCC, Confidence: ConfLowDNSFailed, Reason: "dns failed; used " + s.DomainType + " signal"}
		}
		return Decision{Confidence: ConfUnknown, Reason: "dns failed; no domain signal"}
	}

	// Domain + IP present but disagreement.
	if s.DomainCC != "" && s.IPCC != "" && s.DomainCC != s.IPCC {
		if s.DomainCC == "ID" {
			return Decision{FinalCountry: s.DomainCC, Confidence: ConfMediumDomainIDOffshore, Reason: "domain .id but host ip in " + s.IPCC}
		}
		return Decision{FinalCountry: s.DomainCC, Confidence: ConfMediumDomainCCMismatch, Reason: "domain " + s.DomainCC + " but host ip in " + s.IPCC}
	}

	// Generic TLD + ID host heuristic.
	if s.DomainCC == "" && (s.DomainType == "generic" || s.DomainType == "") && s.IPCC == "ID" {
		return Decision{FinalCountry: "ID", Confidence: ConfMediumGenericTLDIDHost, Reason: "generic tld resolved to id host"}
	}

	// Single signals.
	if s.DomainCC != "" && s.IPCC == "" {
		return Decision{FinalCountry: s.DomainCC, Confidence: ConfLowDNSFailed, Reason: "domain " + s.DomainType + " signal only"}
	}
	if s.IPCC != "" {
		return Decision{FinalCountry: s.IPCC, Confidence: ConfIPOnly, Reason: "ip country only"}
	}

	return Decision{Confidence: ConfUnknown, Reason: "no country signal"}
}
