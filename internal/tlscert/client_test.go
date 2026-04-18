package tlscert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// selfSignedWithCountry returns a tls.Certificate whose leaf subject
// carries the given country code. Country can be empty to simulate DV.
func selfSignedWithCountry(t *testing.T, country string) tls.Certificate {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	var countries []string
	if country != "" {
		countries = []string{country}
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "test.example",
			Country:      countries,
			Organization: []string{"test"},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:  []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	leaf, _ := x509.ParseCertificate(der)
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
		Leaf:        leaf,
	}
}

// dialerFor returns a Dialer func that serves the given certificate from
// an in-memory net.Pipe, bypassing real network I/O.
func dialerFor(t *testing.T, cert tls.Certificate) func(ctx context.Context, addr string, cfg *tls.Config) (*tls.Conn, error) {
	return func(ctx context.Context, addr string, cfg *tls.Config) (*tls.Conn, error) {
		clientSide, serverSide := net.Pipe()
		serverCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		go func() {
			srv := tls.Server(serverSide, serverCfg)
			_ = srv.HandshakeContext(ctx)
			// Keep open long enough for client to read state.
			time.AfterFunc(500*time.Millisecond, func() { _ = srv.Close() })
		}()
		client := tls.Client(clientSide, cfg)
		if err := client.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		return client, nil
	}
}

func TestLookup_EVCertReturnsCountry(t *testing.T) {
	cert := selfSignedWithCountry(t, "ID")
	c := &Client{Timeout: time.Second, Dialer: dialerFor(t, cert)}
	cc, ok := c.Lookup(context.Background(), "test.example")
	if !ok || cc != "ID" {
		t.Errorf("Lookup = (%q, %v), want (ID, true)", cc, ok)
	}
}

func TestLookup_DVCertNoCountry(t *testing.T) {
	cert := selfSignedWithCountry(t, "")
	c := &Client{Timeout: time.Second, Dialer: dialerFor(t, cert)}
	cc, ok := c.Lookup(context.Background(), "test.example")
	if ok || cc != "" {
		t.Errorf("Lookup = (%q, %v), want ('', false)", cc, ok)
	}
}

func TestLookup_CacheHitSkipsDial(t *testing.T) {
	dialed := 0
	cert := selfSignedWithCountry(t, "SG")
	base := dialerFor(t, cert)
	c := &Client{
		Timeout: time.Second,
		Cache:   NewMemCache(),
		Dialer: func(ctx context.Context, addr string, cfg *tls.Config) (*tls.Conn, error) {
			dialed++
			return base(ctx, addr, cfg)
		},
	}
	_, _ = c.Lookup(context.Background(), "x.example")
	_, _ = c.Lookup(context.Background(), "x.example")
	if dialed != 1 {
		t.Errorf("dial count = %d, want 1 (cache should suppress)", dialed)
	}
}

func TestLookup_DialFailReturnsMiss(t *testing.T) {
	c := &Client{
		Timeout: 50 * time.Millisecond,
		Dialer: func(ctx context.Context, addr string, cfg *tls.Config) (*tls.Conn, error) {
			return nil, context.DeadlineExceeded
		},
	}
	cc, ok := c.Lookup(context.Background(), "does.not.exist.example")
	if ok || cc != "" {
		t.Errorf("Lookup = (%q, %v), want ('', false)", cc, ok)
	}
}

func TestDiskCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := NewDiskCache(filepath.Join(dir, "tls"), time.Hour)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	if _, ok := c.Get("x.example"); ok {
		t.Errorf("expected miss")
	}
	c.Put("x.example", "ID")
	if cc, ok := c.Get("x.example"); !ok || cc != "ID" {
		t.Errorf("Get = (%q, %v), want (ID, true)", cc, ok)
	}
	c.Put("redacted.example", "")
	if cc, ok := c.Get("redacted.example"); !ok || cc != "" {
		t.Errorf("Get negative = (%q, %v), want ('', true)", cc, ok)
	}
}

func TestDiskCache_TTLExpiry(t *testing.T) {
	dir := t.TempDir()
	c, err := NewDiskCache(filepath.Join(dir, "tls"), time.Millisecond)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	c.Put("x.example", "US")
	time.Sleep(10 * time.Millisecond)
	if _, ok := c.Get("x.example"); ok {
		t.Errorf("expected expired miss")
	}
}
