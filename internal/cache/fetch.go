package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/sync/singleflight"
)

// Fetcher performs HTTP conditional GET (ETag / If-Modified-Since), writes
// the payload atomically via Store, and collapses concurrent callers via
// singleflight keyed by URL.
type Fetcher struct {
	Client *http.Client
	Store  *Store
	SF     singleflight.Group
}

// NewFetcher returns a Fetcher using a reasonable default client when nil.
func NewFetcher(store *Store, client *http.Client) *Fetcher {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &Fetcher{Client: client, Store: store}
}

// Fetch retrieves url, caching the body under key in Store. If the cached
// meta has ETag / Last-Modified, the request adds conditional headers and a
// 304 response reuses the on-disk body (with a bumped FetchedAt). On 200 the
// body is sha256-hashed and atomically written.
func (f *Fetcher) Fetch(ctx context.Context, url, key string) ([]byte, error) {
	v, err, _ := f.SF.Do(url, func() (any, error) {
		return f.fetchOnce(ctx, url, key)
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// Age reports the time since the cached key was last fetched.
func (f *Fetcher) Age(key string) (time.Duration, error) {
	m, err := f.Store.ReadMeta(key)
	if err != nil {
		return 0, err
	}
	if m.FetchedAt.IsZero() {
		return 0, fmt.Errorf("cache: meta %s missing fetched_at", key)
	}
	return time.Since(m.FetchedAt), nil
}

func (f *Fetcher) fetchOnce(ctx context.Context, url, key string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cache: request: %w", err)
	}
	prev, haveMeta := f.readMetaOk(key)
	if haveMeta {
		if prev.ETag != "" {
			req.Header.Set("If-None-Match", prev.ETag)
		}
		if prev.LastModified != "" {
			req.Header.Set("If-Modified-Since", prev.LastModified)
		}
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cache: do: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotModified && haveMeta:
		body, err := f.Store.Read(key)
		if err != nil {
			return nil, fmt.Errorf("cache: 304 but cache missing: %w", err)
		}
		updated := prev
		updated.FetchedAt = time.Now().UTC()
		if err := f.Store.WriteMeta(key, updated); err != nil {
			return nil, err
		}
		return body, nil
	case resp.StatusCode == http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("cache: read body: %w", err)
		}
		sum := sha256.Sum256(body)
		if err := f.Store.AtomicWrite(key, body); err != nil {
			return nil, err
		}
		m := Meta{
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
			SHA256:       hex.EncodeToString(sum[:]),
			FetchedAt:    time.Now().UTC(),
			Bytes:        int64(len(body)),
		}
		if err := f.Store.WriteMeta(key, m); err != nil {
			return nil, err
		}
		return body, nil
	default:
		return nil, fmt.Errorf("cache: %s %s: status %d", req.Method, url, resp.StatusCode)
	}
}

func (f *Fetcher) readMetaOk(key string) (Meta, bool) {
	m, err := f.Store.ReadMeta(key)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Meta{}, false
		}
		return Meta{}, false
	}
	if _, err := f.Store.Stat(key); err != nil {
		return Meta{}, false
	}
	return m, true
}
