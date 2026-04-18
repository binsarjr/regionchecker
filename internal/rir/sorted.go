package rir

import (
	"encoding/binary"
	"net/netip"
	"sort"
	"time"
)

// RegistryID maps registry names to compact uint8.
type RegistryID uint8

const (
	RegUnknown RegistryID = iota
	RegAPNIC
	RegARIN
	RegRIPE
	RegLACNIC
	RegAFRINIC
	RegIANA
)

func RegistryFromString(s string) RegistryID {
	switch s {
	case "apnic":
		return RegAPNIC
	case "arin":
		return RegARIN
	case "ripencc", "ripe":
		return RegRIPE
	case "lacnic":
		return RegLACNIC
	case "afrinic":
		return RegAFRINIC
	case "iana":
		return RegIANA
	default:
		return RegUnknown
	}
}

func (r RegistryID) String() string {
	switch r {
	case RegAPNIC:
		return "apnic"
	case RegARIN:
		return "arin"
	case RegRIPE:
		return "ripencc"
	case RegLACNIC:
		return "lacnic"
	case RegAFRINIC:
		return "afrinic"
	case RegIANA:
		return "iana"
	}
	return ""
}

// V4Range is a contiguous IPv4 allocation range.
type V4Range struct {
	Start    uint32
	End      uint32 // inclusive
	CC       [2]byte
	Registry RegistryID
	Status   Status
	Date     uint32 // days since epoch, 0 unknown
}

// V6Range is a contiguous IPv6 allocation range expressed as two 128-bit values.
type V6Range struct {
	StartHi uint64
	StartLo uint64
	EndHi   uint64
	EndLo   uint64
	CC      [2]byte
	Registry RegistryID
	Status  Status
	Date    uint32
}

// ASNRange is a contiguous ASN allocation range.
type ASNRange struct {
	Start    uint32
	End      uint32
	CC       [2]byte
	Registry RegistryID
	Status   Status
	Date     uint32
}

// DB holds sorted IPv4/IPv6/ASN range slices for fast country lookup.
type DB struct {
	V4  []V4Range
	V6  []V6Range
	ASN []ASNRange
}

// Meta describes a single lookup match.
type Meta struct {
	Registry string
	Status   Status
	Date     time.Time
	CIDR     string
}

// LookupIP returns the country code for ip, or empty if no match.
func (db *DB) LookupIP(ip netip.Addr) (cc string, meta Meta, ok bool) {
	ip = ip.Unmap()
	if ip.Is4() {
		u := v4ToU32(ip)
		i := sort.Search(len(db.V4), func(i int) bool {
			return db.V4[i].End >= u
		})
		if i < len(db.V4) && db.V4[i].Start <= u {
			r := db.V4[i]
			if r.CC[0] == 0 {
				return "", Meta{}, false
			}
			return string(r.CC[:]), Meta{
				Registry: r.Registry.String(),
				Status:   r.Status,
				Date:     daysToTime(r.Date),
			}, true
		}
		return "", Meta{}, false
	}
	if ip.Is6() {
		b := ip.As16()
		hi := binary.BigEndian.Uint64(b[0:8])
		lo := binary.BigEndian.Uint64(b[8:16])
		i := sort.Search(len(db.V6), func(i int) bool {
			return cmp128(db.V6[i].EndHi, db.V6[i].EndLo, hi, lo) >= 0
		})
		if i < len(db.V6) {
			r := db.V6[i]
			if cmp128(r.StartHi, r.StartLo, hi, lo) <= 0 {
				if r.CC[0] == 0 {
					return "", Meta{}, false
				}
				return string(r.CC[:]), Meta{
					Registry: r.Registry.String(),
					Status:   r.Status,
					Date:     daysToTime(r.Date),
				}, true
			}
		}
	}
	return "", Meta{}, false
}

// LookupASN returns the country code for asn.
func (db *DB) LookupASN(asn uint32) (cc string, meta Meta, ok bool) {
	i := sort.Search(len(db.ASN), func(i int) bool {
		return db.ASN[i].End >= asn
	})
	if i < len(db.ASN) && db.ASN[i].Start <= asn {
		r := db.ASN[i]
		if r.CC[0] == 0 {
			return "", Meta{}, false
		}
		return string(r.CC[:]), Meta{
			Registry: r.Registry.String(),
			Status:   r.Status,
			Date:     daysToTime(r.Date),
		}, true
	}
	return "", Meta{}, false
}

func cmp128(ah, al, bh, bl uint64) int {
	switch {
	case ah < bh:
		return -1
	case ah > bh:
		return 1
	case al < bl:
		return -1
	case al > bl:
		return 1
	}
	return 0
}

var epoch = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

func timeToDays(t time.Time) uint32 {
	if t.IsZero() {
		return 0
	}
	return uint32(t.Sub(epoch).Hours() / 24)
}

func daysToTime(d uint32) time.Time {
	if d == 0 {
		return time.Time{}
	}
	return epoch.Add(time.Duration(d) * 24 * time.Hour)
}
