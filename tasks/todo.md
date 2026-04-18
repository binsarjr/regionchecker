# regionchecker — Phased Todo

Ref spec: `tasks/plan.md`. Ref R&D: `tasks/rnd.md`.

Setiap phase ≤5 file delta, end dengan **Crosscheck Gate** (plan.md §11).

## Gap Crosscheck (pre-exec)
- [x] Input boundary: validate host length ≤253, strip trailing dot
- [x] CIDR input support (`check 8.8.8.0/24` → iterate or reject) → **decision: reject with error** (out of scope MVP)
- [x] Batch dedup before lookup
- [x] HTTP body max size (1 MiB) + batch max 1000
- [x] Reverse DNS (PTR) — **skip** MVP
- [x] Anycast 1.1.1.1=AU documented in golden
- [x] Stale DB behavior: serve + warn, readyz=503
- [x] Orphan tmp/ cleanup on boot
- [x] License file MIT
- [x] README + CHANGELOG skeleton
- [x] gotchas.md for self-improvement loop

---

## P0 — Scaffold
- [x] `go.mod` (go 1.23, minimum deps)
- [x] `cmd/regionchecker/main.go` (urfave/cli skeleton + version vars)
- [x] `.golangci.yml`
- [x] `.github/workflows/ci.yml`
- [x] `README.md` + `LICENSE` + `CHANGELOG.md` + `tasks/gotchas.md`
- [x] **Gate**: `go build ./...` pass, `go vet ./...` clean

## P1 — Core IP lookup
- [x] `internal/bogon/bogon.go` + test
- [x] `internal/rir/parser.go` + test
- [x] `internal/rir/builder.go` (CIDR decompose) + test
- [x] `internal/rir/sorted.go` (binary search) + test + bench
- [x] `internal/rir/serialize.go` (RCHK binary) + test
- [x] **Gate**: all tests pass -race, bench <1µs lookup (19ns/op measured)

## P2 — Core Domain + Resolver
- [x] `internal/domain/cctld.go` (250 entries) + test
- [x] `internal/domain/idn.go` + test
- [x] `internal/domain/gtld.go` + test
- [x] `internal/domain/psl.go` + test
- [x] `internal/resolver/resolver.go` + test
- [x] **Gate**: tests pass, IDN Punycode correct

## P3 — Cache layer
- [x] `internal/cache/store.go` (atomic write) + test
- [x] `internal/cache/fetch.go` (conditional GET + singleflight) + test
- [x] `internal/cache/lock.go` (flock) + test
- [x] `internal/cache/parsed.go` (mmap snapshot) + test
- [x] `internal/cache/mem.go` + `internal/cache/source.go`
- [x] **Gate**: fake HTTP 304/200 test pass, atomic rename verified

## P4 — Classifier + Output + Clock
- [x] `internal/clock/clock.go`
- [x] `internal/classifier/classifier.go` + test
- [x] `internal/classifier/decision.go` + test (golden merge matrix)
- [x] `internal/output/{text,json,csv}.go` + test (consolidated into output.go)
- [x] `pkg/regionchecker/client.go` (public facade)
- [x] **Gate**: golden matrix pass

## P5 — CLI + Config
- [x] `internal/config/config.go` + `defaults.go` + test
- [x] `cmd/regionchecker/main.go` full wiring
- [x] `cmd/regionchecker/healthcheck.go`
- [x] `cmd/regionchecker/bootstrap.go` (replace entrypoint.sh)
- [x] `cmd/regionchecker/subcommands.go` (check, update-db, cache, bench, version)
- [x] **Gate**: smoke `./regionchecker check 8.8.8.8` → US (verified live + E2E)

## P6 — HTTP Server
- [x] `internal/server/server.go` (graceful shutdown)
- [x] `internal/server/handlers.go` (/v1/check, /v1/batch, /healthz, /readyz)
- [x] `internal/server/middleware.go` (request-id, rate-limit, slog)
- [x] `internal/server/metrics.go` (Prometheus)
- [x] **Gate**: httptest table-driven suite passes (10 tests); live curl smoke in P7

