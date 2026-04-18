package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Locker is a cross-process exclusive lock backed by gofrs/flock.
type Locker struct {
	path string
	fl   *flock.Flock
}

// NewLocker returns a Locker bound to path. The parent directory is created
// if missing.
func NewLocker(path string) *Locker {
	return &Locker{path: path, fl: flock.New(path)}
}

// Acquire blocks until the lock is held or ctx is cancelled / deadline hit.
// Retry cadence is 200ms.
func (l *Locker) Acquire(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("cache: lock mkdir: %w", err)
	}
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		ok, err := l.fl.TryLock()
		if err != nil {
			return fmt.Errorf("cache: trylock: %w", err)
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// Release releases the underlying flock.
func (l *Locker) Release() error {
	if l.fl == nil {
		return nil
	}
	return l.fl.Unlock()
}
