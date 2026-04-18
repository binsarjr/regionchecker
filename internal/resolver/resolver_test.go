package resolver_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/binsarjr/regionchecker/internal/resolver"
)

func TestResolveNXDomain(t *testing.T) {
	r := resolver.New(5*time.Second, nil)
	_, err := r.Resolve(context.Background(), "nxdomain.invalid")
	if !errors.Is(err, resolver.ErrUnresolvable) {
		t.Fatalf("expected ErrUnresolvable, got %v", err)
	}
}

func TestResolvePublic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}
	r := resolver.New(5*time.Second, nil)
	addrs, err := r.Resolve(context.Background(), "dns.google")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addrs) < 1 {
		t.Fatalf("expected at least 1 address, got 0")
	}
}

func TestResolveTrailingDot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}
	r := resolver.New(5*time.Second, nil)
	addrs, err := r.Resolve(context.Background(), "dns.google.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addrs) < 1 {
		t.Fatalf("expected at least 1 address, got 0")
	}
}

func TestResolveHostTooLong(t *testing.T) {
	r := resolver.New(2*time.Second, nil)
	host := strings.Repeat("a", 254)
	_, err := r.Resolve(context.Background(), host)
	if !errors.Is(err, resolver.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestResolveEmptyHost(t *testing.T) {
	r := resolver.New(2*time.Second, nil)
	_, err := r.Resolve(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error for empty host, got nil")
	}
	if !errors.Is(err, resolver.ErrInvalidInput) && !errors.Is(err, resolver.ErrUnresolvable) {
		t.Fatalf("expected ErrInvalidInput or ErrUnresolvable, got %v", err)
	}
}

func FuzzResolveHost(f *testing.F) {
	f.Add("dns.google")
	f.Add("nxdomain.invalid")
	f.Add("")
	f.Add(strings.Repeat("a", 254))
	f.Fuzz(func(t *testing.T, host string) {
		r := resolver.New(2*time.Second, nil)
		// just ensure no panic
		_, _ = r.Resolve(context.Background(), host)
	})
}
