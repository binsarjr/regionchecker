package asn

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

// MMDB reads an offline ASN database in MaxMind's MMDB format.
// Compatible with MaxMind GeoLite2-ASN, DB-IP Lite ASN, and ipinfo
// country+asn combined DBs.
type MMDB struct {
	reader *maxminddb.Reader
}

// OpenMMDB memory-maps the MMDB file at path.
// The caller must Close when done.
func OpenMMDB(path string) (*MMDB, error) {
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("asn: open mmdb %s: %w", path, err)
	}
	return &MMDB{reader: r}, nil
}

// Close releases the mmdb reader.
func (m *MMDB) Close() error {
	if m == nil || m.reader == nil {
		return nil
	}
	return m.reader.Close()
}

// mmdbRecord captures fields from both MaxMind-style and ipinfo-style
// ASN databases. The maxminddb decoder ignores fields not present in
// the underlying record.
type mmdbRecord struct {
	// MaxMind / DB-IP Lite fields.
	ASNumber uint32 `maxminddb:"autonomous_system_number"`
	ASOrg    string `maxminddb:"autonomous_system_organization"`
	// ipinfo fields.
	ASNString string `maxminddb:"asn"`
	ASName    string `maxminddb:"as_name"`
}

// Lookup returns the ASN and organisation for ip, or (0, "", false) if missing.
func (m *MMDB) Lookup(ip netip.Addr) (uint32, string, bool) {
	if m == nil || m.reader == nil {
		return 0, "", false
	}
	var rec mmdbRecord
	if err := m.reader.Lookup(ip).Decode(&rec); err != nil {
		return 0, "", false
	}
	if rec.ASNumber == 0 && rec.ASNString != "" {
		rec.ASNumber = parseASN(rec.ASNString)
	}
	if rec.ASOrg == "" && rec.ASName != "" {
		rec.ASOrg = rec.ASName
	}
	if rec.ASNumber == 0 {
		return 0, "", false
	}
	return rec.ASNumber, rec.ASOrg, true
}

// parseASN extracts the numeric portion of an "AS12345" or "12345" string.
func parseASN(s string) uint32 {
	s = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(s)), "AS")
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}