## P7 — Testdata + E2E + Benchmark
- [x] `testdata/delegated-synthetic.txt` (deterministic fixture, hermetic)
- [x] `testdata/hosts-golden.csv`
- [x] `e2e/cli_test.go` (tag e2e) + `e2e/seed/main.go` helper
- [x] **Gate**: E2E pass (8 golden cases), live smoke `./regionchecker check 8.8.8.8` → US

## P8 — ASN Enrichment (gated)
- [x] `internal/asn/mmdb.go` (MaxMind/DB-IP Lite reader)
- [x] `internal/asn/cymru.go` (Team Cymru DNS)
- [x] `internal/asn/orgregex.go` (ID boosters: TELKOM/BIZNET/INDIHOME/LINKNET/CBN)
- [x] classifier `ASNLookup` interface + Result ASN/ASNOrg fields
- [x] **Gate**: gated behind `--mmdb <path>` flag; offline golden unchanged

## P9 — Release (files only; NO tag/push per user auth)
- [x] `.goreleaser.yaml` drop `brews:` block (simple binary releases)
- [x] `Dockerfile` drop `entrypoint.sh` shim, bump golang:1.25-alpine
- [x] `deploy/updater/entrypoint.sh` deleted (logic in `regionchecker bootstrap`)
- [x] `.github/workflows/release.yml` (goreleaser on v* tag, cosign OIDC)
- [x] `.github/workflows/security.yml` (weekly govulncheck + Trivy)
- [x] `CHANGELOG.md` 0.1.0 section
- [x] **Gate**: `goreleaser check` passes (via docker); full test + e2e green
- [ ] User actions remaining: `git tag v0.1.0 && git push --tags` (manual)

---

## Execution log
(filled as phases complete)

- 2026-04-18: P3 complete: cache layer
- 2026-04-18: P1-tail complete: builder_test.go + bench 19ns/op
- 2026-04-18: P2 complete: cctld (248 entries via IANA), suffix, resolver
- 2026-04-18: P4 complete: clock, classifier+decision, output, pkg/regionchecker client; 141 tests pass across 10 pkgs
- 2026-04-18: P5 complete: config, main.go, subcommands (check/update-db/cache/bench/serve), healthcheck, bootstrap; 151 tests pass
- 2026-04-18: P6 complete: server+handlers+middleware+metrics; 161 tests pass across 12 pkgs
- 2026-04-18: P7 complete: testdata synthetic fixture, hosts-golden.csv, e2e/cli_test.go + seed helper; 8 golden cases pass
- 2026-04-18: P8 complete: asn/{mmdb,cymru,orgregex} gated behind --mmdb; 170 tests pass across 14 pkgs
- 2026-04-18: P9 complete: Dockerfile+goreleaser cleanup, release.yml, security.yml, CHANGELOG 0.1.0. User owns tag+push.

## Review

All 9 phases complete. Summary:

- **Tests**: 170 unit + 9 e2e = 179 total across 15 packages, all passing with -race
- **Bench**: rir.LookupIP at 19.17 ns/op, 2 B/op, 1 alloc (target was <1µs → 52× headroom)
- **Binary**: CLI works with 9 subcommands (check, update-db, cache, bench, serve, healthcheck, bootstrap, version)
- **HTTP API**: full REST surface with Prometheus, graceful shutdown, rate limiting
- **ASN enrichment**: optional MMDB + Team Cymru + regex booster, gated behind --mmdb
- **Docker**: distroless-clean (no entrypoint.sh shim), healthcheck wired
- **Release**: goreleaser config valid, workflows in place, awaiting user tag

### Deps added
- gopkg.in/yaml.v3 (config)
- prometheus/client_golang + golang.org/x/time/rate (server)
- oschwald/maxminddb-golang/v2 (asn)

### Files not finished (per plan.md §10 optional gates)
- golangci-lint run (not installed locally; CI handles)
- coverage ≥80% verification (CI threshold in ci.yml)
- Docker `make docker-build` on this machine (can verify later)
- Live integration against real APNIC update-db (user runs manually when ready)

---

## P10 — ASN brand heuristic (FAST, offline)

**Problem**: `tokopedia.com` → IP `47.74.244.18` (Alibaba US) → classifier says US. Actual: Indonesian company hosted abroad. Need ASN org signal to catch ID brand regardless of IP geo.

**Approach**: extend `internal/asn/orgregex.go` with brand map + country. ASN lookup (existing MMDB path) returns org; regex match → boost to country.

