package classifier_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/binsarjr/regionchecker/internal/classifier"
	"github.com/binsarjr/regionchecker/internal/rir"
)

// stubIP is a minimal IPLookup for tests.
type stubIP struct {
	ccByStart map[uint32]string
}

func (s *stubIP) LookupIP(ip netip.Addr) (cc string, meta rir.Meta, ok bool) {
	ip = ip.Unmap()
	if !ip.Is4() {
		return "", rir.Meta{}, false
	}
	b := ip.As4()
	key := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	if cc, ok := s.ccByStart[key]; ok {
		return cc, rir.Meta{Registry: "apnic"}, true
	}
	return "", rir.Meta{}, false
}

type stubASN struct {
	orgByIP map[uint32]stubASNRec
}

type stubASNRec struct {
	num uint32
	org string
}

func (s *stubASN) Lookup(ip netip.Addr) (uint32, string, bool) {
	ip = ip.Unmap()
	if !ip.Is4() {
		return 0, "", false
	}
	b := ip.As4()
	key := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	if r, ok := s.orgByIP[key]; ok {
		return r.num, r.org, true
	}
	return 0, "", false
}

type stubResolver struct {
	addrs map[string][]netip.Addr
	fail  map[string]bool
}

func (s *stubResolver) Resolve(_ context.Context, host string) ([]netip.Addr, error) {
	if s.fail[host] {
		return nil, errors.New("nxdomain")
	}
	if a, ok := s.addrs[host]; ok {
		return a, nil
	}
	return nil, errors.New("nxdomain")
}

func mkIP(s string) netip.Addr {
	a, _ := netip.ParseAddr(s)
	return a
}

func u32(a, b, c, d byte) uint32 {
	return uint32(a)<<24 | uint32(b)<<16 | uint32(c)<<8 | uint32(d)
}

func newClassifier() *classifier.Classifier {
	ip := &stubIP{ccByStart: map[uint32]string{
		u32(8, 8, 8, 8):         "US",
		u32(114, 114, 114, 114): "CN",
		u32(49, 0, 109, 161):    "ID",
		u32(1, 1, 1, 1):         "AU",
	}}
	res := &stubResolver{
		addrs: map[string][]netip.Addr{
			"google.co.id":  {mkIP("49.0.109.161")},
			"tokopedia.com": {mkIP("49.0.109.161")},
			"bbc.co.uk":     {mkIP("8.8.8.8")}, // simulate offshore host
			"example.com":   {mkIP("8.8.8.8")},
		},
		fail: map[string]bool{
			"dns.fail.example": true,
		},
	}
	return classifier.New(ip, res, nil)
}

