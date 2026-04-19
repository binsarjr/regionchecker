# regionchecker — Production System Plan

Source derived from the original R&D notes. Result of orchestrating 4 parallel agents (architecture, cache, docker, testing/CI).

---

## 1. Directory Layout (final)

```
regionchecker/
  cmd/regionchecker/
    main.go                  # urfave/cli wiring, DI root, version vars
    healthcheck.go           # `regionchecker healthcheck --addr` subcommand (Docker HEALTHCHECK)
    bootstrap.go             # `regionchecker bootstrap` (replace entrypoint.sh long-term)
  internal/
    classifier/
      classifier.go          # Classifier interface + default impl
      decision.go            # confidence tiers, reason builder
    rir/
      parser.go              # bufio.Scanner line parse
      sorted.go              # sorted []ipRange + sort.Search
      builder.go             # raw → sorted slice w/ non-power-of-2 CIDR decompose
      serialize.go           # custom binary snapshot (RCHK magic)
    domain/
      cctld.go               # ccTLDMap (~250 entries) + exceptions (uk=GB, weird)
      idn.go                 # idnTLDMap (Punycode keys) + idna.Lookup.ToASCII
      gtld.go                # geoGTLDMap (.tokyo, .berlin, ...)
      psl.go                 # golang.org/x/net/publicsuffix wrapper
    bogon/
      bogon.go               # []netip.Prefix pre-computed, Match() early-exit
    resolver/
      resolver.go            # net.Resolver.LookupNetIP v4+v6 parallel
    cache/
      store.go               # FS store, atomic write, .meta I/O
      fetch.go               # conditional GET (ETag, If-Modified-Since) + singleflight
      lock.go                # gofrs/flock cross-process
      parsed.go              # mmap binary snapshot encode/decode
      mem.go                 # LRU for DNS + online APIs
      source.go              # Source registry (NRO, APNIC, PSL)
    asn/
      mmdb.go                # DB-IP Lite MMDB reader (Phase 2)
      cymru.go               # Team Cymru DNS (Phase 3)
      orgregex.go            # TELKOM/BIZNET/INDIHOME/LINKNET/CBN booster
    server/
      server.go              # http.Server + graceful shutdown
      handlers.go            # /v1/check, /v1/batch, /healthz, /readyz
      middleware.go          # request-ID, slog, rate-limit
      metrics.go             # Prometheus promauto
    config/
      config.go              # Load: flag > env > yaml > default
      defaults.go
    output/
      text.go  json.go  csv.go
    clock/
      clock.go               # Clock interface for injectable time
  pkg/regionchecker/
    client.go                # public library facade
  testdata/
    delegated-apnic-sample.txt
    nro-sample.txt
    psl-snapshot.dat
    hosts-golden.csv
    golden/                  # per-host expected JSON
  deploy/
    k8s/{namespace,configmap,pvc,deployment,service,cronjob,kustomization}.yaml
    updater/entrypoint.sh
  .github/workflows/
    ci.yml  release.yml  security.yml
  Dockerfile .dockerignore docker-compose.yml
  Makefile .goreleaser.yaml .golangci.yml lefthook.yml
  go.mod go.sum README.md CHANGELOG.md LICENSE
```

---

## 2. Core Interfaces

```go
package regionchecker

type Result struct {
    Input          string
    Type           string        // "ip" | "domain"
    Resolved       []netip.Addr
    DomainCountry  string
    DomainSuffix   string        // cctld|idn|geo-gtld|generic|brand
    IPCountry      string
    ASN            uint32
    ASNOrg         string
    Registry       string        // apnic|arin|ripe|lacnic|afrinic
    FinalCountry   string
    Confidence     string        // high|medium|low|unknown + subtype
    Reason         string
    LookupDuration time.Duration
}

type Classifier   interface{ Classify(ctx context.Context, input string) (*Result, error) }
type IPLookup     interface{ Country(ip netip.Addr) (cc string, meta Meta, ok bool) }
type DomainLookup interface{ Country(host string) (cc, suffixType, confidence string) }
type Resolver     interface{ Resolve(ctx context.Context, host string) ([]netip.Addr, error) }
type ASNLookup    interface{ ASN(ctx context.Context, ip netip.Addr) (asn uint32, org string, ok bool) }
type Cache        interface{ Fetch(ctx context.Context, url, key string) ([]byte, error); Age(key string) (time.Duration, error) }
type Clock        interface{ Now() time.Time }

var (
    ErrUnresolvable   = errors.New("regionchecker: host unresolvable")
    ErrBogon          = errors.New("regionchecker: reserved range")
    ErrDBStale        = errors.New("regionchecker: db older than max_age")
    ErrUnknownCountry = errors.New("regionchecker: no country mapping")
    ErrInvalidInput   = errors.New("regionchecker: invalid ip or host")
    ErrNoData         = errors.New("regionchecker: cache empty and offline")
)
```

