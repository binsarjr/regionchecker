// Package classifier merges domain, IP, ASN, TLS cert, and RDAP signals
// into a single Result via an early-exit ladder. Cheap signals are
// consulted first; the ladder short-circuits on the first confident
// answer to minimise latency.
package classifier

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/binsarjr/regionchecker/internal/asn"
	"github.com/binsarjr/regionchecker/internal/bogon"
	"github.com/binsarjr/regionchecker/internal/clock"
	"github.com/binsarjr/regionchecker/internal/domain"
	"github.com/binsarjr/regionchecker/internal/rir"
)

// Sentinel errors returned by Classify.
var (
	ErrBogon          = errors.New("regionchecker: reserved range")
	ErrUnresolvable   = errors.New("regionchecker: host unresolvable")
	ErrInvalidInput   = errors.New("regionchecker: invalid ip or host")
	ErrUnknownCountry = errors.New("regionchecker: no country mapping")
)

// IPLookup is the subset of *rir.DB used by the classifier.
type IPLookup interface {
	LookupIP(ip netip.Addr) (cc string, meta rir.Meta, ok bool)
}

// Resolver resolves a hostname to a list of addresses.
type Resolver interface {
	Resolve(ctx context.Context, host string) ([]netip.Addr, error)
}

// ASNLookup returns an ASN and org for an IP.
type ASNLookup interface {
	Lookup(ip netip.Addr) (asn uint32, org string, ok bool)
}

// TLSCertLookup returns the country from the host's TLS leaf cert
// (Subject.Country). Returns ("", false) for DV certs or dial errors.
type TLSCertLookup interface {
	Lookup(ctx context.Context, host string) (cc string, ok bool)
}

// RDAPLookup returns a registrant country code for a domain.
type RDAPLookup interface {
	Lookup(ctx context.Context, domain string) (cc string, ok bool)
}

// ContentScanLookup fetches a site's HTML and returns the country
// whose markers scored highest above the scanner's threshold.
type ContentScanLookup interface {
	Lookup(ctx context.Context, host string) (cc string, ok bool)
}

// WaybackLookup fetches the nearest archived Wayback Machine snapshot
// of host and scores its body with content-scan detectors. Useful for
// expired or temporarily unreachable domains.
type WaybackLookup interface {
	Lookup(ctx context.Context, host string) (cc string, ok bool)
}

// CTLogLookup returns the Subject.Country of a historical certificate
// for host, sourced from Certificate Transparency (crt.sh). Useful
// when the live cert is DV but the site previously held an OV/EV cert.
type CTLogLookup interface {
	Lookup(ctx context.Context, host string) (cc string, ok bool)
}

// Result is the full classification of a single input.
type Result struct {
	Input             string
	Type              string // "ip" | "domain"
	Resolved          []netip.Addr
	DomainCountry     string
	DomainSuffix      string
	IPCountry         string
	ASN               uint32
	ASNOrg            string
	ASNCountry        string
	CertCountry       string // from TLS leaf Subject.Country
	CTLogCountry      string // from historical CT cert Subject.C (crt.sh)
	RegistrantCountry string // from RDAP registrant
	ContentCountry    string // from HTML content scan
	WaybackCountry    string // from Wayback snapshot content scan
	Registry          string
	FinalCountry      string
	Confidence        string
	Reason            string
	LookupDuration    time.Duration
}

// Classifier orchestrates the early-exit ladder. All enrichment fields
// are optional; nil disables that layer.
type Classifier struct {
	IP          IPLookup
	Resolver    Resolver
	Clock       clock.Clock
	ASN         ASNLookup
	TLSCert     TLSCertLookup
	RDAP        RDAPLookup
	ContentScan ContentScanLookup
	Wayback     WaybackLookup
	CTLog       CTLogLookup
}

// New returns a classifier with default dependencies.
func New(ip IPLookup, r Resolver, c clock.Clock) *Classifier {
	if c == nil {
		c = clock.Real()
	}
	return &Classifier{IP: ip, Resolver: r, Clock: c}
}

// Classify dispatches to the IP or host branch.
func (c *Classifier) Classify(ctx context.Context, input string) (*Result, error) {
	start := c.Clock.Now()
	if input == "" {
		return nil, ErrInvalidInput
	}
	if addr, err := netip.ParseAddr(input); err == nil {
		return c.classifyIP(addr, input, start)
	}
	if c.Resolver == nil {
		return nil, ErrInvalidInput
	}
	return c.classifyHost(ctx, input, start)
}

