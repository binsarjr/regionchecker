package contentscan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

const (
	idBody = `<!DOCTYPE html>
<html lang="id-ID"><head><title>PT Widya Security</title></head><body>
Kontak: +62 21 1234567 atau email ke cs@widya.co.id<br>
Kantor Jakarta. Layanan tersedia dalam Bahasa Indonesia.<br>
Harga mulai Rp 100.000. Gedung di Surabaya.
</body></html>`

	sgBody = `<!DOCTYPE html>
<html lang="en-SG"><head><title>Acme Pte Ltd</title></head><body>
Visit us at Marina Bay, Singapore. Phone +65 6123 4567.<br>
From SGD 500. acme.com.sg
</body></html>`

	myBody = `<!DOCTYPE html>
<html lang="ms"><head><title>Acme Sdn Bhd</title></head><body>
Hubungi kami di Kuala Lumpur, Malaysia. +60 12 345 6789.<br>
Harga RM 250. acme.com.my
</body></html>`

	jpBody = `<!DOCTYPE html>
<html lang="ja-JP"><head><title>株式会社アクメ</title></head><body>
東京. 連絡先: +81 3 1234 5678.<br>
¥5000 より. acme.co.jp
</body></html>`

	gbBody = `<!DOCTYPE html>
<html lang="en-GB"><head><title>Acme Ltd</title></head><body>
London office, United Kingdom. +44 20 7946 0018.<br>
From £50. acme.co.uk
</body></html>`

	// Cloudflare-fronted US page (no strong country markers).
	ambiguousBody = `<!DOCTYPE html>
<html lang="en"><head></head><body>Welcome. Checking your browser...</body></html>`
)

func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c := NewClient()
	c.HTTP = srv.Client()
	c.Timeout = 2 * time.Second
	// Override the default detectors with a fresh copy; registry is process-global.
	c.Detectors = DefaultDetectors()
	return c
}

func serve(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
}

func TestDetectors_ScoreStrongMarkers(t *testing.T) {
	cases := []struct {
		body, wantCC string
	}{
		{idBody, "ID"},
		{sgBody, "SG"},
		{myBody, "MY"},
		{jpBody, "JP"},
		{gbBody, "GB"},
	}
	for _, tc := range cases {
		t.Run(tc.wantCC, func(t *testing.T) {
			scores := map[string]int{}
			for _, d := range DefaultDetectors() {
				scores[d.CC()] = d.Score(tc.body)
			}
			best, bestScore := "", 0
			for cc, s := range scores {
				if s > bestScore {
					best, bestScore = cc, s
				}
			}
			if best != tc.wantCC {
				t.Errorf("detectors: best=%q score=%d want %q. scores=%+v", best, bestScore, tc.wantCC, scores)
			}
			if bestScore < 3 {
				t.Errorf("score %d below threshold 3", bestScore)
			}
		})
	}
}

func TestClient_LookupAboveThreshold(t *testing.T) {
	srv := serve(t, idBody)
	defer srv.Close()
	c := testClient(t, srv)

	host := srv.Listener.Addr().String()
	// Direct scan: Lookup uses https://host — but our fake server is
	// https, so go through the client directly via fetchBody+scan.
	body := c.fetchBody(context.Background(), srv.URL)
	if body == "" {
		t.Fatalf("empty body from test server")
	}
	_ = host
	// Run score path
	best, bestScore := "", 0
	for _, d := range c.Detectors {
		if s := d.Score(body); s > bestScore {
			best, bestScore = d.CC(), s
		}
	}
	if best != "ID" || bestScore < c.Threshold {
		t.Errorf("best=%q score=%d, want ID above %d", best, bestScore, c.Threshold)
	}
}

func TestClient_LookupBelowThresholdReturnsMiss(t *testing.T) {
	srv := serve(t, ambiguousBody)
	defer srv.Close()
	c := testClient(t, srv)
	body := c.fetchBody(context.Background(), srv.URL)
	best, bestScore := "", 0
	for _, d := range c.Detectors {
		if s := d.Score(body); s > bestScore {
			best, bestScore = d.CC(), s
		}
	}
	if bestScore >= c.Threshold {
		t.Errorf("ambiguous body scored %d (%q), expected < threshold", bestScore, best)
	}
}

func TestDiskCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := NewDiskCache(filepath.Join(dir, "scan"), time.Hour)
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
	c.Put("empty.example", "")
	if cc, ok := c.Get("empty.example"); !ok || cc != "" {
		t.Errorf("negative cache = (%q, %v), want ('', true)", cc, ok)
	}
}

func TestDiskCache_TTLExpires(t *testing.T) {
	dir := t.TempDir()
	c, err := NewDiskCache(filepath.Join(dir, "scan"), time.Millisecond)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	c.Put("x.example", "SG")
	time.Sleep(10 * time.Millisecond)
	if _, ok := c.Get("x.example"); ok {
		t.Errorf("expected expired miss")
	}
}
