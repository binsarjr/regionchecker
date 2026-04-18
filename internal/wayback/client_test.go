package wayback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Indonesian-looking HTML that scores on the default ID detector.
const idBody = `<!DOCTYPE html>
<html lang="id-ID"><head><title>Expired</title></head><body>
Kontak: +62 21 1234567. Kantor Jakarta. Bahasa Indonesia.
Harga Rp 100.000.
</body></html>`

// Mock upstream Wayback API + snapshot fetch.
func mockWayback(t *testing.T, body string, available bool, status string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/wayback/available", func(w http.ResponseWriter, r *http.Request) {
		resp := availabilityResp{}
		if available {
			resp.ArchivedSnapshots.Closest.Available = true
			resp.ArchivedSnapshots.Closest.Status = status
			// Use server's own URL so fetch can resolve.
			srvURL := "http://" + r.Host + "/web/20240101120000/" + r.URL.Query().Get("url")
			resp.ArchivedSnapshots.Closest.URL = srvURL
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/web/", func(w http.ResponseWriter, r *http.Request) {
		// Only serve when id_ variant is requested (rawSnapshotURL rewrite).
		if !strings.Contains(r.URL.Path, "id_") {
			http.Error(w, "want id_", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	})
	return httptest.NewServer(mux)
}

// overrideAPI redirects the hard-coded archive.org base URL used by
// findSnapshot to the test server by shadowing http.Client behaviour.
type rewriteRT struct {
	real  http.RoundTripper
	to    string
	count *int
}

func (rt *rewriteRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.count != nil {
		*rt.count++
	}
	if req.URL.Host == "archive.org" {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(rt.to, "http://")
	}
	return rt.real.RoundTrip(req)
}

func TestLookup_ScoresID(t *testing.T) {
	srv := mockWayback(t, idBody, true, "200")
	defer srv.Close()
	count := 0
	c := NewClient()
	c.HTTP = &http.Client{Transport: &rewriteRT{
		real:  http.DefaultTransport,
		to:    srv.URL,
		count: &count,
	}}
	c.Timeout = 2 * time.Second

	cc, ok := c.Lookup(context.Background(), "expired.example")
	if !ok || cc != "ID" {
		t.Errorf("Lookup = (%q, %v), want (ID, true)", cc, ok)
	}
}

func TestLookup_NoArchiveReturnsMiss(t *testing.T) {
	srv := mockWayback(t, "", false, "")
	defer srv.Close()
	c := NewClient()
	c.HTTP = &http.Client{Transport: &rewriteRT{
		real: http.DefaultTransport,
		to:   srv.URL,
	}}
	c.Timeout = 2 * time.Second

	cc, ok := c.Lookup(context.Background(), "never-archived.example")
	if ok || cc != "" {
		t.Errorf("Lookup = (%q, %v), want ('', false)", cc, ok)
	}
}

func TestLookup_CacheSuppressesNetwork(t *testing.T) {
	srv := mockWayback(t, idBody, true, "200")
	defer srv.Close()
	count := 0
	c := NewClient()
	c.HTTP = &http.Client{Transport: &rewriteRT{
		real:  http.DefaultTransport,
		to:    srv.URL,
		count: &count,
	}}
	c.Cache = &memCache{data: make(map[string]string)}
	c.Timeout = 2 * time.Second

	_, _ = c.Lookup(context.Background(), "host.example")
	first := count
	_, _ = c.Lookup(context.Background(), "host.example")
	if count != first {
		t.Errorf("cache did not suppress: before=%d after=%d", first, count)
	}
}

func TestRawSnapshotURL(t *testing.T) {
	cases := map[string]string{
		"https://web.archive.org/web/20240101/https://example.com":     "https://web.archive.org/web/20240101id_/https://example.com",
		"https://web.archive.org/web/20240101id_/https://example.com":  "https://web.archive.org/web/20240101id_/https://example.com",
		"not-a-wayback-url.example": "not-a-wayback-url.example",
	}
	for in, want := range cases {
		if got := rawSnapshotURL(in); got != want {
			t.Errorf("rawSnapshotURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiskCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := NewDiskCache(filepath.Join(dir, "wb"), time.Hour)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	c.Put("x.example", "ID")
	if cc, ok := c.Get("x.example"); !ok || cc != "ID" {
		t.Errorf("Get = (%q, %v), want (ID, true)", cc, ok)
	}
}

// memCache is a simple in-process Cache used only by tests.
type memCache struct {
	data map[string]string
}

func (m *memCache) Get(host string) (string, bool) { cc, ok := m.data[host]; return cc, ok }
func (m *memCache) Put(host, cc string)            { m.data[host] = cc }
