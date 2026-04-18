package asn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// ErrNotFound indicates a Cymru lookup returned no TXT record.
var ErrNotFound = errors.New("asn: cymru no result")

// CymruClient queries Team Cymru's IP→ASN TXT DNS service.
// Zone: *.origin.asn.cymru.com for IPv4, *.origin6.asn.cymru.com for IPv6.
//
// Response TXT format (pipe-separated):
//
//	"15169 | 8.8.8.0/24 | US | arin | 1992-12-01"
type CymruClient struct {
	Resolver *net.Resolver
	Timeout  time.Duration
}

// NewCymruClient returns a client using the system resolver and a 3s timeout.
func NewCymruClient() *CymruClient {
	return &CymruClient{Resolver: net.DefaultResolver, Timeout: 3 * time.Second}
}

// ASNResult is the parsed Cymru TXT record.
type ASNResult struct {
	ASN      uint32
	Prefix   string
	Country  string
	Registry string
	Date     string
}

// Lookup resolves ip to an ASNResult via Cymru DNS TXT.
func (c *CymruClient) Lookup(ctx context.Context, ip netip.Addr) (ASNResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	q := cymruQueryName(ip)
	if q == "" {
		return ASNResult{}, fmt.Errorf("asn: unsupported addr %s", ip)
	}
	records, err := c.Resolver.LookupTXT(ctx, q)
	if err != nil {
		return ASNResult{}, ErrNotFound
	}
	if len(records) == 0 {
		return ASNResult{}, ErrNotFound
	}
	return parseCymruTXT(records[0])
}

// cymruQueryName returns the DNS name to query for ip.
func cymruQueryName(ip netip.Addr) string {
	ip = ip.Unmap()
	if ip.Is4() {
		b := ip.As4()
		return fmt.Sprintf("%d.%d.%d.%d.origin.asn.cymru.com",
			b[3], b[2], b[1], b[0])
	}
	if ip.Is6() {
		b := ip.As16()
		var parts []string
		for i := 15; i >= 0; i-- {
			parts = append(parts, fmt.Sprintf("%x", b[i]&0x0f), fmt.Sprintf("%x", b[i]>>4))
		}
		return strings.Join(parts, ".") + ".origin6.asn.cymru.com"
	}
	return ""
}

// parseCymruTXT parses a pipe-separated Cymru TXT response.
func parseCymruTXT(s string) (ASNResult, error) {
	fields := strings.Split(s, "|")
	if len(fields) < 5 {
		return ASNResult{}, fmt.Errorf("asn: short cymru record %q", s)
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	// ASN field may contain multiple origins; take the first.
	asnField := strings.Fields(fields[0])
	if len(asnField) == 0 {
		return ASNResult{}, fmt.Errorf("asn: empty asn in %q", s)
	}
	asn, err := strconv.ParseUint(asnField[0], 10, 32)
	if err != nil {
		return ASNResult{}, fmt.Errorf("asn: parse %q: %w", asnField[0], err)
	}
	return ASNResult{
		ASN:      uint32(asn),
		Prefix:   fields[1],
		Country:  fields[2],
		Registry: fields[3],
		Date:     fields[4],
	}, nil
}
