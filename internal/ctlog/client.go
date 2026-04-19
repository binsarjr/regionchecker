// Package ctlog queries Certificate Transparency via crt.sh for the
// Subject.Country of historical certificates issued to a domain.
// Catches brands that previously held OV/EV certs (CA-validated
// country) even if the current live cert is DV (Let's Encrypt /
// Google Trust / Cloudflare Universal SSL).
package ctlog

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Client fetches CT log cert subjects via crt.sh.
type Client struct {
	HTTP      *http.Client
	Cache     Cache
	MaxCerts  int           // limit per-domain cert fetches
	Timeout   time.Duration // total budget per Lookup
	UserAgent string
}

// Cache persists results. Negative hits cached as empty cc.
type Cache interface {
	Get(host string) (cc string, ok bool)
	Put(host string, cc string)
}

// NewClient returns a Client with conservative defaults (10 certs,
// 8s total timeout).
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 8 * time.Second},
		MaxCerts:  10,
		Timeout:   8 * time.Second,
		UserAgent: "regionchecker/0.2 (+https://github.com/binsarjr/regionchecker)",
	}
}

// Lookup returns the first non-empty Subject.Country from historical
// certs for host. Returns ("", false) when all certs are DV or crt.sh
// is unavailable.
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
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	ids := c.queryIDs(ctx, host)
	if len(ids) == 0 {
		return ""
	}
	limit := c.MaxCerts
	if limit <= 0 {
		limit = 10
	}
	// Older cert IDs (smaller numbers) are more likely to be OV/EV
	// from before the DV-dominance era; prefer them.
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	for _, id := range ids {
		if cc := c.fetchSubjectCountry(ctx, id); cc != "" {
			return cc
		}
		if ctx.Err() != nil {
			return ""
		}
	}
	return ""
}

// crtEntry is the subset of crt.sh JSON we need.
type crtEntry struct {
	ID int64 `json:"id"`
}

func (c *Client) queryIDs(ctx context.Context, host string) []int64 {
	u := "https://crt.sh/?q=" + host + "&output=json&exclude=expired"
	body := c.get(ctx, u, 1<<20)
	if len(body) == 0 {
		return nil
	}
	var entries []crtEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil
	}
	out := make([]int64, 0, len(entries))
	seen := make(map[int64]struct{}, len(entries))
	for _, e := range entries {
		if e.ID == 0 {
			continue
		}
		if _, ok := seen[e.ID]; ok {
			continue
		}
		seen[e.ID] = struct{}{}
		out = append(out, e.ID)
	}
	return out
}

// fetchSubjectCountry downloads the PEM cert and returns the first
// non-empty Subject.Country, uppercase alpha-2.
func (c *Client) fetchSubjectCountry(ctx context.Context, id int64) string {
	u := "https://crt.sh/?d=" + itoa(id)
	body := c.get(ctx, u, 64<<10)
	if len(body) == 0 {
		return ""
	}
	block, _ := pem.Decode(body)
	if block == nil {
		return ""
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return ""
	}
	for _, cc := range cert.Subject.Country {
		cc = strings.ToUpper(strings.TrimSpace(cc))
		if len(cc) == 2 {
			return cc
		}
	}
	return ""
}

func (c *Client) get(ctx context.Context, url string, limit int64) []byte {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil
	}
	return data
}

func itoa(n int64) string {
	// strconv.FormatInt without import bloat
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