---

## 3. Data Flow

```
input → parse (ip|host)
  │
  ├─ ip branch
  │    bogon.Match → ErrBogon (early exit)
  │    rir.Country → cc, registry, meta
  │    [asn.ASN]   → asn, org
  │
  └─ host branch
       idna.Lookup.ToASCII → normalize
       domain.Country       → cc, suffixType (via ccTLD+IDN+gTLD map + PSL)
       resolver.Resolve     → []netip.Addr
       foreach addr:
         bogon.Match
         rir.Country
         [asn.ASN]
  │
  ▼
classifier.Decision: merge domain-cc + ip-cc → FinalCountry + Confidence tier:
  high                              (domain cc match ip cc)
  medium-domain-id-offshore-host    (domain .id but ip != ID)
  medium-generic-tld-id-host        (generic TLD, ip = ID)
  low-dns-failed                    (domain only, no DNS)
  ip-only                           (raw IP input)
  │
  ▼
output.Write: text | json | csv
```

---

## 4. CLI Subcommands

| Cmd | Flags | Desc |
|---|---|---|
| `check [host...]` | `--online --country ID --confidence medium+ -o text|json|csv -f file --timeout 5s --concurrency N --offline` | Single/bulk classify |
| `update-db` | `--source nro|apnic|arin|ripe|lacnic|afrinic --force` | Refresh delegated cache |
| `cache info` | — | List keys, age, sha256, size |
| `cache clear` | — | Wipe raw + parsed |
| `serve` | `--listen :8080 --rate-limit 100 --bind 0.0.0.0` | HTTP API |
| `bench` | `--samples 10000` | Lookup latency percentiles |
| `healthcheck` | `--addr URL` | Exit 0/1 (Docker HEALTHCHECK) |
| `bootstrap` | — | Pre-warm cache + exec next cmd |
| `version` | — | Build info |

---

## 5. HTTP API

| Method | Path | Purpose |
|---|---|---|
| GET | `/v1/check?host=...&online=...&country=ID&confidence=medium+` | Single |
| POST | `/v1/batch` body `{hosts:[],country:"",confidence:""}` (max 1000) | Bulk |
| GET | `/healthz` | Liveness |
| GET | `/readyz` | DB loaded + fresh (<48h) |
| GET | `/metrics` | Prometheus |

Middleware: request-ID (ULID), slog JSON, rate-limit `x/time/rate` per IP LRU.
Graceful shutdown: `signal.NotifyContext` + 15s drain.

---

## 6. Cache Layer (critical)

### Layout
```
$XDG_CACHE_HOME/regionchecker/
  raw/{nro-delegated-stats, delegated-apnic-latest, publicsuffix.dat}
  raw/*.meta                           # JSON: etag, last_modified, fetched_at, sha256, bytes
  parsed/{ipv4-ranges.bin, ipv6-ranges.bin, asn-ranges.bin, schema_version}
  lock/update.lock                     # flock cross-process
  tmp/                                 # staging for atomic writes
```

### Conditional GET
`If-None-Match` + `If-Modified-Since` → 304 = bump fetched_at; 200 = atomic write (tmp+fsync+rename+dir.Sync).

### TTL state machine
| Age | Action |
|---|---|
| <24h | fresh, no network |
| 24–72h | serve cache, async refresh |
| >72h | sync refresh; fail → `ErrDBStale` + warn |
| missing | cold fetch; `--offline` → `ErrNoData` |

