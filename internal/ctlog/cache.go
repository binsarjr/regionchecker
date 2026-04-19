package ctlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DiskCache persists crt.sh results with TTL. Negative hits cached to
// avoid hammering crt.sh on every invocation.
type DiskCache struct {
	dir string
	ttl time.Duration
	mu  sync.Mutex
}

type diskEntry struct {
	Host      string    `json:"host"`
	Country   string    `json:"country"`
	FetchedAt time.Time `json:"fetched_at"`
}

// NewDiskCache creates a cache at dir.
func NewDiskCache(dir string, ttl time.Duration) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &DiskCache{dir: dir, ttl: ttl}, nil
}

// Get returns (cc, true) for a fresh entry.
func (c *DiskCache) Get(host string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := os.ReadFile(c.path(host))
	if err != nil {
		return "", false
	}
	var e diskEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return "", false
	}
	if time.Since(e.FetchedAt) > c.ttl {
		return "", false
	}
	return e.Country, true
}

// Put writes an entry.
func (c *DiskCache) Put(host string, cc string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := json.Marshal(diskEntry{Host: host, Country: cc, FetchedAt: time.Now()})
	if err != nil {
		return
	}
	_ = os.WriteFile(c.path(host), data, 0o644)
}

func (c *DiskCache) path(host string) string {
	sum := sha256.Sum256([]byte(host))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}
