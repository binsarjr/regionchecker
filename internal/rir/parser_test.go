package rir

import (
	"net/netip"
	"strings"
	"testing"
)

const sample = `2|apnic|20260417|4|19830613|20260417|+1000
apnic|*|ipv4|*|2|summary
apnic|*|ipv6|*|1|summary
apnic|*|asn|*|1|summary
apnic|CN|ipv4|1.0.1.0|256|20110414|allocated
apnic|JP|ipv4|1.0.16.0|4096|20110414|allocated
apnic|AU|ipv6|2001:dc0::|32|20020801|allocated
apnic|KR|asn|9318|1|20021219|allocated
`

func TestParser(t *testing.T) {
	p := &Parser{}
	var got []Record
	n, err := p.Parse(strings.NewReader(sample), func(r Record) error {
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n != 4 {
		t.Fatalf("delivered=%d want 4", n)
	}
	if p.Header.Registry != "apnic" || p.Header.Version != "2" {
		t.Errorf("header=%+v", p.Header)
	}
	if got[0].CC != "CN" || got[0].Type != TypeIPv4 || got[0].Value != 256 {
		t.Errorf("rec0=%+v", got[0])
	}
	if got[2].Type != TypeIPv6 || got[2].Value != 32 || got[2].CC != "AU" {
		t.Errorf("rec2=%+v", got[2])
	}
	if got[3].Type != TypeASN {
		t.Errorf("rec3=%+v", got[3])
	}
}

func TestCIDRsFromV4Count(t *testing.T) {
	cases := []struct {
		start string
		count uint64
		want  []string
	}{
		{"1.0.0.0", 256, []string{"1.0.0.0/24"}},
		{"8.8.8.0", 1024, []string{"8.8.8.0/22"}},
		{"10.0.0.0", 65536, []string{"10.0.0.0/16"}},
		// 768 = 512 + 256, 192.0.2.0 aligned to /23 covers 192.0.2.0-192.0.3.255
		{"192.0.2.0", 768, []string{"192.0.2.0/23", "192.0.4.0/24"}},
		{"1.0.0.0", 1, []string{"1.0.0.0/32"}},
	}
	for _, c := range cases {
		t.Run(c.start, func(t *testing.T) {
			addr := netip.MustParseAddr(c.start)
			got, err := CIDRsFromV4Count(addr, c.count)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("len=%d want %d got=%v", len(got), len(c.want), got)
			}
			for i, p := range got {
				if p.String() != c.want[i] {
					t.Errorf("[%d]=%s want %s", i, p, c.want[i])
				}
			}
		})
	}
}