### Deliverables
- [ ] `internal/asn/orgregex.go`: add brand map (default embedded + optional `--asn-hints <path>` JSON override). Entries: TOKOPEDIA→ID, BUKALAPAK→ID, GOJEK→ID, SHOPEE→ID (careful SG ambiguity), BLIBLI→ID, TRAVELOKA→ID, DANA→ID, OVO→ID, GOPAY→ID, KLOOK→ID, JNE→ID, LINKNET→ID, MNC→ID, PT_MORA→ID, etc.
- [ ] Rename `BoostCountry` → `BrandCountry` (clearer intent). Update `orgregex_test.go`.
- [ ] `internal/classifier/classifier.go`: pass `asnCC` to `Decide`. Add `ASNCountry` field to `Result`.
- [ ] `internal/classifier/decision.go`: new tier `ConfHighASNBrand` (when asn brand match overrides ip geo). Integrate in `Decide`.
- [ ] `internal/output/output.go`: add `asn_country` to JSON/CSV.
- [ ] `internal/classifier/classifier_test.go`: new test — tokopedia.com, alibaba IP, ASN=TOKOPEDIA → final ID, confidence high-asn-brand.
- [ ] **Gate**: `go test ./... -race` pass. Manual smoke with MMDB.

## P11 — RDAP enrichment (online, gated)

**Problem**: when ASN brand unknown, need RDAP registrant country as fallback.

**Approach**: new `internal/rdap` package. HTTP client with IANA bootstrap TLD→RDAP base URL mapping (embedded snapshot). Aggressive cache on disk (`cache/rdap/<domain>.json`). 2s default timeout. Gate behind `--rdap` flag.

### Deliverables
- [ ] `internal/rdap/bootstrap.go`: embed IANA RDAP bootstrap JSON (v1). Lookup TLD → base URL.
- [ ] `internal/rdap/client.go`: GET `<base>/domain/<name>`, parse entities[].vcardArray for registrant country. Timeout ctx-driven.
- [ ] `internal/rdap/cache.go`: filesystem cache, TTL 7d, atomic write.
- [ ] `internal/rdap/client_test.go`: unit tests (httptest fake server, golden response).
- [ ] `internal/classifier/classifier.go`: `RDAPLookup` interface, `RegistrantCountry` field.
- [ ] `internal/classifier/decision.go`: integrate rdap signal; tiers `ConfHighRDAPAgree`, `ConfMediumRDAPDisagree`.
- [ ] `cmd/regionchecker/subcommands.go`: `--rdap` flag on `check`.
- [ ] **Gate**: tests pass. Smoke with real RDAP for `tokopedia.com`.

## P12 — `--all` flag (full aggregation)

**Problem**: user wants exact location via combined signals.

**Approach**: `--all` enables `--rdap` + auto-detect MMDB + brand heuristic. Decision uses weighted voting across all signals.

### Deliverables
- [ ] `cmd/regionchecker/subcommands.go`: `--all` flag. Auto-resolves MMDB default paths (env `REGIONCHECKER_MMDB`, `~/.local/share/regionchecker/asn.mmdb`, common DB-IP paths). Enables RDAP.
- [ ] `internal/classifier/decision.go`: `DecideAll` (or extend Decide) with voting — if 2+ agree, high; if ASN brand hit, override IP; rdap fills gaps.
- [ ] Update `Result.Reason` to enumerate contributing signals.
- [ ] `README.md`: document `--all`, precedence order, trade-offs.
- [ ] **Gate**: smoke `./bin/regionchecker check --all -o json tokopedia.com` → ID with reason listing all signals.

### Execution log (P10-P12)

