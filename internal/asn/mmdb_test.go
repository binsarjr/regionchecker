package asn_test

import (
	"net/netip"
	"testing"

	"github.com/binsarjr/regionchecker/internal/asn"
)

func TestOpenMMDB_MissingFile(t *testing.T) {
	_, err := asn.OpenMMDB("/no/such/path.mmdb")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestMMDB_NilSafeLookup(t *testing.T) {
	var m *asn.MMDB
	asn2, org, ok := m.Lookup(netip.MustParseAddr("8.8.8.8"))
	if asn2 != 0 || org != "" || ok {
		t.Errorf("nil Lookup returned (%d, %q, %v)", asn2, org, ok)
	}
}

func TestMMDB_CloseNilSafe(t *testing.T) {
	var m *asn.MMDB
	if err := m.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
}
