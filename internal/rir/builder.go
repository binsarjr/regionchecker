package rir

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"sort"
)

// Build parses r as a delegated stats stream and returns a populated DB with
// sorted V4 / V6 / ASN slices ready for LookupIP.
func Build(r io.Reader) (*DB, error) {
	db := &DB{
		V4:  make([]V4Range, 0, 200_000),
		V6:  make([]V6Range, 0, 100_000),
		ASN: make([]ASNRange, 0, 50_000),
	}
	p := &Parser{}
	_, err := p.Parse(r, func(rec Record) error {
		if rec.CC == "" || len(rec.CC) != 2 {
			return nil
		}
		cc := [2]byte{rec.CC[0], rec.CC[1]}
		reg := RegistryFromString(rec.Registry)
		dateDays := timeToDays(rec.Date)
		switch rec.Type {
		case TypeIPv4:
			start, err := netip.ParseAddr(rec.Start)
			if err != nil || !start.Is4() {
				return fmt.Errorf("rir: ipv4 start %q: %v", rec.Start, err)
			}
			u := v4ToU32(start)
			if rec.Value == 0 {
				return nil
			}
			end := u + uint32(rec.Value) - 1
			db.V4 = append(db.V4, V4Range{
				Start:    u,
				End:      end,
				CC:       cc,
				Registry: reg,
				Status:   rec.Status,
				Date:     dateDays,
			})
		case TypeIPv6:
			start, err := netip.ParseAddr(rec.Start)
			if err != nil || !start.Is6() {
				return fmt.Errorf("rir: ipv6 start %q: %v", rec.Start, err)
			}
			b := start.As16()
			hi := binary.BigEndian.Uint64(b[0:8])
			lo := binary.BigEndian.Uint64(b[8:16])
			prefixLen := uint(rec.Value)
			if prefixLen == 0 || prefixLen > 128 {
				return fmt.Errorf("rir: ipv6 prefix %d invalid", prefixLen)
			}
			eh, el := prefixEnd128(hi, lo, prefixLen)
			db.V6 = append(db.V6, V6Range{
				StartHi:  hi,
				StartLo:  lo,
				EndHi:    eh,
				EndLo:    el,
				CC:       cc,
				Registry: reg,
				Status:   rec.Status,
				Date:     dateDays,
			})
		case TypeASN:
			startASN, err := parseUint32(rec.Start)
			if err != nil {
				return fmt.Errorf("rir: asn start %q: %v", rec.Start, err)
			}
			if rec.Value == 0 {
				return nil
			}
			db.ASN = append(db.ASN, ASNRange{
				Start:    startASN,
				End:      startASN + uint32(rec.Value) - 1,
				CC:       cc,
				Registry: reg,
				Status:   rec.Status,
				Date:     dateDays,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(db.V4, func(i, j int) bool { return db.V4[i].Start < db.V4[j].Start })
	sort.Slice(db.V6, func(i, j int) bool {
		return cmp128(db.V6[i].StartHi, db.V6[i].StartLo, db.V6[j].StartHi, db.V6[j].StartLo) < 0
	})
	sort.Slice(db.ASN, func(i, j int) bool { return db.ASN[i].Start < db.ASN[j].Start })
	return db, nil
}

func parseUint32(s string) (uint32, error) {
	var n uint32
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not digit: %q", s)
		}
		n = n*10 + uint32(c-'0')
	}
	return n, nil
}

// prefixEnd128 returns the last address in prefix (hi,lo)/bits.
func prefixEnd128(hi, lo uint64, bits uint) (uint64, uint64) {
	if bits >= 128 {
		return hi, lo
	}
	if bits <= 64 {
		hostBits := 64 - bits
		mask := uint64(1)<<hostBits - 1
		return hi | mask, ^uint64(0)
	}
	hostBits := 128 - bits
	mask := uint64(1)<<hostBits - 1
	return hi, lo | mask
}
