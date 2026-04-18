package domain

import (
	"net"
	"strings"
)

// Country returns (cc, suffixType, confidence) for a hostname.
//
//	suffixType ∈ {"cctld","idn","geo-gtld","generic",""}
//	confidence ∈ {"high","medium","low","unknown"}
func Country(host string) (cc, suffixType, confidence string) {
	if host == "" {
		return "", "", "unknown"
	}

	// Raw IP → not a domain.
	if net.ParseIP(host) != nil {
		return "", "", "unknown"
	}

	// Strip trailing dot, lowercase.
	host = strings.TrimSuffix(host, ".")
	host = strings.ToLower(host)

	// Reject overly long hostnames.
	if len(host) > 253 {
		return "", "", "unknown"
	}

	// Single label (no dot) → no TLD to resolve.
	if !strings.Contains(host, ".") {
		return "", "", "unknown"
	}

	// Normalize IDN to Punycode (best-effort; ignore error and use original).
	ascii, err := ToASCII(host)
	if err == nil {
		host = ascii
	}

	// Extract effective TLD.
	etld, _ := EffectiveTLD(host)
	if etld == "" {
		return "", "generic", "low"
	}
	// Reduce multi-label eTLD to the rightmost label for map lookups
	// (e.g. "co.id" → "id", "com.au" → "au").
	tld := etld
	if idx := strings.LastIndex(etld, "."); idx >= 0 {
		tld = etld[idx+1:]
	}

	// 1. ccTLD map.
	if mapped, ok := ccTLDMap[tld]; ok {
		switch tld {
		case "eu":
			return mapped, "cctld", "low" // multinational
		case "su":
			return mapped, "cctld", "medium" // historical
		default:
			return mapped, "cctld", "high"
		}
	}

	// 2. IDN ccTLD map (Punycode keys).
	if mapped, ok := IDNCountry(tld); ok {
		return mapped, "idn", "high"
	}

	// 3. Geographic gTLD.
	if mapped, ok := GeoCountry(tld); ok {
		return mapped, "geo-gtld", "medium"
	}

	// 4. Generic / brand.
	return "", "generic", "low"
}
