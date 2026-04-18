// Package rir parses Regional Internet Registry delegated statistics files
// (APNIC, ARIN, RIPE NCC, LACNIC, AFRINIC, NRO combined) into IP/ASN ranges
// tagged with ISO 3166-1 alpha-2 country codes.
//
// Format reference: https://www.apnic.net/about-apnic/corporate-documents/documents/resource-guidelines/rir-statistics-exchange-format/
package rir

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// RecordType enumerates delegation resource types.
type RecordType uint8

const (
	TypeUnknown RecordType = iota
	TypeIPv4
	TypeIPv6
	TypeASN
)

// Status classifies delegation state.
type Status uint8

const (
	StatusUnknown Status = iota
	StatusAllocated
	StatusAssigned
	StatusReserved
	StatusAvailable
	StatusIETF
)

func parseStatus(s string) Status {
	switch s {
	case "allocated":
		return StatusAllocated
	case "assigned":
		return StatusAssigned
	case "reserved":
		return StatusReserved
	case "available":
		return StatusAvailable
	case "ietf":
		return StatusIETF
	default:
		return StatusUnknown
	}
}

// Record represents a single delegated-stats line.
type Record struct {
	Registry string
	CC       string // ISO 3166-1 alpha-2, empty for reserved/available
	Type     RecordType
	Start    string // raw start token (IPv4 dotted, IPv6 addr, ASN digits)
	Value    uint64 // IPv4: count; IPv6: prefix length; ASN: count
	Date     time.Time
	Status   Status
	OpaqueID string // extended format only
}

// Header describes the delegated file version line.
type Header struct {
	Version   string
	Registry  string
	Serial    string
	Records   uint64
	StartDate string
	EndDate   string
	UTCOffset string
}

// Parser streams delegated lines, invoking fn per record. Summary and version
// lines populate Header fields on the returned value.
type Parser struct {
	Header Header
	// Skip controls whether reserved/available records are passed to fn.
	// Default false = all records delivered.
	Skip bool
}

// Parse reads from r, invoking fn for every data record. It returns the
// header parsed from the first version line and the count of records
// delivered to fn.
func (p *Parser) Parse(r io.Reader, fn func(Record) error) (uint64, error) {
	br := bufio.NewScanner(r)
	br.Buffer(make([]byte, 1<<16), 1<<20)
	var delivered uint64
	var headerSeen bool
	for br.Scan() {
		line := br.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) < 6 {
			return delivered, fmt.Errorf("rir: short line: %q", line)
		}
		// Summary lines: apnic|*|ipv4|*|52341|summary (6 fields)
		if len(fields) == 6 && fields[5] == "summary" {
			continue
		}
		if len(fields) < 7 {
			return delivered, fmt.Errorf("rir: short record line: %q", line)
		}
		// Version line: 2|apnic|20260417|78234|19830613|20260417|+1000
		if !headerSeen && len(fields) == 7 && isDigits(fields[0]) && isDigits(fields[2]) {
			recs, _ := strconv.ParseUint(fields[3], 10, 64)
			p.Header = Header{
				Version:   fields[0],
				Registry:  fields[1],
				Serial:    fields[2],
				Records:   recs,
				StartDate: fields[4],
				EndDate:   fields[5],
				UTCOffset: fields[6],
			}
			headerSeen = true
			continue
		}
		// Catch-all summary with star country too
		if fields[1] == "*" {
			continue
		}
		rec, err := parseRecord(fields)
		if err != nil {
			return delivered, err
		}
		if p.Skip && (rec.Status == StatusReserved || rec.Status == StatusAvailable) {
			continue
		}
		if err := fn(rec); err != nil {
			return delivered, err
		}
		delivered++
	}
	if err := br.Err(); err != nil {
		return delivered, fmt.Errorf("rir: scan: %w", err)
	}
	return delivered, nil
}

func parseRecord(f []string) (Record, error) {
	r := Record{
		Registry: f[0],
		CC:       f[1],
		Start:    f[3],
		Status:   parseStatus(f[6]),
	}
	switch f[2] {
	case "ipv4":
		r.Type = TypeIPv4
	case "ipv6":
		r.Type = TypeIPv6
	case "asn":
		r.Type = TypeASN
	default:
		return r, fmt.Errorf("rir: unknown type %q", f[2])
	}
	v, err := strconv.ParseUint(f[4], 10, 64)
	if err != nil {
		return r, fmt.Errorf("rir: value %q: %w", f[4], err)
	}
	r.Value = v
	if f[5] != "" {
		if t, err := time.Parse("20060102", f[5]); err == nil {
			r.Date = t
		}
	}
	if len(f) >= 8 {
		r.OpaqueID = f[7]
	}
	return r, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// CIDRsFromV4Count decomposes a contiguous IPv4 range [start, start+count) into
// the minimum list of aligned CIDR prefixes. Handles non-power-of-2 legacy
// blocks found in older APNIC allocations.
func CIDRsFromV4Count(start netip.Addr, count uint64) ([]netip.Prefix, error) {
	if !start.Is4() {
		return nil, fmt.Errorf("rir: CIDRsFromV4Count: not IPv4")
	}
	if count == 0 {
		return nil, nil
	}
	if count > math.MaxUint32 {
		return nil, fmt.Errorf("rir: v4 count overflow %d", count)
	}
	s := v4ToU32(start)
	out := make([]netip.Prefix, 0, 4)
	remain := uint32(count)
	cursor := s
	for remain > 0 {
		// Max aligned block is the largest power of 2 that:
		// - divides cursor (alignment)
		// - is <= remain
		var block uint32 = 1
		for block <= remain>>1 && cursor%(block<<1) == 0 && block<<1 != 0 {
			block <<= 1
		}
		bits := 32 - log2u32(block)
		pfx, err := netip.AddrFrom4(u32ToV4(cursor)).Prefix(int(bits))
		if err != nil {
			return nil, err
		}
		out = append(out, pfx)
		cursor += block
		remain -= block
	}
	return out, nil
}

func v4ToU32(a netip.Addr) uint32 {
	b := a.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func u32ToV4(u uint32) [4]byte {
	return [4]byte{byte(u >> 24), byte(u >> 16), byte(u >> 8), byte(u)}
}

func log2u32(x uint32) uint32 {
	var n uint32
	for x > 1 {
		x >>= 1
		n++
	}
	return n
}
