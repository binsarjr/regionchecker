// Package rdap implements a minimal RDAP (RFC 9082/9083) client for
// resolving domain registrant country codes. Designed as an optional,
// online-only enrichment signal gated behind --rdap or --all flags.
package rdap

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed dns.json
var ianaBootstrap []byte

// Bootstrap maps TLDs to RDAP service base URLs per IANA's dns.json.
type Bootstrap struct {
	byTLD map[string]string
}

// LoadBootstrap parses the embedded IANA bootstrap snapshot.
func LoadBootstrap() (*Bootstrap, error) {
	return parseBootstrap(ianaBootstrap)
}

// parseBootstrap decodes the IANA services array into a TLD→URL map.
//
// Bootstrap format:
//
//	{"services": [ [ ["com","net"], ["https://.../"] ], ... ]}
func parseBootstrap(raw []byte) (*Bootstrap, error) {
	var doc struct {
		Services [][][]string `json:"services"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("rdap: parse bootstrap: %w", err)
	}
	b := &Bootstrap{byTLD: make(map[string]string, 512)}
	for _, svc := range doc.Services {
		if len(svc) < 2 {
			continue
		}
		tlds, urls := svc[0], svc[1]
		if len(urls) == 0 {
			continue
		}
		base := strings.TrimRight(urls[0], "/") + "/"
		for _, tld := range tlds {
			b.byTLD[strings.ToLower(tld)] = base
		}
	}
	return b, nil
}

// BaseURL returns the RDAP service base for tld (e.g. "com" → "https://rdap.verisign.com/com/v1/"),
// or "" when no mapping exists.
func (b *Bootstrap) BaseURL(tld string) string {
	if b == nil {
		return ""
	}
	return b.byTLD[strings.ToLower(strings.TrimPrefix(tld, "."))]
}

// tldOf returns the rightmost label of domain, lowercased.
func tldOf(domain string) string {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if i := strings.LastIndex(domain, "."); i >= 0 {
		return domain[i+1:]
	}
	return domain
}
