package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetcher200Then304(t *testing.T) {
	const etag = `"v1"`
	const lastMod = "Wed, 21 Oct 2015 07:28:00 GMT"
	body := []byte("payload v1")
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("If-None-Match") == etag || r.Header.Get("If-Modified-Since") == lastMod {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", lastMod)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := NewFetcher(store, srv.Client())

	ctx := context.Background()
	got, err := f.Fetch(ctx, srv.URL, "key")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("first body mismatch: %q", got)
	}
	m1, err := store.ReadMeta("key")
	if err != nil {
		t.Fatal(err)
	}
	if m1.ETag != etag || m1.LastModified != lastMod || m1.Bytes != int64(len(body)) {
		t.Fatalf("meta after 200: %+v", m1)
	}
	if m1.SHA256 == "" {
		t.Fatal("sha256 empty after 200")
	}
	firstFetched := m1.FetchedAt

	time.Sleep(10 * time.Millisecond)

	got2, err := f.Fetch(ctx, srv.URL, "key")
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if string(got2) != string(body) {
		t.Fatalf("second body mismatch: %q", got2)
	}
	m2, err := store.ReadMeta("key")
	if err != nil {
		t.Fatal(err)
	}
	if !m2.FetchedAt.After(firstFetched) {
		t.Fatalf("fetched_at not bumped after 304: %v vs %v", m2.FetchedAt, firstFetched)
	}
	if m2.ETag != etag || m2.SHA256 != m1.SHA256 {
		t.Fatalf("meta identity lost after 304: %+v", m2)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", hits)
	}
}

func TestFetcherSingleflight(t *testing.T) {
	var hits int32
	body := []byte("sf-body")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		time.Sleep(50 * time.Millisecond) // widen the race window
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := NewFetcher(store, srv.Client())

	const n = 10
	var wg sync.WaitGroup
	errs := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			b, err := f.Fetch(context.Background(), srv.URL, "sfkey")
			if err != nil {
				errs <- err
				return
			}
			if string(b) != string(body) {
				errs <- &fetchBodyErr{b}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("singleflight should collapse to 1 hit, got %d", got)
	}
}

type fetchBodyErr struct{ body []byte }

func (e *fetchBodyErr) Error() string { return "unexpected body: " + string(e.body) }

func TestFetcherAge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("age"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := NewFetcher(store, srv.Client())
	if _, err := f.Fetch(context.Background(), srv.URL, "agekey"); err != nil {
		t.Fatal(err)
	}
	age, err := f.Age("agekey")
	if err != nil {
		t.Fatal(err)
	}
	if age < 0 || age > time.Minute {
		t.Fatalf("age out of range: %v", age)
	}
}

func TestFetcher5xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := NewFetcher(store, srv.Client())
	if _, err := f.Fetch(context.Background(), srv.URL, "k"); err == nil {
		t.Fatal("expected error on 500")
	}
}
