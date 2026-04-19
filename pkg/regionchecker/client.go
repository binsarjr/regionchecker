// Package regionchecker is the public library facade for the regionchecker
// project. Downstream consumers depend only on this package; internal
// subpackages are not part of the stable surface.
package regionchecker

import (
	"context"
	"net/netip"
	"time"

	"github.com/binsarjr/regionchecker/internal/classifier"
	"github.com/binsarjr/regionchecker/internal/clock"
	"github.com/binsarjr/regionchecker/internal/rir"
)

// Re-exported sentinel errors. Callers should use errors.Is.
var (
	ErrBogon          = classifier.ErrBogon
	ErrUnresolvable   = classifier.ErrUnresolvable
	ErrInvalidInput   = classifier.ErrInvalidInput
	ErrUnknownCountry = classifier.ErrUnknownCountry
)

// Result mirrors classifier.Result for stable public use.
type Result struct {
	Input             string
	Type              string
	Resolved          []netip.Addr
	DomainCountry     string
	DomainSuffix      string
	IPCountry         string
	ASN               uint32
	ASNOrg            string
	ASNCountry        string
	CertCountry       string
	CTLogCountry      string
	RegistrantCountry string
	ContentCountry    string
	WaybackCountry    string
	Registry          string
	FinalCountry      string
	Confidence        string
	Reason            string
	LookupDuration    time.Duration
}

// IPLookup resolves a country code from an IP address.
type IPLookup interface {
	LookupIP(ip netip.Addr) (cc string, meta rir.Meta, ok bool)
}

// Resolver resolves a hostname to a list of addresses.
type Resolver interface {
	Resolve(ctx context.Context, host string) ([]netip.Addr, error)
}

// Client is the public entry point for classification.
type Client struct {
	c *classifier.Classifier
}

// New wires an IP lookup, a DNS resolver, and a Clock (optional) into a Client.
// Pass nil for resolver to classify raw IPs only.
func New(ip IPLookup, r Resolver, clk clock.Clock) *Client {
	return &Client{c: classifier.New(ip, r, clk)}
}

// Classify inspects input (raw IP or hostname) and returns a Result.
func (c *Client) Classify(ctx context.Context, input string) (*Result, error) {
	r, err := c.c.Classify(ctx, input)
	if err != nil {
		return nil, err
	}
	return (*Result)(r), nil
}
