// Package classifier merges RIR IP lookups and domain-suffix signals into a
// single Result with a confidence tier.
package classifier

import (
	"context"
	"errors"
	"net/netip"
	"time"

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

// ASNLookup returns an ASN and org for an IP. Optional (pass nil to disable).
type ASNLookup interface {
	Lookup(ip netip.Addr) (asn uint32, org string, ok bool)
}

// Result is the full classification of a single input.
type Result struct {
	Input          string
	Type           string // "ip" | "domain"
	Resolved       []netip.Addr
	DomainCountry  string
	DomainSuffix   string
	IPCountry      string
	ASN            uint32
	ASNOrg         string
	Registry       string
	FinalCountry   string
	Confidence     string
	Reason         string
	LookupDuration time.Duration
}

// Classifier wires IP + domain + resolver lookups behind a single Classify call.
type Classifier struct {
	IP       IPLookup
	Resolver Resolver
	Clock    clock.Clock
	ASN      ASNLookup // optional; nil disables ASN enrichment
}

// New returns a classifier with default dependencies.
// Pass nil for resolver if domain resolution is not needed.
func New(ip IPLookup, r Resolver, c clock.Clock) *Classifier {
	if c == nil {
		c = clock.Real()
	}
	return &Classifier{IP: ip, Resolver: r, Clock: c}
}

// Classify inspects input (raw IP or hostname) and returns a Result.
// Returns ErrBogon if input is a raw IP in a reserved range.
// Returns ErrInvalidInput for empty/malformed inputs.
// Returns ErrUnresolvable if a hostname cannot be resolved and has no domain signal.
func (c *Classifier) Classify(ctx context.Context, input string) (*Result, error) {
	start := c.Clock.Now()
	if input == "" {
		return nil, ErrInvalidInput
	}

	// IP branch.
	if addr, err := netip.ParseAddr(input); err == nil {
		return c.classifyIP(addr, input, start)
	}

	// Host branch.
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
	var asNum uint32
	var asOrg string
	if c.ASN != nil {
		asNum, asOrg, _ = c.ASN.Lookup(addr)
	}
	dec := Decide("", "", cc, false, true)
	return &Result{
		Input:          input,
		Type:           "ip",
		Resolved:       []netip.Addr{addr},
		IPCountry:      cc,
		ASN:            asNum,
		ASNOrg:         asOrg,
		Registry:       meta.Registry,
		FinalCountry:   dec.FinalCountry,
		Confidence:     dec.Confidence,
		Reason:         dec.Reason,
		LookupDuration: c.Clock.Now().Sub(start),
	}, nil
}

func (c *Classifier) classifyHost(ctx context.Context, input string, start time.Time) (*Result, error) {
	domCC, domType, _ := domain.Country(input)

	addrs, resolveErr := c.Resolver.Resolve(ctx, input)
	dnsFailed := resolveErr != nil

	// Pick first non-bogon address.
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
		ipCC = cc
		registry = meta.Registry
	}

	if dnsFailed && domCC == "" {
		return nil, ErrUnresolvable
	}

	dec := Decide(domCC, domType, ipCC, dnsFailed, false)
	return &Result{
		Input:          input,
		Type:           "domain",
		Resolved:       addrs,
		DomainCountry:  domCC,
		DomainSuffix:   domType,
		IPCountry:      ipCC,
		Registry:       registry,
		FinalCountry:   dec.FinalCountry,
		Confidence:     dec.Confidence,
		Reason:         dec.Reason,
		LookupDuration: c.Clock.Now().Sub(start),
	}, nil
}