- 2026-04-18: P10 complete. ASN brand regex expanded (TOKOPEDIA/BUKALAPAK/GOJEK/TRAVELOKA/BLIBLI/HALODOC/JNE/DETIK/KOMPAS-GRAMEDIA + ISPs). MMDB reader gained ipinfo format support (fields `asn` string + `as_name`). Decide refactored to accept `Signals` struct with asnCC. New tier `high-asn-brand`. Result exposes `ASNCountry`.
- 2026-04-18: P11 complete. `internal/rdap` package: IANA bootstrap embed (69KB), HTTP client with 2-step chain (registry → registrar "related" link for gTLDs), disk cache TTL 7d, vcardArray parser. Classifier gained `RDAPLookup` interface + `RegistrantCountry` field. New tier `high-rdap-registrant`.
- 2026-04-18: P12 complete. `--all` flag autoloads MMDB (env `REGIONCHECKER_MMDB`, `~/.cache/regionchecker/asn.mmdb`, common paths) + enables RDAP. `--rdap` flag standalone. `--rdap-timeout` configurable. Smoke verified: tokopedia.com → ID (RDAP override), google.com → US, bukalapak.com → ID, google.co.id → ID (domain+rdap agree, high), 8.8.8.8 → US.

### Gate — P11/P12
- **Tests**: 192 unit + 9 e2e = 201 total, all -race green
- **Smoke**: `./bin/regionchecker check --all -o json tokopedia.com` → `final_country: ID, confidence: high-rdap-registrant, reason: rdap registrant ID overrides ip US`
- **Artifacts**: bin/regionchecker 17.4M, RDAP cache `~/.cache/regionchecker/rdap/<sha>.json`, autoMMDB resolves from env/cfg/filesystem

## P13 — SSL cert layer + early-exit ladder

**Problem**: RDAP slow (1-2s cold), and ASN brand narrow. User wants **exact** location with **minimum latency**, enrichment default-on.

**Approach**: new `internal/tlscert` package — TLS dial, extract leaf `Subject.Country`. OV/EV certs carry country (CA-validated); DV certs miss. Refactor classifier to early-exit ladder — cheapest signals first, return on first confident answer. Enrichment defaults to ON; `--fast` opts out.

### Deliverables
- [x] `internal/tlscert/client.go` — dial + Subject.C extraction + memCache
- [x] `internal/tlscert/cache.go` — disk cache 7d TTL
- [x] `internal/tlscert/client_test.go` — self-signed cert fixtures, 6 tests
- [x] `internal/classifier/classifier.go` — full refactor to early-exit ladder; new `TLSCertLookup` interface; `CertCountry` field; removed voting-based Decide usage in host branch
- [x] `internal/classifier/decision.go` — add `ConfHighSSLCert` tier (Decide kept for backward compat)
- [x] `internal/classifier/classifier_test.go` — new tests: TLS cert wins on generic TLD; early-exit on ccTLD match (RDAP/TLS not called)
- [x] `internal/output/output.go` — `cert_country` in JSON/CSV
- [x] `pkg/regionchecker/client.go` — `CertCountry` field
- [x] `cmd/regionchecker/subcommands.go` — default-on enrichment, `--fast`, `--no-cert`, `--no-rdap`, `--cert-timeout`; autoMMDB always-on; removed `--all` (now implicit)

### Ladder (host branch)
1. ccTLD + IP agree → `high`, return (~ms)
2. ccTLD ≠ IP + domCC=ID → `medium-domain-id-offshore-host`, return
3. ccTLD ≠ IP + domCC other → `medium-domain-cc-mismatch`, return
4. Generic TLD → ASN brand (offline, µs) → `high-asn-brand`, return
5. Generic TLD → TLS cert Subject.C (~200-800ms cold) → `high-ssl-cert`, return
6. Generic TLD → RDAP registrant (~500-2000ms cold) → `high-rdap-registrant`, return
7. Generic TLD + IP=ID → `medium-generic-tld-id-host`, return
8. Single-signal fallback / unknown

### Smoke results (cold cache)
| Input | FinalCountry | Confidence | Layer hit | Latency |
|-------|--------------|-----------|-----------|---------|
| google.co.id | ID | `medium-domain-id-offshore-host` | 2 | 43ms |
| tokopedia.com | ID | `high-ssl-cert` | 5 | 787ms |
| bukalapak.com | ID | `high-rdap-registrant` | 6 | 861ms |
| 8.8.8.8 | US | `ip-only` | (raw IP) | 0ms |

### Smoke results (warm cache)
- tokopedia.com → 6ms (TLS cert cached)
- `--fast` tokopedia.com → US (ip-only), 3ms

### Gate
- **Tests**: 200 unit + 9 e2e = 209 total, all -race green
- **Build**: `go vet ./...` clean, `go build ./...` success
- **Binary**: 17.4M, added dns.json 69KB embed
