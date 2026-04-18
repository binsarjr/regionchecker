// Package cache implements the regionchecker on-disk and in-memory cache:
// atomic-write filesystem store, conditional-GET fetcher with singleflight,
// cross-process flock, mmap parsed snapshots, and a generic LRU.
package cache

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Store is a filesystem-backed cache rooted at dir.
//
// Layout:
//
//	<dir>/raw/<key>         raw payload
//	<dir>/raw/<key>.meta    JSON Meta sidecar
//	<dir>/parsed/<name>     parsed binary snapshots (RCHK)
//	<dir>/lock/update.lock  flock file
//	<dir>/tmp/              staging for atomic writes
type Store struct {
	dir string
}

// Meta is the JSON sidecar recorded next to each raw payload.
type Meta struct {
	ETag         string    `json:"etag"`
	LastModified string    `json:"last_modified"`
	SHA256       string    `json:"sha256"`
	FetchedAt    time.Time `json:"fetched_at"`
	Bytes        int64     `json:"bytes"`
}

// New creates the cache directory tree under dir and returns a Store. Missing
// subdirectories are created with mode 0o755.
func New(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("cache: empty dir")
	}
	for _, sub := range []string{"raw", "parsed", "lock", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("cache: mkdir %s: %w", sub, err)
		}
	}
	return &Store{dir: dir}, nil
}

// Dir reports the root cache directory.
func (s *Store) Dir() string { return s.dir }

// RawPath is the path for a raw payload file.
func (s *Store) RawPath(key string) string { return filepath.Join(s.dir, "raw", key) }

// MetaPath is the path for a raw payload's JSON sidecar.
func (s *Store) MetaPath(key string) string { return filepath.Join(s.dir, "raw", key+".meta") }

// ParsedPath is the path for a parsed snapshot by name.
func (s *Store) ParsedPath(name string) string { return filepath.Join(s.dir, "parsed", name) }

// LockPath returns the default update.lock path.
func (s *Store) LockPath() string { return filepath.Join(s.dir, "lock", "update.lock") }

// TmpDir returns the staging tmp dir.
func (s *Store) TmpDir() string { return filepath.Join(s.dir, "tmp") }

// AtomicWrite writes data to raw/<key> atomically: tmp file → fsync →
// rename → fsync parent dir (POSIX). Fails if the tmp write or rename fails.
func (s *Store) AtomicWrite(key string, data []byte) error {
	if key == "" {
		return fmt.Errorf("cache: empty key")
	}
	tmp, err := s.newTmp(key)
	if err != nil {
		return err
	}
	cleanup := tmp
	defer func() {
		if cleanup != "" {
			_ = os.Remove(cleanup)
		}
	}()
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("cache: open tmp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("cache: write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("cache: fsync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("cache: close tmp: %w", err)
	}
	dst := s.RawPath(key)
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("cache: rename: %w", err)
	}
	cleanup = ""
	if err := syncDir(filepath.Dir(dst)); err != nil {
		return err
	}
	return nil
}

// Read returns the raw payload for key.
func (s *Store) Read(key string) ([]byte, error) {
	return os.ReadFile(s.RawPath(key))
}

// Stat returns FileInfo for raw/<key>.
func (s *Store) Stat(key string) (os.FileInfo, error) {
	return os.Stat(s.RawPath(key))
}

// WriteMeta writes the meta sidecar for key atomically.
func (s *Store) WriteMeta(key string, m Meta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("cache: marshal meta: %w", err)
	}
	return s.AtomicWrite(key+".meta", b)
}

// ReadMeta reads and decodes the meta sidecar for key.
func (s *Store) ReadMeta(key string) (Meta, error) {
	var m Meta
	b, err := os.ReadFile(s.MetaPath(key))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("cache: decode meta: %w", err)
	}
	return m, nil
}

// SweepTmp deletes orphan files in tmp/ older than olderThan.
func (s *Store) SweepTmp(olderThan time.Duration) error {
	entries, err := os.ReadDir(s.TmpDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := time.Now().Add(-olderThan)
	var firstErr error
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(s.TmpDir(), e.Name())); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *Store) newTmp(key string) (string, error) {
	var r [8]byte
	if _, err := io.ReadFull(rand.Reader, r[:]); err != nil {
		return "", fmt.Errorf("cache: rand: %w", err)
	}
	name := fmt.Sprintf("%s.%s", filepath.Base(key), hex.EncodeToString(r[:]))
	return filepath.Join(s.TmpDir(), name), nil
}

// syncDir fsyncs a directory so rename durability is persisted. Windows lacks
// a portable directory fsync, so skip there.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("cache: open dir: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("cache: fsync dir: %w", err)
	}
	return nil
}
