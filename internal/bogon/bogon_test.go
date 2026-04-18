package bogon

import (
	"net/netip"
	"testing"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		in   string
		want Category
	}{
		{"0.0.0.0", CatUnspecified},
		{"10.0.0.1", CatPrivate},
		{"100.64.0.1", CatCGNAT},
		{"127.0.0.1", CatLoopback},
		{"169.254.1.1", CatLinkLocal},
		{"172.16.0.1", CatPrivate},
		{"172.31.255.255", CatPrivate},
		{"172.32.0.1", CatPublic},
		{"192.0.2.1", CatDocumentation},
		{"192.168.1.1", CatPrivate},
		{"198.18.0.1", CatBenchmark},
		{"198.51.100.1", CatDocumentation},
		{"203.0.113.1", CatDocumentation},
		{"224.0.0.1", CatMulticast},
		{"255.255.255.255", CatBroadcast},
		{"8.8.8.8", CatPublic},
		{"1.1.1.1", CatPublic},
		{"::", CatUnspecified},
		{"::1", CatLoopback},
		{"fe80::1", CatLinkLocal},
		{"fc00::1", CatULA},
		{"fd00::1", CatULA},
		{"ff00::1", CatMulticast},
		{"2001:db8::1", CatDocumentation},
		{"2001:4860:4860::8888", CatPublic},
		{"::ffff:8.8.8.8", CatPublic}, // unwrapped to v4 public
		{"::ffff:10.0.0.1", CatPrivate},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			ip, err := netip.ParseAddr(c.in)
			if err != nil {
				t.Fatalf("parse %q: %v", c.in, err)
			}
			got := Match(ip)
			if got != c.want {
				t.Errorf("Match(%s) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestIsBogon(t *testing.T) {
	if !IsBogon(netip.MustParseAddr("10.0.0.1")) {
		t.Error("10.0.0.1 should be bogon")
	}
	if IsBogon(netip.MustParseAddr("8.8.8.8")) {
		t.Error("8.8.8.8 should not be bogon")
	}
}

func TestInvalidAddr(t *testing.T) {
	var zero netip.Addr
	if Match(zero) != CatReserved {
		t.Errorf("invalid addr should map to reserved, got %q", Match(zero))
	}
}

func BenchmarkMatch(b *testing.B) {
	addrs := []netip.Addr{
		netip.MustParseAddr("8.8.8.8"),
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("2001:4860:4860::8888"),
		netip.MustParseAddr("fe80::1"),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Match(addrs[i%len(addrs)])
	}
}
