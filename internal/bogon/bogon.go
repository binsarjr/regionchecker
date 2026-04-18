// Package bogon classifies IP addresses that belong to reserved, private,
// link-local, loopback, CGNAT, documentation, benchmark, or multicast ranges.
// These ranges must be filtered before RIR country lookup.
package bogon

import (
	"net/netip"
	"sort"
)

// Category describes a bogon classification.
type Category string

const (
	CatPublic        Category = ""
	CatPrivate       Category = "private"
	CatLoopback      Category = "loopback"
	CatLinkLocal     Category = "link-local"
	CatCGNAT         Category = "cgnat"
	CatMulticast     Category = "multicast"
	CatReserved      Category = "reserved"
	CatDocumentation Category = "documentation"
	CatBenchmark     Category = "benchmark"
	CatBroadcast     Category = "broadcast"
	CatUnspecified   Category = "unspecified"
	CatULA           Category = "ula"
	CatDiscard       Category = "discard"
	CatTeredo        Category = "teredo"
	CatV4Mapped      Category = "v4-mapped"
)

type entry struct {
	prefix netip.Prefix
	cat    Category
}

// prefixes contains IPv4 and IPv6 reserved ranges. Order matters only for
// overlapping specificity (e.g. 127.0.0.1 inside 127.0.0.0/8); Contains()
// walks longest-first for deterministic results.
var prefixes []entry

func init() {
	defs := []struct {
		cidr string
		cat  Category
	}{
		// IPv4
		{"0.0.0.0/8", CatUnspecified},
		{"10.0.0.0/8", CatPrivate},
		{"100.64.0.0/10", CatCGNAT},
		{"127.0.0.0/8", CatLoopback},
		{"169.254.0.0/16", CatLinkLocal},
		{"172.16.0.0/12", CatPrivate},
		{"192.0.0.0/24", CatReserved},
		{"192.0.2.0/24", CatDocumentation},
		{"192.88.99.0/24", CatReserved},
		{"192.168.0.0/16", CatPrivate},
		{"198.18.0.0/15", CatBenchmark},
		{"198.51.100.0/24", CatDocumentation},
		{"203.0.113.0/24", CatDocumentation},
		{"224.0.0.0/4", CatMulticast},
		{"240.0.0.0/4", CatReserved},
		{"255.255.255.255/32", CatBroadcast},
		// IPv6
		{"::/128", CatUnspecified},
		{"::1/128", CatLoopback},
		{"::ffff:0:0/96", CatV4Mapped},
		{"64:ff9b::/96", CatReserved},
		{"100::/64", CatDiscard},
		{"2001::/32", CatTeredo},
		{"2001:db8::/32", CatDocumentation},
		{"fc00::/7", CatULA},
		{"fe80::/10", CatLinkLocal},
		{"ff00::/8", CatMulticast},
	}
	prefixes = make([]entry, 0, len(defs))
	for _, d := range defs {
		p, err := netip.ParsePrefix(d.cidr)
		if err != nil {
			panic("bogon: invalid internal prefix " + d.cidr + ": " + err.Error())
		}
		prefixes = append(prefixes, entry{prefix: p, cat: d.cat})
	}
	// Longest-prefix first so specific ranges win over generic supernets
	// (e.g. 255.255.255.255/32 vs 240.0.0.0/4).
	sort.SliceStable(prefixes, func(i, j int) bool {
		return prefixes[i].prefix.Bits() > prefixes[j].prefix.Bits()
	})
}

// Match returns the bogon category for ip, or CatPublic if ip is globally
// routable. IPv4-mapped IPv6 addresses are unwrapped before classification.
func Match(ip netip.Addr) Category {
	if !ip.IsValid() {
		return CatReserved
	}
	ip = ip.Unmap()
	for _, e := range prefixes {
		if e.prefix.Contains(ip) {
			return e.cat
		}
	}
	return CatPublic
}

// IsBogon reports whether ip belongs to any reserved/private range.
func IsBogon(ip netip.Addr) bool {
	return Match(ip) != CatPublic
}
