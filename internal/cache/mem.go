package cache

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// LRU is a type-safe wrapper around hashicorp/golang-lru/v2 with optional
// per-entry TTL. Zero TTL disables expiry.
type LRU[K comparable, V any] struct {
	c   *lru.Cache[K, entry[V]]
	ttl time.Duration
	now func() time.Time
	mu  sync.Mutex
}

type entry[V any] struct {
	v       V
	expires time.Time
}

// NewLRU constructs an LRU with the given size and TTL.
func NewLRU[K comparable, V any](size int, ttl time.Duration) (*LRU[K, V], error) {
	c, err := lru.New[K, entry[V]](size)
	if err != nil {
		return nil, err
	}
	return &LRU[K, V]{c: c, ttl: ttl, now: time.Now}, nil
}

// Get returns the cached value for k if present and not expired.
func (l *LRU[K, V]) Get(k K) (V, bool) {
	var zero V
	e, ok := l.c.Get(k)
	if !ok {
		return zero, false
	}
	if !e.expires.IsZero() && l.now().After(e.expires) {
		l.c.Remove(k)
		return zero, false
	}
	return e.v, true
}

// Put stores v under k with the configured TTL.
func (l *LRU[K, V]) Put(k K, v V) {
	e := entry[V]{v: v}
	if l.ttl > 0 {
		e.expires = l.now().Add(l.ttl)
	}
	l.c.Add(k, e)
}

// Len returns the number of items currently in the cache (including expired
// entries that have not yet been evicted).
func (l *LRU[K, V]) Len() int { return l.c.Len() }

// Remove drops k from the cache.
func (l *LRU[K, V]) Remove(k K) { l.c.Remove(k) }

// Purge evicts all entries.
func (l *LRU[K, V]) Purge() { l.c.Purge() }

// SetClock overrides the time source. Intended for tests.
func (l *LRU[K, V]) SetClock(now func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now == nil {
		now = time.Now
	}
	l.now = now
}