### Parsed binary snapshot
```
magic "RCHK" | version u32 | count u32 | raw_sha256[32] | reserved[16]
rows: ipv4=12B (start,end,cc[2],registry,status) ; ipv6=36B
```
Target load <20ms for 700k rows via mmap + unsafe.Slice reinterpret.

### Concurrency
`sync.Mutex` per CacheKey + `gofrs/flock` cross-process + `x/sync/singleflight` collapse N callers → 1 HTTP hit.

### In-memory
- DNS LRU 10k, TTL min(record,5m)
- API LRU 5k, TTL 1h
- Bogon: frozen slice, init-once

### UX
`cache info` (meta only, fast) · `cache clear` · `update-db --force` · `--offline`.

### Cold start
Sweep orphan `tmp/` files >5min. Auto `update-db` on missing cache unless `--offline`. Parsed sha256 mismatch → rebuild.

---

## 7. Config Precedence

flag > env `REGIONCHECKER_*` > `$XDG_CONFIG_HOME/regionchecker/config.yaml` > defaults.

```yaml
cache_dir: ~/.cache/regionchecker
db_source: nro
db_max_age: 48h
db_refresh_interval: 24h
online_enabled: false
ipinfo_token: ""
dns_timeout: 3s
dns_servers: []
server: {port: 8080, rate_limit: 100, read_timeout: 10s, write_timeout: 10s, max_batch: 1000}
log_level: info
log_format: json
mmdb_path: ""
asn_org_boosters: [TELKOM, BIZNET, INDIHOME, LINKNET, CBN]
```

---

## 8. Observability

- `log/slog` JSON handler, request-ID via ctx.
- Metrics (promauto):
  - `regionchecker_lookups_total{result,type}`
  - `regionchecker_lookup_duration_seconds{type}` histogram
  - `regionchecker_cache_hit_total{source}` / `_miss_total` / `_refresh_total{source,result}`
  - `regionchecker_cache_age_seconds{source}` / `_size_bytes{source}`
  - `regionchecker_parse_duration_seconds{source}`
  - `regionchecker_http_requests_total{path,code}` / `_duration_seconds`
  - `regionchecker_db_age_seconds` gauge

---

## 9. Deployment (already scaffolded)

Files existing di repo:
- `Dockerfile` — distroless nonroot (uid 65532), pinned digest, static Go binary, CGO off, trimpath, BuildKit cache mounts, HEALTHCHECK via `regionchecker healthcheck`.
- `docker-compose.yml` — `regionchecker` (serve) + `regionchecker-updater` (sidecar) shared volume `regionchecker-cache`, read_only rootfs, tmpfs `/tmp`, cap_drop ALL, no-new-privileges, mem 256m/128m, pids_limit 128, json-file log rotation.
- `deploy/updater/entrypoint.sh` — pre-warm + `updater-loop` 86400s + jitter 3600s.
- `deploy/k8s/` — namespace, configmap, pvc, deployment (2 replicas), service ClusterIP, cronjob (03:00 UTC daily), kustomization. PodSecurity `restricted`, `automountServiceAccountToken: false`, readOnlyRootFilesystem, seccomp RuntimeDefault.
- `.dockerignore` — excludes .git, tasks/, dist/, testdata.
- `Makefile` — build, docker-build/push (buildx amd64+arm64), test -race, lint, bench, release.
- `.goreleaser.yaml` — 3 OS × 2 arch, SBOM, multi-arch manifest, homebrew tap, conventional-commit changelog, cosign keyless, syft SBOM.

### Docker caveat (note)
`entrypoint.sh` requires a shell → distroless has none. Two options:
1. **Long-term (chosen)**: port the logic to a `regionchecker bootstrap` subcommand → remove `entrypoint.sh` from runtime, keep distroless clean.
2. **Short-term**: swap runtime to `alpine:3.20`.

Decision: implement the `bootstrap` subcommand in Phase 5 as part of server mode, then refactor Dockerfile/compose to drop the `entrypoint.sh` shim.

### Pre-publish
- Pin fresh distroless digest: `docker buildx imagetools inspect gcr.io/distroless/static-debian12:nonroot`.
- Create GHCR repo + homebrew tap repo, or drop `brews:` from goreleaser.

