// Package wayback queries the Internet Archive's Wayback Machine for
// the nearest archived snapshot of a host, fetches the raw archived
// HTML, and scores it against a set of content-scan detectors. Useful
// for expired/unreachable domains that historically had identifying
// content (language, addresses, legal entity markers) in their pages.
package wayback

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/binsarjr/regionchecker/internal/contentscan"
)

// Client fetches Wayback snapshots and scores them.
type Client struct {
	HTTP      *http.Client
	Detectors []contentscan.Detector
	Cache     Cache
	Threshold int
	Timeout   time.Duration
	UserAgent string
	MaxBody   int64
}

// Cache persists wayback results. Empty string caches a negative.
type Cache interface {
	Get(host string) (cc string, ok bool)
	Put(host string, cc string)
}

// NewClient returns a Client using the default content-scan detectors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 8 * time.Second},
		Detectors: contentscan.DefaultDetectors(),
		Threshold: 3,
		Timeout:   8 * time.Second,
		UserAgent: "regionchecker/0.2 (+https://github.com/binsarjr/regionchecker)",
		MaxBody:   512 << 10,
	}
}

// Lookup returns the detector-scored country of the nearest archived
// snapshot for host. Returns ("", false) when the host was never
// archived, the snapshot is unreachable, or scores fall below threshold.
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
	cc := c.lookup(ctx, host)
	if c.Cache != nil {
		c.Cache.Put(host, cc)
	}
	return cc, cc != ""
}

func (c *Client) lookup(ctx context.Context, host string) string {
	snap := c.findSnapshot(ctx, "http://"+host)
	if snap == "" {
		snap = c.findSnapshot(ctx, "https://"+host)
	}
	if snap == "" {
		return ""
	}
	// Rewrite to id_ variant for raw archived body (no Wayback chrome).
	raw := rawSnapshotURL(snap)
	body := c.fetchBody(ctx, raw)
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

// availabilityResp is the /wayback/available JSON envelope.
type availabilityResp struct {
	ArchivedSnapshots struct {
		Closest struct {
			Available bool   `json:"available"`
			URL       string `json:"url"`
			Status    string `json:"status"`
		} `json:"closest"`
	} `json:"archived_snapshots"`
}

func (c *Client) findSnapshot(ctx context.Context, target string) string {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	u := "https://archive.org/wayback/available?url=" + target
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ""
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var av availabilityResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<10)).Decode(&av); err != nil {
		return ""
	}
	if !av.ArchivedSnapshots.Closest.Available {
		return ""
	}
	if av.ArchivedSnapshots.Closest.Status != "" && !strings.HasPrefix(av.ArchivedSnapshots.Closest.Status, "2") {
		return ""
	}
	return av.ArchivedSnapshots.Closest.URL
}

// rawSnapshotURL rewrites https://web.archive.org/web/<ts>/<url>
// into https://web.archive.org/web/<ts>id_/<url> — the id_ suffix
// returns the archived body without Wayback's toolbar injection.
func rawSnapshotURL(u string) string {
	const marker = "/web/"
	idx := strings.Index(u, marker)
	if idx < 0 {
		return u
	}
	tail := u[idx+len(marker):]
	slash := strings.IndexByte(tail, '/')
	if slash < 0 {
		return u
	}
	ts := tail[:slash]
	rest := tail[slash:]
	if strings.HasSuffix(ts, "id_") {
		return u
	}
	return u[:idx+len(marker)] + ts + "id_" + rest
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
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
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
