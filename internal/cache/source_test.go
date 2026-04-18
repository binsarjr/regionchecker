package cache

import "testing"

func TestSourceRegistry(t *testing.T) {
	s, ok := Get("nro")
	if !ok {
		t.Fatal("nro missing")
	}
	if s.Name != "nro" || s.Key == "" || s.URL == "" {
		t.Fatalf("nro entry invalid: %+v", s)
	}
	if _, ok := Get("NRO"); !ok {
		t.Fatal("case-insensitive lookup failed")
	}
	for _, name := range []string{"apnic", "arin", "ripe", "lacnic", "afrinic", "psl"} {
		if _, ok := Get(name); !ok {
			t.Fatalf("%s missing", name)
		}
	}
	if _, ok := Get("bogus"); ok {
		t.Fatal("bogus should not exist")
	}
}

func TestSourceAllImmutable(t *testing.T) {
	m := All()
	if len(m) == 0 {
		t.Fatal("empty sources")
	}
	delete(m, "nro")
	if _, ok := Get("nro"); !ok {
		t.Fatal("mutating All() affected registry")
	}
}