func (c *Classifier) classifyIP(addr netip.Addr, input string, start time.Time) (*Result, error) {
	addr = addr.Unmap()
	if cat := bogon.Match(addr); cat != bogon.CatPublic {
		return nil, ErrBogon
	}
	cc, meta, _ := c.IP.LookupIP(addr)

	r := &Result{
		Input:     input,
		Type:      "ip",
		Resolved:  []netip.Addr{addr},
		IPCountry: cc,
		Registry:  meta.Registry,
	}

	// ASN brand can override IP geo on raw IP input too.
	if c.ASN != nil {
		asNum, asOrg, _ := c.ASN.Lookup(addr)
		r.ASN, r.ASNOrg = asNum, asOrg
		if asOrg != "" {
			if brandCC, ok := asn.BrandCountry(asOrg); ok {
				r.ASNCountry = brandCC
				r.FinalCountry = brandCC
				r.Confidence = ConfHighASNBrand
				r.Reason = "asn brand " + brandCC
				if cc != "" && cc != brandCC {
					r.Reason += " overrides ip " + cc
				}
				r.LookupDuration = c.Clock.Now().Sub(start)
				return r, nil
			}
		}
	}

	if cc != "" {
		r.FinalCountry = cc
		r.Confidence = ConfIPOnly
		r.Reason = "raw ip input; rir lookup"
	} else {
		r.Confidence = ConfUnknown
		r.Reason = "no country signal"
	}
	r.LookupDuration = c.Clock.Now().Sub(start)
	return r, nil
}

// enrichApex tries TLS cert, content scan, RDAP, then Wayback against
// apex in that order. Returns (cc, src) where src ∈ {"tls", "scan",
// "rdap", "wayback"}; ("", "") on total miss.
func (c *Classifier) enrichApex(ctx context.Context, apex string) (string, string) {
	if c.TLSCert != nil {
		if cc, ok := c.TLSCert.Lookup(ctx, apex); ok {
			return cc, "tls"
		}
	}
	if c.CTLog != nil {
		if cc, ok := c.CTLog.Lookup(ctx, apex); ok {
			return cc, "ctlog"
		}
	}
	if c.ContentScan != nil {
		if cc, ok := c.ContentScan.Lookup(ctx, apex); ok {
			return cc, "scan"
		}
	}
	if c.RDAP != nil {
		if cc, ok := c.RDAP.Lookup(ctx, apex); ok {
			return cc, "rdap"
		}
	}
	if c.Wayback != nil {
		if cc, ok := c.Wayback.Lookup(ctx, apex); ok {
			return cc, "wayback"
		}
	}
	return "", ""
}