---

## 10. Testing

### Pyramid
| Layer | Target |
|---|---|
| Unit table-driven | <50ms/pkg |
| Integration (real fixture) | <500ms |
| E2E (`os/exec` binary) | <5s, tag `e2e` |
| Benchmark | <1μs/lookup, 0 allocs |
| Race | always-on CI |
| Fuzz | `ParseDelegatedLine`, `ParseHost` 5min weekly |

### No-mock policy
- HTTP → `httptest.NewServer` real fixtures
- Time → inject `Clock` interface
- DNS → real resolver, NX via `nxdomain.invalid`
- FS → `t.TempDir()`

### Golden set (`testdata/hosts-golden.csv`)
Covers: public (8.8.8.8=US, 114.114.114.114=CN, 49.0.109.161=ID), anycast edge case (1.1.1.1=AU per APNIC — documented), v4-mapped IPv6, bogons (10.x, 100.64.x, 169.254.x, fe80::, ::1), domain (example.com=US generic, example.co.id=ID high, пример.example=RU IDN placeholder, www.alpha.example=ID SLD placeholder).

### CI workflows
- `ci.yml`: lint (golangci v1.61) + matrix test (ubuntu/macos/windows × go 1.23/1.24) -race -coverprofile, codecov threshold 1% drop fail, bench with `benchstat` PR comment.
- `release.yml`: tag `v*` → goreleaser → GHCR + Docker Hub + GitHub Releases, cosign keyless OIDC, syft SBOM.
- `security.yml`: weekly govulncheck + Trivy (fail HIGH/CRIT) + Dependabot.

### `.golangci.yml`
errcheck, gosec, revive(line-length 120), unconvert, unparam, gocritic, ineffassign, misspell, prealloc, nilerr, nolintlint, govet, staticcheck, gofmt, goimports.

### Pre-commit (`lefthook.yml`)
fmt, imports, golangci (changed files), go test -race.

---

## 11. Crosscheck Checklist (phase gate)

Before marking a phase complete (REQUIRED):
- [ ] `go vet ./...` clean
- [ ] `golangci-lint run` clean
- [ ] `go test -race -count=1 ./...` pass
- [ ] `go test -bench=. -benchmem` within budget
- [ ] Coverage ≥80%, no >1% regression vs main
- [ ] `govulncheck ./...` clean
- [ ] Docker build + `trivy image` (no HIGH/CRIT)
- [ ] Golden set passes
- [ ] Smoke: `./regionchecker check 8.8.8.8` → `US`
- [ ] README updated if behavior changed
- [ ] No `TODO|FIXME|panic|println` introduced
- [ ] No new direct deps without approval
- [ ] Gap scan: all features in `plan.md` covered or flagged

---

## 12. Phases

Each phase: max 5 new/modified files (per user global rules), end with the crosscheck gate in §11.

- **P0**: repo scaffold — `go.mod`, `cmd/regionchecker/main.go` skeleton, `Makefile` verify, `.golangci.yml`, `lefthook.yml`, CI `ci.yml`.
- **P1 (core IP)**: `rir/{parser,sorted,builder,serialize}.go` + `bogon/bogon.go`.
- **P2 (core domain)**: `domain/{cctld,idn,gtld,psl}.go` + `resolver/resolver.go`.
- **P3 (cache)**: `cache/{store,fetch,lock,parsed,mem,source}.go`.
- **P4 (classifier + output)**: `classifier/{classifier,decision}.go` + `output/{text,json,csv}.go` + `clock/clock.go`.
- **P5 (CLI wiring)**: `cmd/regionchecker/{main,healthcheck,bootstrap}.go` + subcommand implementations + `config/{config,defaults}.go`.
- **P6 (server)**: `server/{server,handlers,middleware,metrics}.go`.
- **P7 (testdata + e2e)**: fixtures + golden + E2E suite + benchmark baseline.
- **P8 (asn enrichment)**: `asn/{mmdb,cymru,orgregex}.go`.
- **P9 (release)**: `.goreleaser.yaml` verify, release workflow dry-run, tag v0.1.0, publish.

Each phase merged to main via PR with the crosscheck checklist in §11.
