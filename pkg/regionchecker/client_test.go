package regionchecker_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/binsarjr/regionchecker/internal/rir"
	"github.com/binsarjr/regionchecker/pkg/regionchecker"
)

type stubIP struct{}

func (stubIP) LookupIP(ip netip.Addr) (string, rir.Meta, bool) {
	if ip.String() == "8.8.8.8" {
		return "US", rir.Meta{Registry: "arin"}, true
	}
	return "", rir.Meta{}, false
}

func TestClient_RawIP(t *testing.T) {
	c := regionchecker.New(stubIP{}, nil, nil)
	r, err := c.Classify(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if r.FinalCountry != "US" {
		t.Errorf("FinalCountry = %q, want US", r.FinalCountry)
	}
	if r.Registry != "arin" {
		t.Errorf("Registry = %q, want arin", r.Registry)
	}
}

func TestClient_Bogon(t *testing.T) {
	c := regionchecker.New(stubIP{}, nil, nil)
	_, err := c.Classify(context.Background(), "10.0.0.1")
	if !errors.Is(err, regionchecker.ErrBogon) {
		t.Errorf("err = %v, want ErrBogon", err)
	}
}

func TestClient_InvalidInput(t *testing.T) {
	c := regionchecker.New(stubIP{}, nil, nil)
	_, err := c.Classify(context.Background(), "")
	if !errors.Is(err, regionchecker.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}
