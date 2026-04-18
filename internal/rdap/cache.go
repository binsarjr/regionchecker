package rdap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DiskCache stores RDAP results as JSON files under dir/<sha>.json.
// Empty country values are cached as negative results to avoid repeated
// redacted lookups. TTL is applied on read.
type DiskCache struct {
	dir string
	ttl time.Duration
	mu  sync.Mutex
}

type cacheEntry struct {
	Domain    string    `json:"domain"`
	Country   string    `json:"country"`
	FetchedAt time.Time `json:"fetched_at"`
}

// NewDiskCache returns a DiskCache rooted at dir with the given TTL.
// dir is created if missing.
func NewDiskCache(dir string, ttl time.Duration) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &DiskCache{dir: dir, ttl: ttl}, nil
}

// Get returns (cc, true) when a fresh cache entry exists for domain.
// Returns ("", true) for a fresh negative (redacted) result, signalling
// the caller not to re-fetch. Returns ("", false) for miss or expired.
func (c *DiskCache) Get(domain string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := os.ReadFile(c.path(domain))
	if err != nil {
		return "", false
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return "", false
	}
	if time.Since(e.FetchedAt) > c.ttl {
		return "", false
	}
	return e.Country, true
}

// Put writes the entry for domain.
func (c *DiskCache) Put(domain string, cc string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := json.Marshal(cacheEntry{
		Domain:    domain,
		Country:   cc,
		FetchedAt: time.Now(),
	})
	if err != nil {
		return
	}
	_ = os.WriteFile(c.path(domain), data, 0o644)
}

func (c *DiskCache) path(domain string) string {
	sum := sha256.Sum256([]byte(domain))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}
