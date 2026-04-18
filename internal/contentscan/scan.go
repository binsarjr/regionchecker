// Package contentscan fetches a site's HTML and scores it against a
// set of country-specific detectors. Useful as a last-resort signal
// when the technical surface (IP/ASN/TLS/RDAP) is obscured by CDNs
// (Cloudflare, Fastly) and privacy-proxy registrars.
//
// Each Detector inspects the page body and returns a score; the
// highest score above the Client threshold wins. Detectors for
// ID/SG/MY/US/GB/JP are registered by default via DefaultDetectors.
package contentscan

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Detector inspects a page body for a single country's markers.
type Detector interface {
	// CC returns the ISO 3166 alpha-2 country code this detector targets.
	CC() string
	// Score returns a non-negative score for the page body. Higher means
	// stronger evidence for CC. Typical: 0 for unrelated, 5+ for strong hit.
	Score(body string) int
}

// Cache persists scan results. Empty string caches a negative result.
type Cache interface {
	Get(host string) (cc string, ok bool)
	Put(host string, cc string)
}

// Client runs HTTP content scans across registered detectors.
type Client struct {
	HTTP      *http.Client
	Detectors []Detector
	Cache     Cache
	Threshold int
	UserAgent string
	MaxBody   int64
	Timeout   time.Duration
}

// NewClient returns a Client with the default detector set, a 4s
// timeout, 512KB body cap, a browser-like UA, and a threshold of 3.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 4 * time.Second},
		Detectors: DefaultDetectors(),
		Threshold: 3,
		UserAgent: "Mozilla/5.0 (compatible; regionchecker/0.1; +https://github.com/binsarjr/regionchecker)",
		MaxBody:   512 << 10,
		Timeout:   4 * time.Second,
	}
}

// Lookup fetches https://host (then http:// on TLS failure), scores
// the body across detectors, and returns the highest-scoring CC when
// score ≥ Threshold. Returns ("", false) for fetch errors, blocked
// pages, or scores below threshold.
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
	cc := c.scan(ctx, host)
	if c.Cache != nil {
		c.Cache.Put(host, cc)
	}
	return cc, cc != ""
}

func (c *Client) scan(ctx context.Context, host string) string {
	body := c.fetchBody(ctx, "https://"+host)
	if body == "" {
		body = c.fetchBody(ctx, "http://"+host)
	}
	if body == "" {
		return ""
	}
	best, bestScore := "", 0
	for _, d := range c.Detectors {
		s := d.Score(body)
		if s > bestScore {
			best, bestScore = d.CC(), s
		}
	}
	if bestScore < c.Threshold {
		return ""
	}
	return best
}

func (c *Client) fetchBody(ctx context.Context, url string) string {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en,id;q=0.9,*;q=0.5")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ""
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "html") && !strings.Contains(strings.ToLower(ct), "xml") && !strings.Contains(strings.ToLower(ct), "text/plain") {
		return ""
	}
	limit := c.MaxBody
	if limit <= 0 {
		limit = 512 << 10
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return ""
	}
	return string(data)
}

// RegisterDetector adds d to the defaults list. Safe for init-time use.
func RegisterDetector(d Detector) {
	defaultsMu.Lock()
	defer defaultsMu.Unlock()
	defaults = append(defaults, d)
}

// DefaultDetectors returns a copy of the registered detector list.
func DefaultDetectors() []Detector {
	defaultsMu.Lock()
	defer defaultsMu.Unlock()
	out := make([]Detector, len(defaults))
	copy(out, defaults)
	return out
}

var (
	defaultsMu sync.Mutex
	defaults   []Detector
)

// Debug returns a per-detector score breakdown. Useful for tuning.
func (c *Client) Debug(ctx context.Context, host string) (string, map[string]int) {
	body := c.fetchBody(ctx, "https://"+host)
	if body == "" {
		body = c.fetchBody(ctx, "http://"+host)
	}
	scores := make(map[string]int, len(c.Detectors))
	for _, d := range c.Detectors {
		scores[d.CC()] = d.Score(body)
	}
	best, bestScore := "", 0
	for cc, s := range scores {
		if s > bestScore {
			best, bestScore = cc, s
		}
	}
	if bestScore < c.Threshold {
		best = ""
	}
	return best, scores
}

// FetchError is returned by the test harness to surface diagnostic
// information without leaking it to callers.
type FetchError struct{ inner error }

func (e *FetchError) Error() string { return fmt.Sprintf("contentscan: %v", e.inner) }
