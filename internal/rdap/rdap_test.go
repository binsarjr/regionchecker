package rdap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBootstrap_BaseURL(t *testing.T) {
	b, err := LoadBootstrap()
	if err != nil {
		t.Fatalf("LoadBootstrap: %v", err)
	}
	// .com is always in the IANA bootstrap.
	if got := b.BaseURL("com"); !strings.HasSuffix(got, "/") || got == "" {
		t.Errorf("BaseURL(com) = %q", got)
	}
	if got := b.BaseURL(".com"); got == "" {
		t.Errorf("BaseURL(.com) should normalize leading dot")
	}
	if got := b.BaseURL("nosuch-tld-zz"); got != "" {
		t.Errorf("BaseURL(nosuch) = %q, want empty", got)
	}
}

func TestTLDOf(t *testing.T) {
	cases := map[string]string{
		"tokopedia.com":   "com",
		"google.co.id":    "id",
		"WWW.EXAMPLE.ORG": "org",
		"single":          "single",
		"trailing.dot.":   "dot",
	}
	for in, want := range cases {
		if got := tldOf(in); got != want {
			t.Errorf("tldOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractRegistrant_CCParam(t *testing.T) {
	raw := []byte(`{
		"entities": [{
			"roles": ["registrant"],
			"vcardArray": ["vcard", [
				["version", {}, "text", "4.0"],
				["fn", {}, "text", "PT Tokopedia"],
				["adr", {"cc": "ID"}, "text", ["","","","","","","Indonesia"]]
			]]
		}]
	}`)
	info, ok := extractRegistrant(raw)
	if !ok || info.Country != "ID" {
		t.Errorf("got (%+v, %v), want ID", info, ok)
	}
}

func TestExtractRegistrant_NestedEntity(t *testing.T) {
	// Some registries wrap registrant under a nested entity.
	raw := []byte(`{
		"entities": [{
			"roles": ["registrar"],
			"entities": [{
				"roles": ["registrant"],
				"vcardArray": ["vcard", [
					["version", {}, "text", "4.0"],
					["adr", {"cc": "us"}, "text", ["","","","","","","USA"]]
				]]
			}]
		}]
	}`)
	info, ok := extractRegistrant(raw)
	if !ok || info.Country != "US" {
		t.Errorf("got (%+v, %v), want US", info, ok)
	}
}

func TestExtractRegistrant_Redacted(t *testing.T) {
	raw := []byte(`{"entities":[{"roles":["registrant"],"vcardArray":["vcard",[["version",{},"text","4.0"]]]}]}`)
	if _, ok := extractRegistrant(raw); ok {
		t.Errorf("redacted response should return ok=false")
	}
}

func TestClient_Lookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/domain/tokopedia.com") {
			w.Header().Set("Content-Type", "application/rdap+json")
			_, _ = w.Write([]byte(`{"entities":[{"roles":["registrant"],"vcardArray":["vcard",[["adr",{"cc":"ID"},"text",["","","","","","","Indonesia"]]]]}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/domain/missing.com") {
			http.Error(w, "nope", http.StatusNotFound)
			return
		}
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer srv.Close()
	c := &Client{
		HTTP:      srv.Client(),
		Bootstrap: &Bootstrap{byTLD: map[string]string{"com": srv.URL + "/"}},
		Timeout:   2 * time.Second,
	}
	cc, ok := c.Lookup(context.Background(), "tokopedia.com")
	if !ok || cc != "ID" {
		t.Errorf("Lookup(tokopedia.com) = (%q, %v), want (ID, true)", cc, ok)
	}
	cc, ok = c.Lookup(context.Background(), "missing.com")
	if ok || cc != "" {
		t.Errorf("Lookup(missing.com) = (%q, %v), want ('', false)", cc, ok)
	}
}

func TestClient_FollowsRelatedLink(t *testing.T) {
	// Simulate .com chain: registry response is thin (registrar only);
	// follow "related" link to registrar RDAP which exposes registrant.
	var registrarURL string
	registrarSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"entities":[{"roles":["registrant"],"vcardArray":["vcard",[["adr",{"cc":"ID"},"text",["","","","","","","Indonesia"]]]]}]}`))
	}))
	defer registrarSrv.Close()
	registrarURL = registrarSrv.URL + "/domain/TOKOPEDIA.COM"

	registrySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `{"entities":[{"roles":["registrar"],"vcardArray":["vcard",[["fn",{},"text","CSC"]]]}],"links":[{"rel":"related","type":"application/rdap+json","href":"` + registrarURL + `"}]}`
		_, _ = w.Write([]byte(body))
	}))
	defer registrySrv.Close()

	c := &Client{
		HTTP:      registrySrv.Client(),
		Bootstrap: &Bootstrap{byTLD: map[string]string{"com": registrySrv.URL + "/"}},
		Timeout:   2 * time.Second,
	}
	cc, ok := c.Lookup(context.Background(), "tokopedia.com")
	if !ok || cc != "ID" {
		t.Errorf("Lookup = (%q, %v), want (ID, true)", cc, ok)
	}
}

func TestDiskCache_GetPut(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewDiskCache(filepath.Join(dir, "rdap"), time.Hour)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	// Miss.
	if _, ok := cache.Get("tokopedia.com"); ok {
		t.Errorf("expected miss")
	}
	// Positive put/get.
	cache.Put("tokopedia.com", "ID")
	if cc, ok := cache.Get("tokopedia.com"); !ok || cc != "ID" {
		t.Errorf("Get = (%q, %v), want (ID, true)", cc, ok)
	}
	// Negative put cached as-is (empty cc, ok=true).
	cache.Put("redacted.com", "")
	if cc, ok := cache.Get("redacted.com"); !ok || cc != "" {
		t.Errorf("Get negative = (%q, %v), want ('', true)", cc, ok)
	}
}

func TestDiskCache_TTL(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewDiskCache(filepath.Join(dir, "rdap"), 1*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	cache.Put("expire.com", "US")
	time.Sleep(10 * time.Millisecond)
	if _, ok := cache.Get("expire.com"); ok {
		t.Errorf("expected expired miss")
	}
}

func TestClient_UsesCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"entities":[{"roles":["registrant"],"vcardArray":["vcard",[["adr",{"cc":"SG"},"text",["","","","","","","Singapore"]]]]}]}`))
	}))
	defer srv.Close()
	dir, _ := os.MkdirTemp("", "rdap-")
	defer os.RemoveAll(dir)
	cache, _ := NewDiskCache(dir, time.Hour)
	c := &Client{
		HTTP:      srv.Client(),
		Bootstrap: &Bootstrap{byTLD: map[string]string{"com": srv.URL + "/"}},
		Cache:     cache,
		Timeout:   2 * time.Second,
	}
	_, _ = c.Lookup(context.Background(), "a.com")
	_, _ = c.Lookup(context.Background(), "a.com")
	if hits != 1 {
		t.Errorf("server hits = %d, want 1 (cache should suppress)", hits)
	}
}
