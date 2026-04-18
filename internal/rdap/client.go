package rdap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotFound indicates the RDAP server returned 404 for the domain.
var ErrNotFound = errors.New("rdap: domain not found")

// ErrRedacted indicates the response was served but the registrant
// country was not disclosed (GDPR / ICANN privacy).
var ErrRedacted = errors.New("rdap: registrant redacted")

// ErrNoBootstrap indicates the TLD has no IANA RDAP service mapping.
var ErrNoBootstrap = errors.New("rdap: no bootstrap for tld")

// Client is a minimal RDAP domain-lookup client.
type Client struct {
	HTTP      *http.Client
	Bootstrap *Bootstrap
	Cache     Cache   // optional
	UserAgent string
	Timeout   time.Duration
}

// Cache is the persistence interface used by Client. Filesystem
// implementation lives in cache.go; tests may supply an in-memory stub.
type Cache interface {
	Get(domain string) (cc string, ok bool)
	Put(domain string, cc string)
}

// NewClient returns a Client using the embedded IANA bootstrap, a 3s
// timeout, and no cache. Callers may override any field.
func NewClient() (*Client, error) {
	b, err := LoadBootstrap()
	if err != nil {
		return nil, err
	}
	return &Client{
		HTTP:      &http.Client{Timeout: 3 * time.Second},
		Bootstrap: b,
		UserAgent: "regionchecker/0.1 (+https://github.com/binsarjr/regionchecker)",
		Timeout:   3 * time.Second,
	}, nil
}

// Lookup resolves the registrant country code for domain. Returns ("", false)
// when redacted or unavailable; callers should treat absence as non-fatal.
func (c *Client) Lookup(ctx context.Context, domain string) (string, bool) {
	if c == nil || c.Bootstrap == nil {
		return "", false
	}
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if domain == "" {
		return "", false
	}
	if c.Cache != nil {
		if cc, ok := c.Cache.Get(domain); ok {
			return cc, cc != ""
		}
	}
	cc, _ := c.fetch(ctx, domain)
	if c.Cache != nil {
		c.Cache.Put(domain, cc)
	}
	return cc, cc != ""
}

func (c *Client) fetch(ctx context.Context, domain string) (string, error) {
	tld := tldOf(domain)
	base := c.Bootstrap.BaseURL(tld)
	if base == "" {
		return "", ErrNoBootstrap
	}
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	body, err := c.get(ctx, base+"domain/"+domain)
	if err != nil {
		return "", err
	}
	if info, ok := extractRegistrant(body); ok {
		return info.Country, nil
	}
	// Follow "related" RDAP link — gTLDs (e.g. .com) expose only registrar
	// at the registry level; registrant data lives at the registrar's RDAP.
	if follow := extractRelatedRDAP(body); follow != "" {
		body2, err2 := c.get(ctx, follow)
		if err2 == nil {
			if info, ok := extractRegistrant(body2); ok {
				return info.Country, nil
			}
		}
	}
	return "", ErrRedacted
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/rdap+json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rdap: %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
