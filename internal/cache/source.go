package cache

import "strings"

// Source describes a remote dataset the fetcher knows how to retrieve.
// ParseFn is optional; callers may leave it nil and parse raw bytes directly.
type Source struct {
	Name    string
	URL     string
	Key     string
	ParseFn func([]byte) (any, error)
}

var sources = map[string]Source{
	"nro": {
		Name: "nro",
		URL:  "https://www.nro.net/wp-content/uploads/apnic-uploads/delegated-extended",
		Key:  "nro-delegated-stats",
	},
	"apnic": {
		Name: "apnic",
		URL:  "https://ftp.apnic.net/stats/apnic/delegated-apnic-latest",
		Key:  "delegated-apnic-latest",
	},
	"arin": {
		Name: "arin",
		URL:  "https://ftp.arin.net/pub/stats/arin/delegated-arin-latest",
		Key:  "delegated-arin-latest",
	},
	"ripe": {
		Name: "ripe",
		URL:  "https://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-latest",
		Key:  "delegated-ripencc-latest",
	},
	"lacnic": {
		Name: "lacnic",
		URL:  "https://ftp.lacnic.net/pub/stats/lacnic/delegated-lacnic-latest",
		Key:  "delegated-lacnic-latest",
	},
	"afrinic": {
		Name: "afrinic",
		URL:  "https://ftp.afrinic.net/pub/stats/afrinic/delegated-afrinic-latest",
		Key:  "delegated-afrinic-latest",
	},
	"psl": {
		Name: "psl",
		URL:  "https://publicsuffix.org/list/public_suffix_list.dat",
		Key:  "public_suffix_list.dat",
	},
}

// Get returns the registered source by name (case-insensitive).
func Get(name string) (Source, bool) {
	s, ok := sources[strings.ToLower(name)]
	return s, ok
}

// All returns a copy of the registered sources map.
func All() map[string]Source {
	out := make(map[string]Source, len(sources))
	for k, v := range sources {
		out[k] = v
	}
	return out
}
