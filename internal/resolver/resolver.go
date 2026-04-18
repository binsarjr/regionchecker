package resolver

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

var (
	ErrUnresolvable = errors.New("regionchecker: host unresolvable")
	ErrInvalidInput = errors.New("regionchecker: invalid ip or host")
)

const (
	cacheSize = 10000
	cacheTTL  = 5 * time.Minute
)

type cacheEntry struct {
	addrs   []netip.Addr
	expires time.Time
}

// Resolver resolves hostnames to IP addresses with an optional custom DNS
// server list, per-lookup timeout, and an in-memory LRU cache.
type Resolver struct {
	r       *net.Resolver
	timeout time.Duration
	cache   *lru.Cache[string, cacheEntry]
}

// New creates a Resolver. timeout is applied per-lookup (0 = no timeout).
// servers optionally overrides the system DNS (e.g. []string{"8.8.8.8:53"}).
// Pass nil or an empty slice to use the system resolver.
func New(timeout time.Duration, servers []string) *Resolver {
	c, _ := lru.New[string, cacheEntry](cacheSize)

	nr := &net.Resolver{}
	if len(servers) > 0 {
		idx := 0
		nr = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				srv := servers[idx%len(servers)]
				idx++
				return d.DialContext(ctx, "udp", srv)
			},
		}
	}

	return &Resolver{r: nr, timeout: timeout, cache: c}
}

// Resolve returns all IPv4+IPv6 addresses for host.
// Strips a trailing dot. Rejects hosts longer than 253 chars (ErrInvalidInput).
// Unmaps IPv4-in-IPv6 addresses. Returns ErrUnresolvable when the host cannot
// be resolved or yields no addresses.
func (r *Resolver) Resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if host == "" {
		return nil, ErrInvalidInput
	}

	host = strings.TrimSuffix(host, ".")

	if len(host) > 253 {
		return nil, ErrInvalidInput
	}

	if host == "" {
		return nil, ErrInvalidInput
	}

	// Check cache.
	if e, ok := r.cache.Get(host); ok {
		if time.Now().Before(e.expires) {
			return e.addrs, nil
		}
		r.cache.Remove(host)
	}

	lookupCtx := ctx
	var cancel context.CancelFunc
	if r.timeout > 0 {
		lookupCtx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	addrs, err := r.r.LookupNetIP(lookupCtx, "ip", host)
	if err != nil {
		return nil, ErrUnresolvable
	}
	if len(addrs) == 0 {
		return nil, ErrUnresolvable
	}

	result := make([]netip.Addr, len(addrs))
	for i, a := range addrs {
		result[i] = a.Unmap()
	}

	r.cache.Add(host, cacheEntry{
		addrs:   result,
		expires: time.Now().Add(cacheTTL),
	})

	return result, nil
}
