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
			"google.co.id": {mkIP("49.0.109.161")},
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
		name       string
		domainCC   string
		domainType string
		ipCC       string
		dnsFailed  bool
		isIP       bool
		wantFinal  string
		wantConf   string
	}{
		{"raw ip us", "", "", "US", false, true, "US", classifier.ConfIPOnly},
		{"high match", "ID", "cctld", "ID", false, false, "ID", classifier.ConfHigh},
		{"medium id offshore", "ID", "cctld", "US", false, false, "ID", classifier.ConfMediumDomainIDOffshore},
		{"medium cc mismatch", "GB", "cctld", "US", false, false, "GB", classifier.ConfMediumDomainCCMismatch},
		{"medium generic id", "", "generic", "ID", false, false, "ID", classifier.ConfMediumGenericTLDIDHost},
		{"low dns failed", "ID", "cctld", "", true, false, "ID", classifier.ConfLowDNSFailed},
		{"unknown", "", "", "", true, false, "", classifier.ConfUnknown},
		{"generic ip-only non-id", "", "generic", "US", false, false, "US", classifier.ConfIPOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := classifier.Decide(tc.domainCC, tc.domainType, tc.ipCC, tc.dnsFailed, tc.isIP)
			if d.FinalCountry != tc.wantFinal || d.Confidence != tc.wantConf {
				t.Errorf("Decide = (%q, %q), want (%q, %q)",
					d.FinalCountry, d.Confidence, tc.wantFinal, tc.wantConf)
			}
		})
	}
}
