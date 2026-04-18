package cache

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLockerBlocksUntilRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a := NewLocker(path)
	b := NewLocker(path)

	if err := a.Acquire(context.Background()); err != nil {
		t.Fatalf("a.Acquire: %v", err)
	}

	type result struct {
		t   time.Time
		err error
	}
	ch := make(chan result, 1)
	go func() {
		err := b.Acquire(context.Background())
		ch <- result{t: time.Now(), err: err}
	}()

	time.Sleep(300 * time.Millisecond)
	select {
	case got := <-ch:
		t.Fatalf("second locker acquired too early: %+v", got)
	default:
	}
	releasedAt := time.Now()
	if err := a.Release(); err != nil {
		t.Fatalf("a.Release: %v", err)
	}

	select {
	case got := <-ch:
		if got.err != nil {
			t.Fatalf("b.Acquire error: %v", got.err)
		}
		if got.t.Before(releasedAt) {
			t.Fatalf("b acquired before a released")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for b to acquire")
	}
	if err := b.Release(); err != nil {
		t.Fatalf("b.Release: %v", err)
	}
}

func TestLockerContextDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a := NewLocker(path)
	b := NewLocker(path)
	if err := a.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer a.Release()

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := b.Acquire(ctx)
	if err == nil {
		_ = b.Release()
		t.Fatal("expected deadline error")
	}
	if time.Since(start) < 200*time.Millisecond {
		t.Fatalf("returned too early: %v", time.Since(start))
	}
}
