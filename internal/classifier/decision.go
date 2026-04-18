package classifier

// Confidence tiers returned by Decide.
const (
	ConfHigh                    = "high"
	ConfMediumDomainIDOffshore  = "medium-domain-id-offshore-host"
	ConfMediumGenericTLDIDHost  = "medium-generic-tld-id-host"
	ConfMediumDomainCCMismatch  = "medium-domain-cc-mismatch"
	ConfLowDNSFailed            = "low-dns-failed"
	ConfIPOnly                  = "ip-only"
	ConfUnknown                 = "unknown"
)

// Decision holds the final country code, confidence tier, and human-readable reason.
type Decision struct {
	FinalCountry string
	Confidence   string
	Reason       string
}

// Decide merges a domain-level country (from the suffix dispatcher) with an
// IP-level country (from the RIR lookup) into a final country + confidence.
//
//	domainCC     domain-level country, "" if not applicable (raw IP) or unknown
//	domainType   suffix type: "cctld", "idn", "geo-gtld", "generic", ""
//	ipCC         country of the first resolved IP, "" if DNS failed or all bogons
//	dnsFailed    true when host lookup returned zero usable addresses
//	isIPInput    true when the caller passed a raw IP (no domain branch)
func Decide(domainCC, domainType, ipCC string, dnsFailed, isIPInput bool) Decision {
	// Raw IP input: IP-only tier.
	if isIPInput {
		return Decision{
			FinalCountry: ipCC,
			Confidence:   ConfIPOnly,
			Reason:       "raw ip input; rir lookup",
		}
	}

	// Host input but DNS failed: fall back to domain cc if present.
	if dnsFailed {
		if domainCC != "" {
			return Decision{
				FinalCountry: domainCC,
				Confidence:   ConfLowDNSFailed,
				Reason:       "dns failed; used " + domainType + " signal",
			}
		}
		return Decision{Confidence: ConfUnknown, Reason: "dns failed; no domain signal"}
	}

	// Both signals present and agree: high confidence.
	if domainCC != "" && ipCC != "" && domainCC == ipCC {
		return Decision{
			FinalCountry: domainCC,
			Confidence:   ConfHigh,
			Reason:       "domain " + domainType + " matches ip country",
		}
	}

	// Domain present but IP disagrees.
	if domainCC != "" && ipCC != "" && domainCC != ipCC {
		// .id offshore-host pattern.
		if domainCC == "ID" {
			return Decision{
				FinalCountry: domainCC,
				Confidence:   ConfMediumDomainIDOffshore,
				Reason:       "domain .id but host ip in " + ipCC,
			}
		}
		return Decision{
			FinalCountry: domainCC,
			Confidence:   ConfMediumDomainCCMismatch,
			Reason:       "domain " + domainCC + " but host ip in " + ipCC,
		}
	}

	// Generic TLD + ID host: medium.
	if domainCC == "" && (domainType == "generic" || domainType == "") && ipCC == "ID" {
		return Decision{
			FinalCountry: "ID",
			Confidence:   ConfMediumGenericTLDIDHost,
			Reason:       "generic tld resolved to id host",
		}
	}

	// Only one signal.
	if domainCC != "" && ipCC == "" {
		return Decision{
			FinalCountry: domainCC,
			Confidence:   ConfLowDNSFailed,
			Reason:       "domain " + domainType + " signal only",
		}
	}
	if ipCC != "" {
		return Decision{
			FinalCountry: ipCC,
			Confidence:   ConfIPOnly,
			Reason:       "ip country only",
		}
	}

	return Decision{Confidence: ConfUnknown, Reason: "no country signal"}
}
