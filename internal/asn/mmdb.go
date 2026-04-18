package asn

import (
	"fmt"
	"net/netip"

	"github.com/oschwald/maxminddb-golang/v2"
)

// MMDB reads an offline ASN database in MaxMind's MMDB format.
// Compatible with DB-IP Lite ASN and MaxMind GeoLite2-ASN.
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

// mmdbRecord captures the subset of ASN fields we read.
type mmdbRecord struct {
	ASNumber uint32 `maxminddb:"autonomous_system_number"`
	ASOrg    string `maxminddb:"autonomous_system_organization"`
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
	if rec.ASNumber == 0 {
		return 0, "", false
	}
	return rec.ASNumber, rec.ASOrg, true
}