func TestClassify_RawIP(t *testing.T) {
	c := newClassifier()
	r, err := c.Classify(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.FinalCountry != "US" {
		t.Errorf("FinalCountry = %q, want US", r.FinalCountry)
	}
	if r.Confidence != classifier.ConfIPOnly {
		t.Errorf("Confidence = %q, want %q", r.Confidence, classifier.ConfIPOnly)
	}
	if r.Type != "ip" {
		t.Errorf("Type = %q, want ip", r.Type)
	}
}

func TestClassify_AnycastAU(t *testing.T) {
	// 1.1.1.1 is APNIC-allocated to AU (gotchas §1).
	c := newClassifier()
	r, err := c.Classify(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.FinalCountry != "AU" {
		t.Errorf("FinalCountry = %q, want AU (allocation, not routing)", r.FinalCountry)
	}
}

func TestClassify_Bogon(t *testing.T) {
	c := newClassifier()
	for _, ip := range []string{"10.1.1.1", "127.0.0.1", "192.168.1.1", "169.254.1.1"} {
		_, err := c.Classify(context.Background(), ip)
		if !errors.Is(err, classifier.ErrBogon) {
			t.Errorf("Classify(%q) err = %v, want ErrBogon", ip, err)
		}
	}
}

func TestClassify_IPv4MappedIPv6(t *testing.T) {
	// ::ffff:8.8.8.8 must unmap to 8.8.8.8 and return US (gotchas §3).
	c := newClassifier()
	r, err := c.Classify(context.Background(), "::ffff:8.8.8.8")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.FinalCountry != "US" {
		t.Errorf("FinalCountry = %q, want US", r.FinalCountry)
	}
}

func TestClassify_HighConfidence(t *testing.T) {
	// google.co.id → domain ID, host ID → high.
	c := newClassifier()
	r, err := c.Classify(context.Background(), "google.co.id")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.FinalCountry != "ID" || r.Confidence != classifier.ConfHigh {
		t.Errorf("got (%q, %q), want (ID, high)", r.FinalCountry, r.Confidence)
	}
}

func TestClassify_MediumGenericTLDIDHost(t *testing.T) {
	// tokopedia.com → generic tld, host ID → medium-generic-tld-id-host.
	c := newClassifier()
	r, err := c.Classify(context.Background(), "tokopedia.com")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.Confidence != classifier.ConfMediumGenericTLDIDHost {
		t.Errorf("Confidence = %q, want %q", r.Confidence, classifier.ConfMediumGenericTLDIDHost)
	}
	if r.FinalCountry != "ID" {
		t.Errorf("FinalCountry = %q, want ID", r.FinalCountry)
	}
}

func TestClassify_ASNBrandOverridesIPGeo(t *testing.T) {
	// tokopedia.com resolves to a US Alibaba IP but ASN org TOKOPEDIA
	// matches brand→ID, so final should be ID with high-asn-brand confidence.
	c := newClassifier()
	c.ASN = &stubASN{orgByIP: map[uint32]stubASNRec{
		u32(47, 74, 244, 18): {num: 138062, org: "TOKOPEDIA"},
	}}
	// Point tokopedia.com at the US Alibaba IP.
	res, ok := c.Resolver.(*stubResolver)
	if !ok {
		t.Fatalf("resolver type")
	}
	res.addrs["tokopedia.com"] = []netip.Addr{mkIP("47.74.244.18")}
	// Also teach the IP stub that 47.74.244.18 is US.
	ip, ok := c.IP.(*stubIP)
	if !ok {
		t.Fatalf("ip type")
	}
	ip.ccByStart[u32(47, 74, 244, 18)] = "US"

	r, err := c.Classify(context.Background(), "tokopedia.com")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.FinalCountry != "ID" {
		t.Errorf("FinalCountry = %q, want ID", r.FinalCountry)
	}
	if r.Confidence != classifier.ConfHighASNBrand {
		t.Errorf("Confidence = %q, want %q", r.Confidence, classifier.ConfHighASNBrand)
	}
	if r.ASNCountry != "ID" {
		t.Errorf("ASNCountry = %q, want ID", r.ASNCountry)
	}
	if r.ASNOrg != "TOKOPEDIA" {
		t.Errorf("ASNOrg = %q, want TOKOPEDIA", r.ASNOrg)
	}
}

type stubTLSCert struct {
	byHost map[string]string
}

func (s *stubTLSCert) Lookup(_ context.Context, host string) (string, bool) {
	cc, ok := s.byHost[host]
	return cc, ok && cc != ""
}

func TestClassify_SSLCertCountryWins(t *testing.T) {
	// Generic TLD host, ASN brand misses, TLS cert Subject.C = ID.
	c := newClassifier()
	res := c.Resolver.(*stubResolver)
	res.addrs["evcert.com"] = []netip.Addr{mkIP("47.74.244.18")}
	ip := c.IP.(*stubIP)
	ip.ccByStart[u32(47, 74, 244, 18)] = "US"
	c.TLSCert = &stubTLSCert{byHost: map[string]string{"evcert.com": "ID"}}

	r, err := c.Classify(context.Background(), "evcert.com")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.FinalCountry != "ID" {
		t.Errorf("FinalCountry = %q, want ID", r.FinalCountry)
	}
	if r.Confidence != classifier.ConfHighSSLCert {
		t.Errorf("Confidence = %q, want %q", r.Confidence, classifier.ConfHighSSLCert)
	}
	if r.CertCountry != "ID" {
		t.Errorf("CertCountry = %q, want ID", r.CertCountry)
	}
}

func TestClassify_LadderEarlyExitOnDomainIPAgree(t *testing.T) {
	// google.co.id resolves to ID IP — layer 1 short-circuit.
	// TLS cert + RDAP stubs must NOT be called.
	called := false
	c := newClassifier()
	c.TLSCert = &stubTLSCert{byHost: map[string]string{"google.co.id": "XX"}}
	c.RDAP = &stubRDAP{onCall: func() { called = true }}

	r, err := c.Classify(context.Background(), "google.co.id")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.Confidence != classifier.ConfHigh {
		t.Errorf("Confidence = %q, want high (early exit)", r.Confidence)
	}
	if called {
		t.Errorf("RDAP should not be called when layer 1 wins")
	}
	if r.CertCountry != "" || r.RegistrantCountry != "" {
		t.Errorf("enrichment fields should be empty on early exit")
	}
}

type stubRDAP struct {
	cc     string
	onCall func()
}

func (s *stubRDAP) Lookup(_ context.Context, _ string) (string, bool) {
	if s.onCall != nil {
		s.onCall()
	}
	return s.cc, s.cc != ""
}

func TestClassify_DomainCCMismatch(t *testing.T) {
	// bbc.co.uk → domain GB, host US → medium-domain-cc-mismatch (not .id).
	c := newClassifier()
	r, err := c.Classify(context.Background(), "bbc.co.uk")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.Confidence != classifier.ConfMediumDomainCCMismatch {
		t.Errorf("Confidence = %q, want %q", r.Confidence, classifier.ConfMediumDomainCCMismatch)
	}
	if r.FinalCountry != "GB" {
		t.Errorf("FinalCountry = %q, want GB (trust domain)", r.FinalCountry)
	}
}

func TestClassify_DNSFailWithDomainSignal(t *testing.T) {
	// DNS fails but we have a .fail hostname with no domain signal → ErrUnresolvable.
	c := newClassifier()
	_, err := c.Classify(context.Background(), "dns.fail.example")
	if !errors.Is(err, classifier.ErrUnresolvable) {
		t.Errorf("err = %v, want ErrUnresolvable", err)
	}
}

func TestClassify_EmptyInput(t *testing.T) {
	c := newClassifier()
	_, err := c.Classify(context.Background(), "")
	if !errors.Is(err, classifier.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestDecide_Matrix(t *testing.T) {
	cases := []struct {
		name      string
		s         classifier.Signals
		wantFinal string
		wantConf  string
	}{
		{"raw ip us", classifier.Signals{IPCC: "US", IsIPInput: true}, "US", classifier.ConfIPOnly},
		{"high match", classifier.Signals{DomainCC: "ID", DomainType: "cctld", IPCC: "ID"}, "ID", classifier.ConfHigh},
		{"medium id offshore", classifier.Signals{DomainCC: "ID", DomainType: "cctld", IPCC: "US"}, "ID", classifier.ConfMediumDomainIDOffshore},
		{"medium cc mismatch", classifier.Signals{DomainCC: "GB", DomainType: "cctld", IPCC: "US"}, "GB", classifier.ConfMediumDomainCCMismatch},
		{"medium generic id", classifier.Signals{DomainType: "generic", IPCC: "ID"}, "ID", classifier.ConfMediumGenericTLDIDHost},
		{"low dns failed", classifier.Signals{DomainCC: "ID", DomainType: "cctld", DNSFailed: true}, "ID", classifier.ConfLowDNSFailed},
		{"unknown", classifier.Signals{DNSFailed: true}, "", classifier.ConfUnknown},
		{"generic ip-only non-id", classifier.Signals{DomainType: "generic", IPCC: "US"}, "US", classifier.ConfIPOnly},
		{"asn brand overrides ip", classifier.Signals{DomainType: "generic", IPCC: "US", ASNCC: "ID"}, "ID", classifier.ConfHighASNBrand},
		{"asn brand matches ip", classifier.Signals{DomainType: "generic", IPCC: "ID", ASNCC: "ID"}, "ID", classifier.ConfHigh},
		{"asn brand matches domain", classifier.Signals{DomainCC: "ID", DomainType: "cctld", IPCC: "US", ASNCC: "ID"}, "ID", classifier.ConfHigh},
		{"asn brand raw ip override", classifier.Signals{IPCC: "US", ASNCC: "ID", IsIPInput: true}, "ID", classifier.ConfHighASNBrand},
		{"rdap overrides ip", classifier.Signals{DomainType: "generic", IPCC: "US", RDAPCC: "ID"}, "ID", classifier.ConfHighRDAPRegistrant},
		{"rdap matches domain", classifier.Signals{DomainCC: "ID", DomainType: "cctld", IPCC: "US", RDAPCC: "ID"}, "ID", classifier.ConfHigh},
		{"asn+rdap agree", classifier.Signals{DomainType: "generic", IPCC: "US", ASNCC: "ID", RDAPCC: "ID"}, "ID", classifier.ConfHigh},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := classifier.Decide(tc.s)
			if d.FinalCountry != tc.wantFinal || d.Confidence != tc.wantConf {
				t.Errorf("Decide = (%q, %q), want (%q, %q)",
					d.FinalCountry, d.Confidence, tc.wantFinal, tc.wantConf)
			}
		})
	}
}
