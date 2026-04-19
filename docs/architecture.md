# Architecture

## Component map

```
cmd/regionchecker/          CLI entry (urfave/cli), subcommand wiring, DI root
internal/
  bogon/                    Pre-filter reserved / private / CGNAT / link-local
  rir/                      RIR delegated parser + sorted range index + RCHK binary snapshot
  domain/                   ccTLD map, IDN Punycode map, geo-gTLD map, PSL wrapper
  resolver/                 net.Resolver.LookupNetIP (v4+v6 parallel) + memCache
  cache/                    Atomic FS store, conditional GET, flock, mmap parsed snapshot
  asn/                      MMDB reader (MaxMind / DB-IP / ipinfo), Team Cymru DNS, brand regex
  tlscert/                  TLS dial + leaf Subject.Country, 7d disk cache
  contentscan/              HTTP body fetch + per-country detector scoring
  rdap/                     IANA bootstrap, registry→registrar chain, privacy-proxy filter
  ctlog/                    crt.sh Certificate Transparency historical cert lookup
  wayback/                  Wayback Machine snapshot fetch + content-scan rescore
  classifier/               Early-exit ladder + tier builder
  server/                   HTTP handlers, middleware, Prometheus metrics
  config/                   Flag > env > YAML > defaults
  output/                   text / JSON / CSV writers
  clock/                    Injectable time source
pkg/regionchecker/          Public library facade
```

## Core interfaces

```go
type Result struct {
    Input             string
    Type              string   // "ip" | "domain"
    Resolved          []netip.Addr
    DomainCountry     string
    DomainSuffix      string   // cctld | idn | geo-gtld | generic | brand
    IPCountry         string
    ASN               uint32
    ASNOrg            string
    ASNCountry        string
    CertCountry       string
    ContentCountry    string
    RegistrantCountry string
    CTLogCountry      string
    WaybackCountry    string
    Registry          string   // apnic | arin | ripe | lacnic | afrinic
    FinalCountry      string
    Confidence        string   // see confidence-tiers.md
    Reason            string
    LookupDuration    time.Duration
}

type Classifier   interface{ Classify(ctx context.Context, input string) (*Result, error) }
type IPLookup     interface{ Country(ip netip.Addr) (cc string, meta Meta, ok bool) }
type DomainLookup interface{ Country(host string) (cc, suffixType, confidence string) }
type Resolver     interface{ Resolve(ctx context.Context, host string) ([]netip.Addr, error) }
type ASNLookup    interface{ ASN(ctx context.Context, ip netip.Addr) (asn uint32, org string, ok bool) }
type TLSCertLookup interface{ Country(ctx context.Context, host string) (cc string, ok bool) }
type RDAPLookup    interface{ Registrant(ctx context.Context, host string) (cc string, ok bool) }
type Cache         interface{ Fetch(ctx context.Context, url, key string) ([]byte, error) }
type Clock         interface{ Now() time.Time }
```

## Data flow (top-level)

1. Input parsed as IP (`netip.ParseAddr`) or host (`idna.Lookup.ToASCII` → lower).
2. IP branch: bogon pre-check → RIR sorted-range lookup → optional ASN enrichment.
3. Host branch: suffix lookup + DNS resolve in parallel → enrichment ladder (see `flowchart.md`).
4. Classifier merges signals into a `Result` with one tier.
5. Writer encodes `Result` as text / JSON / CSV.

## IP branch detail

- `bogon.Match(addr)` checks a pre-frozen `[]netip.Prefix` with early-exit; hits short-circuit to `reserved` / `private` / `cgnat` / `loopback` / `linklocal` / `multicast` with no country output.
- `addr.Unmap()` applied before dispatch so `::ffff:8.8.8.8` uses the v4 table.
- `rir.LookupIP(addr)` binary-searches a sorted slice of `(start,end,cc[2],registry,status)` rows loaded via mmap from the RCHK snapshot. ~19 ns/op, 1 alloc.
- Non-power-of-2 legacy IPv4 blocks decomposed at build time into largest-aligned-CIDR pieces so search invariant holds.

## Host branch detail

See [flowchart.md](flowchart.md) for the visual. Textual summary:

1. **ccTLD + IP agree** → `high`.
2. **ccTLD ≠ IP geo**, domain is `.id` → `medium-domain-id-offshore-host`.
3. **ccTLD ≠ IP geo**, domain is some other country → `medium-domain-cc-mismatch`.
4. **Generic TLD** (`.com` etc) — enter enrichment ladder:
   - ASN brand regex (offline, µs) → `high-asn-brand`.
   - TLS cert leaf `Subject.Country` → `high-ssl-cert`.
   - HTTP content scan (lang, currency, legal entity, phones) → `high-content-scan`.
   - RDAP registrant country (privacy-proxy filtered) → `high-rdap-registrant`.
   - crt.sh CT log historical cert → `high-ct-log`.
   - Wayback Machine snapshot rescored → `medium-wayback-snapshot`.
5. **Apex fallback**: if a subdomain has no signal, PSL → registrable parent and re-run enrichment against that.
6. **Generic TLD + IP=ID** → `medium-generic-tld-id-host`.
7. Nothing hits → `low-dns-failed` / `ip-only` / `unknown`.

Never-error rule: failed classification surfaces an `unknown` row with a diagnostic reason rather than returning an error, so batch (CSV/JSON) output stays consistent.

## Concurrency

- Per-CacheKey `sync.Mutex` + cross-process `gofrs/flock` + `x/sync/singleflight` collapses concurrent fetches to a single HTTP hit.
- Resolver runs v4 and v6 lookups in parallel via `errgroup`.
- Enrichment ladder is sequential (early-exit), but each layer has its own in-memory + disk cache so warm runs skip I/O.

## Observability

- `log/slog` JSON handler, request-ID via context.
- Prometheus metrics (promauto): `regionchecker_lookups_total{result,type}`, `_lookup_duration_seconds{type}`, `_cache_hit_total{source}` / `_miss_total` / `_refresh_total{source,result}`, `_cache_age_seconds{source}`, `_db_age_seconds`, `_http_requests_total{path,code}`.
