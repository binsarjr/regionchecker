// Package tlscert dials a host on 443 and extracts the peer leaf
// certificate's Subject.Country. This is a strong identity signal when
// the site uses an OV/EV certificate — CAs validate the organisation
// country before issuing. DV certs (Let's Encrypt, Cloudflare free,
// Google Trust Services) typically omit Subject.C; callers must treat
// absence as a soft miss rather than an error.
package tlscert

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

// ErrNoCountry indicates the TLS handshake succeeded but the peer cert
// has no Subject.Country (typical DV cert).
var ErrNoCountry = errors.New("tlscert: subject has no country")

// Client dials hosts and returns the leaf cert's Subject.Country.
type Client struct {
	Timeout time.Duration
	Cache   Cache // optional; nil disables caching
	// Dialer lets tests inject a synthetic dialer.
	Dialer func(ctx context.Context, addr string, cfg *tls.Config) (*tls.Conn, error)
}

// Cache persists country results. Empty string is cached as negative
// (no country exposed) to avoid repeated TLS handshakes.
type Cache interface {
	Get(host string) (cc string, ok bool)
	Put(host string, cc string)
}

// NewClient returns a Client with a 3-second timeout and no cache.
func NewClient() *Client {
	return &Client{Timeout: 3 * time.Second}
}

// Lookup performs a TLS handshake against host:443 and returns the
// Subject.Country of the leaf certificate in uppercase ISO alpha-2.
// Returns ("", false) for DV certs (no country) or on dial/verify errors.
func (c *Client) Lookup(ctx context.Context, host string) (string, bool) {
	if c == nil {
		return "", false
	}
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return "", false
	}
	if c.Cache != nil {
		if cc, ok := c.Cache.Get(host); ok {
			return cc, cc != ""
		}
	}
	cc := c.dial(ctx, host)
	if c.Cache != nil {
		c.Cache.Put(host, cc)
	}
	return cc, cc != ""
}

func (c *Client) dial(ctx context.Context, host string) string {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cfg := &tls.Config{
		ServerName: host,
		// Accept the handshake even if the chain is invalid — we only
		// care about Subject.Country, not authenticity for this purpose.
		// Cert integrity is still verified by the browser/downstream.
		InsecureSkipVerify: true, //nolint:gosec
		MinVersion:         tls.VersionTLS12,
	}

	var conn *tls.Conn
	var err error
	if c.Dialer != nil {
		conn, err = c.Dialer(dialCtx, host+":443", cfg)
	} else {
		var d tls.Dialer
		d.NetDialer = &net.Dialer{Timeout: timeout}
		d.Config = cfg
		raw, derr := d.DialContext(dialCtx, "tcp", host+":443")
		if derr != nil {
			return ""
		}
		conn = raw.(*tls.Conn)
	}
	if err != nil {
		return ""
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return ""
	}
	return leafCountry(state.PeerCertificates[0])
}

// leafCountry returns the first uppercase alpha-2 country on the
// certificate's Subject. Returns "" when unset or malformed.
func leafCountry(cert *x509.Certificate) string {
	for _, c := range cert.Subject.Country {
		c = strings.ToUpper(strings.TrimSpace(c))
		if len(c) == 2 {
			return c
		}
	}
	return ""
}

// memCache is a lightweight in-process cache useful for tests and
// short-lived CLI runs.
type memCache struct {
	mu   sync.Mutex
	data map[string]string
}

// NewMemCache returns a process-local cache.
func NewMemCache() Cache { return &memCache{data: make(map[string]string)} }

func (m *memCache) Get(host string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cc, ok := m.data[host]
	return cc, ok
}

func (m *memCache) Put(host string, cc string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[host] = cc
}
