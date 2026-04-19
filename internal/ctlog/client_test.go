package ctlog

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeCertPEM returns a self-signed cert PEM with Subject.Country.
func makeCertPEM(t *testing.T, country string) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	var c []string
	if country != "" {
		c = []string{country}
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test.example",
			Country:    c,
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// rewriteRT makes requests to crt.sh land on a local httptest server.
type rewriteRT struct{ to string }

func (rt *rewriteRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "crt.sh" {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(rt.to, "http://")
	}
	return http.DefaultTransport.RoundTrip(req)
}

func TestLookup_FindsCountryFromHistoricalCert(t *testing.T) {
	// 3 cert entries; the oldest (lowest id) carries Subject.C = ID.
	idCert := makeCertPEM(t, "ID")
	dvCert := makeCertPEM(t, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("output") == "json" {
			_, _ = fmt.Fprintf(w, `[{"id":100},{"id":200},{"id":300}]`)
			return
		}
		id := q.Get("d")
		switch id {
		case "100":
			_, _ = w.Write(idCert)
		case "200", "300":
			_, _ = w.Write(dvCert)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient()
	c.HTTP = &http.Client{Transport: &rewriteRT{to: srv.URL}}
	c.Timeout = 2 * time.Second
	c.MaxCerts = 5

	cc, ok := c.Lookup(context.Background(), "target.example")
	if !ok || cc != "ID" {
		t.Errorf("Lookup = (%q, %v), want (ID, true)", cc, ok)
	}
}

func TestLookup_AllDVReturnsMiss(t *testing.T) {
	dvCert := makeCertPEM(t, "")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("output") == "json" {
			_, _ = fmt.Fprintf(w, `[{"id":1}]`)
			return
		}
		_, _ = w.Write(dvCert)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient()
	c.HTTP = &http.Client{Transport: &rewriteRT{to: srv.URL}}
	c.Timeout = 2 * time.Second

	cc, ok := c.Lookup(context.Background(), "dvonly.example")
	if ok || cc != "" {
		t.Errorf("Lookup = (%q, %v), want ('', false)", cc, ok)
	}
}

func TestLookup_NoCertsReturnsMiss(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `[]`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient()
	c.HTTP = &http.Client{Transport: &rewriteRT{to: srv.URL}}
	c.Timeout = 2 * time.Second

	cc, ok := c.Lookup(context.Background(), "ghost.example")
	if ok || cc != "" {
		t.Errorf("expected miss")
	}
}

func TestDiskCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := NewDiskCache(filepath.Join(dir, "ct"), time.Hour)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	c.Put("x.example", "ID")
	if cc, ok := c.Get("x.example"); !ok || cc != "ID" {
		t.Errorf("Get = (%q, %v), want (ID, true)", cc, ok)
	}
	c.Put("neg.example", "")
	if cc, ok := c.Get("neg.example"); !ok || cc != "" {
		t.Errorf("neg = (%q, %v), want ('', true)", cc, ok)
	}
}

func TestItoa(t *testing.T) {
	cases := map[int64]string{0: "0", 42: "42", -7: "-7", 12345: "12345"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}