// classifyHost runs the early-exit ladder for hostname inputs.
//
// Ladder order (first confident answer wins):
//  1. ccTLD + IP agree → ConfHigh
//  2. ccTLD ≠ IP, ccTLD=ID → ConfMediumDomainIDOffshore
//  3. ccTLD ≠ IP, ccTLD other → ConfMediumDomainCCMismatch
//  4. Generic TLD → ASN brand (offline, µs)
//  5. Generic TLD → TLS cert Subject.C (online, ~200ms)
//  6. Generic TLD → RDAP registrant (online, ~500-2000ms)
//  7. Generic TLD + IP=ID → ConfMediumGenericTLDIDHost
//  8. IP-only fallback
func (c *Classifier) classifyHost(ctx context.Context, input string, start time.Time) (*Result, error) {
	domCC, domType, _ := domain.Country(input)
	addrs, resolveErr := c.Resolver.Resolve(ctx, input)
	dnsFailed := resolveErr != nil

	var firstPublic netip.Addr
	for _, a := range addrs {
		if bogon.Match(a) == bogon.CatPublic {
			firstPublic = a
			break
		}
	}

	var ipCC, registry string
	if firstPublic.IsValid() {
		cc, meta, _ := c.IP.LookupIP(firstPublic)
		ipCC, registry = cc, meta.Registry
	}

	r := &Result{
		Input:         input,
		Type:          "domain",
		Resolved:      addrs,
		DomainCountry: domCC,
		DomainSuffix:  domType,
		IPCountry:     ipCC,
		Registry:      registry,
	}
	finish := func() (*Result, error) {
		r.LookupDuration = c.Clock.Now().Sub(start)
		return r, nil
	}

	// Layer 1: ccTLD + IP agree → high, return.
	if domCC != "" && ipCC == domCC {
		r.FinalCountry = domCC
		r.Confidence = ConfHigh
		r.Reason = "domain " + domType + " matches ip country"
		return finish()
	}

	// Layer 2/3: ccTLD present but disagrees with IP.
	if domCC != "" && ipCC != "" && domCC != ipCC {
		r.FinalCountry = domCC
		if domCC == "ID" {
			r.Confidence = ConfMediumDomainIDOffshore
			r.Reason = "domain .id but host ip in " + ipCC
		} else {
			r.Confidence = ConfMediumDomainCCMismatch
			r.Reason = "domain " + domCC + " but host ip in " + ipCC
		}
		return finish()
	}

	// DNS failed: enrichment via RDAP still works (registry, not DNS).
	// TLS/content scan need DNS and are only tried on the apex if distinct.
	if dnsFailed {
		// 1. RDAP on the input itself — hits for typo'd but registered domains.
		if c.RDAP != nil {
			if cc, ok := c.RDAP.Lookup(ctx, input); ok {
				r.RegistrantCountry = cc
				r.FinalCountry = cc
				r.Confidence = ConfHighRDAPRegistrant
				r.Reason = "dns failed; rdap registrant " + cc
				return finish()
			}
		}
		// 2. Apex fallback — try full enrichment (TLS+scan+RDAP) on the
		//    registrable parent, useful when only the subdomain is wrong.
		if apex, err := domain.RegisteredDomain(input); err == nil && apex != "" && apex != input {
			if cc, src := c.enrichApex(ctx, apex); cc != "" {
				r.FinalCountry = cc
				switch src {
				case "tls":
					r.Confidence = ConfHighSSLCert
					r.CertCountry = cc
				case "ctlog":
					r.Confidence = ConfHighCTLog
					r.CTLogCountry = cc
				case "scan":
					r.Confidence = ConfHighContent
					r.ContentCountry = cc
				case "rdap":
					r.Confidence = ConfHighRDAPRegistrant
					r.RegistrantCountry = cc
				}
				r.Reason = "dns failed; apex " + apex + " " + src + " " + cc
				return finish()
			}
		}
		// 3. Wayback Machine — archived snapshot may identify origin even
		//    when the domain is expired or temporarily unreachable.
		if c.Wayback != nil {
			if cc, ok := c.Wayback.Lookup(ctx, input); ok {
				r.WaybackCountry = cc
				r.FinalCountry = cc
				r.Confidence = ConfMediumWayback
				r.Reason = "dns failed; wayback snapshot " + cc
				return finish()
			}
		}
		// 4. Fall back to ccTLD signal if present.
		if domCC != "" {
			r.FinalCountry = domCC
			r.Confidence = ConfLowDNSFailed
			r.Reason = "dns failed; used " + domType + " signal"
			return finish()
		}
		// 5. Last resort: surface Unknown with a proper row rather than
		//    aborting — callers batch-processing CSV need consistent output.
		r.Confidence = ConfUnknown
		r.Reason = "dns failed; no enrichment signal"
		return finish()
	}

	// From here we are in the generic/unknown-TLD path. Walk enrichment
	// sources cheapest-first and return on the first confident hit.

	// Layer 4: ASN brand (offline, µs).
	if c.ASN != nil && firstPublic.IsValid() {
		asNum, asOrg, _ := c.ASN.Lookup(firstPublic)
		r.ASN, r.ASNOrg = asNum, asOrg
		if asOrg != "" {
			if brandCC, ok := asn.BrandCountry(asOrg); ok {
				r.ASNCountry = brandCC
				r.FinalCountry = brandCC
				r.Confidence = ConfHighASNBrand
				r.Reason = "asn brand " + brandCC
				if ipCC != "" && ipCC != brandCC {
					r.Reason += " overrides ip " + ipCC
				}
				return finish()
			}
		}
	}

	// Layer 5: TLS cert Subject.Country (online, ~200ms).
	if c.TLSCert != nil {
		if certCC, ok := c.TLSCert.Lookup(ctx, input); ok {
			r.CertCountry = certCC
			r.FinalCountry = certCC
			r.Confidence = ConfHighSSLCert
			r.Reason = "tls cert subject " + certCC
			if ipCC != "" && ipCC != certCC {
				r.Reason += " overrides ip " + ipCC
			}
			return finish()
		}
	}

	// Layer 5b: CT log historical cert Subject.Country (crt.sh). Runs
	// after the live TLS handshake so current DV certs don't short-
	// circuit, but before content scan because CA-validated Subject.C
	// is a stronger signal than page heuristics.
	if c.CTLog != nil {
		if ctCC, ok := c.CTLog.Lookup(ctx, input); ok {
			r.CTLogCountry = ctCC
			r.FinalCountry = ctCC
			r.Confidence = ConfHighCTLog
			r.Reason = "ct log historical cert subject " + ctCC
			if ipCC != "" && ipCC != ctCC {
				r.Reason += " overrides ip " + ipCC
			}
			return finish()
		}
	}

	// Layer 6: HTML content scan (online, ~500-800ms). Runs before RDAP
	// because privacy-proxy registrars (Cloudflare, Domains-By-Proxy)
	// poison RDAP registrant data.
	if c.ContentScan != nil {
		if contentCC, ok := c.ContentScan.Lookup(ctx, input); ok {
			r.ContentCountry = contentCC
			r.FinalCountry = contentCC
			r.Confidence = ConfHighContent
			r.Reason = "content scan " + contentCC
			if ipCC != "" && ipCC != contentCC {
				r.Reason += " overrides ip " + ipCC
			}
			return finish()
		}
	}

	// Layer 7: RDAP registrant (online, slow).
	if c.RDAP != nil {
		if rdapCC, ok := c.RDAP.Lookup(ctx, input); ok {
			r.RegistrantCountry = rdapCC
			r.FinalCountry = rdapCC
			r.Confidence = ConfHighRDAPRegistrant
			r.Reason = "rdap registrant " + rdapCC
			if ipCC != "" && ipCC != rdapCC {
				r.Reason += " overrides ip " + ipCC
			}
			return finish()
		}
	}

	// Layer 7: Generic TLD + ID host heuristic (legacy tier).
	if domCC == "" && (domType == "generic" || domType == "") && ipCC == "ID" {
		r.FinalCountry = "ID"
		r.Confidence = ConfMediumGenericTLDIDHost
		r.Reason = "generic tld resolved to id host"
		return finish()
	}

	// Layer 8: apex fallback — input resolved but all enrichment missed.
	// Try the registrable parent (e.g., subdomain.example.com → example.com);
	// often its RDAP or TLS cert carries the origin country.
	if apex, err := domain.RegisteredDomain(input); err == nil && apex != "" && apex != input {
		if cc, src := c.enrichApex(ctx, apex); cc != "" {
			r.FinalCountry = cc
			switch src {
			case "tls":
				r.Confidence = ConfHighSSLCert
				r.CertCountry = cc
			case "ctlog":
				r.Confidence = ConfHighCTLog
				r.CTLogCountry = cc
			case "scan":
				r.Confidence = ConfHighContent
				r.ContentCountry = cc
			case "rdap":
				r.Confidence = ConfHighRDAPRegistrant
				r.RegistrantCountry = cc
			case "wayback":
				r.Confidence = ConfMediumWayback
				r.WaybackCountry = cc
			}
			r.Reason = "apex " + apex + " " + src + " " + cc
			if ipCC != "" && ipCC != cc {
				r.Reason += " overrides ip " + ipCC
			}
			return finish()
		}
	}

	// Layer 9: Wayback on input — historical content when live fetch
	// misses (JS-only pages, geofenced content, etc.).
	if c.Wayback != nil {
		if cc, ok := c.Wayback.Lookup(ctx, input); ok {
			r.WaybackCountry = cc
			r.FinalCountry = cc
			r.Confidence = ConfMediumWayback
			r.Reason = "wayback snapshot " + cc
			if ipCC != "" && ipCC != cc {
				r.Reason += " overrides ip " + ipCC
			}
			return finish()
		}
	}

	// Layer 10: Single-signal fallback.
	if domCC != "" && ipCC == "" {
		r.FinalCountry = domCC
		r.Confidence = ConfLowDNSFailed
		r.Reason = "domain " + domType + " signal only"
		return finish()
	}
	if ipCC != "" {
		r.FinalCountry = ipCC
		r.Confidence = ConfIPOnly
		r.Reason = "ip country only"
		return finish()
	}
	r.Confidence = ConfUnknown
	r.Reason = "no country signal"
	return finish()
}
