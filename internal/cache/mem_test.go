package cache

import (
	"testing"
	"time"
)

func TestLRUPutGet(t *testing.T) {
	c, err := NewLRU[string, int](4, 0)
	if err != nil {
		t.Fatal(err)
	}
	c.Put("a", 1)
	c.Put("b", 2)
	got, ok := c.Get("a")
	if !ok || got != 1 {
		t.Fatalf("a: got %v %v", got, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("unexpected hit")
	}
}

func TestLRUTTLExpiry(t *testing.T) {
	c, err := NewLRU[string, string](4, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	c.SetClock(func() time.Time { return now })
	c.Put("k", "v")
	if v, ok := c.Get("k"); !ok || v != "v" {
		t.Fatalf("fresh: %v %v", v, ok)
	}
	now = now.Add(200 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected expired entry")
	}
}

func TestLRUEviction(t *testing.T) {
	c, err := NewLRU[int, int](2, 0)
	if err != nil {
		t.Fatal(err)
	}
	c.Put(1, 1)
	c.Put(2, 2)
	c.Put(3, 3) // evicts 1
	if _, ok := c.Get(1); ok {
		t.Fatal("1 should be evicted")
	}
	if v, ok := c.Get(2); !ok || v != 2 {
		t.Fatalf("2: %v %v", v, ok)
	}
	if v, ok := c.Get(3); !ok || v != 3 {
		t.Fatalf("3: %v %v", v, ok)
	}
}

func TestLRURemovePurge(t *testing.T) {
	c, err := NewLRU[string, int](4, 0)
	if err != nil {
		t.Fatal(err)
	}
	c.Put("a", 1)
	c.Put("b", 2)
	c.Remove("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("remove failed")
	}
	c.Purge()
	if c.Len() != 0 {
		t.Fatalf("purge: %d", c.Len())
	}
}
