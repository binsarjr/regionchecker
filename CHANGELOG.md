# Changelog

All notable changes to this project will be documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.2] - 2026-04-19
### Added
- **Certificate Transparency layer** (`internal/ctlog`) — queries
  crt.sh for historical certificates issued to the host, parses the
  first N (oldest first, biased toward OV/EV from pre-DV era), returns
  the first non-empty `Subject.Country`. Catches legacy enterprises
  and regulated sectors (banking, gov) that currently serve DV but
  previously held OV/EV certificates.
- New confidence tier: `high-ct-log`.
- Result field: `CTLogCountry` (JSON `ctlog_country`, CSV column).
- CLI flags: `--no-ctlog`, `--ctlog-timeout`.

### Discussed but not implemented
- HackerTarget / URLScan / CommonCrawl: these are subdomain-enumeration
  APIs; they don't produce country signals, so they're outside the
  classifier's scope.
- VirusTotal domain_siblings: requires an API key and indirect at best.
  Deferred pending a config-backed key rollout.

## [0.2.1] - 2026-04-18
### Added
- **Apex fallback** — when a subdomain (e.g., typo'd hostname) has no
  DNS/enrichment signal, the classifier retries TLS cert, content scan,
  RDAP, and Wayback against the registrable parent (via PSL).
- **Wayback Machine layer** (`internal/wayback`) — fetches the nearest
  archived snapshot and scores its body with content-scan detectors.
  Catches expired/unreachable domains. Flags `--no-wayback` /
  `--wayback-timeout`. 7-day disk cache.
- New confidence tier: `medium-wayback-snapshot`.
- Result field: `WaybackCountry` (JSON `wayback_country`, CSV column).

### Changed
- **Never-error classification**: `classifyHost` no longer returns
  `ErrUnresolvable`; instead surfaces a row with `unknown` confidence
  and the diagnostic reason so batch (CSV/JSON) consumers get
  consistent output. CLI also converts residual errors (bogon,
  invalid input) into `unknown` rows.
- DNS-fail path now walks RDAP-on-input → apex enrichment →
  Wayback → ccTLD → unknown (was: ccTLD → error).

### Fixed
- `dasarpemrogramangolangs.novalagung.com` (subdomain typo) now
  classifies to ID via apex RDAP instead of erroring out.

## [0.2.0] - 2026-04-18
### Added
- **Multi-signal enrichment ladder** — cheapest signals first, early-exit
  on first confident answer. Default-on, `--fast` opts out.
- `internal/rdap` — IANA bootstrap embed, registry→registrar "related"
  link chain, disk cache 7d, privacy-proxy filter (Cloudflare, Domains
  By Proxy, WhoisGuard, etc.) drops poisoned registrant data.
- `internal/tlscert` — TLS dial + leaf `Subject.Country` extraction,
  disk cache 7d. Catches OV/EV-certified brands hidden behind CDNs.
- `internal/contentscan` — HTTP body fetch + per-country detector
  scoring. Ships ID/SG/MY/GB/JP/US detectors (lang attr, phone prefix,
  ccTLD refs, cities, legal entity `PT`/`Pte Ltd`/`Sdn Bhd`/`株式会社`,
  currency). Rescues Cloudflare-fronted privacy-proxied sites.
- ASN brand regex expanded (TOKOPEDIA, BUKALAPAK, GOJEK, TRAVELOKA,
  BLIBLI, HALODOC, JNE, DETIK, KOMPAS-GRAMEDIA + carriers).
- MMDB reader supports both MaxMind and ipinfo schemas.
- New confidence tiers: `high-asn-brand`, `high-ssl-cert`,
  `high-content-scan`, `high-rdap-registrant`.
- Result fields: `ASNCountry`, `CertCountry`, `ContentCountry`,
  `RegistrantCountry` exposed in JSON/CSV output.
- CLI flags: `--fast`, `--no-cert`, `--no-scan`, `--no-rdap`,
  `--cert-timeout`, `--scan-timeout`. `autoMMDB` always-on.
- `testdata/indo-generic-tld.txt` — 74 Indonesian companies on generic
  TLDs for integration testing.
- `docs/flow.html` + `docs/flow.pdf` — 8-page flow report.

### Changed
- `classifier.Decide` now receives `Signals` struct (growth-friendly).
- Host branch rewritten as explicit early-exit ladder (was Decide-driven).
- Removed `--all` flag (enrichment now default-on).

### Fixed
- Cloudflare-proxied Indonesian sites (e.g. `widyasecurity.com`) now
  resolve to ID via content scan instead of misclassifying as US.
- Traveloka and similar sites where RDAP returns privacy-proxy's
  country now fall through to content scan for true origin.

## [0.1.0] - 2026-04-18
### Added
- RIR-based IP→country lookup with bogon filtering and 19ns/op binary search.
- Delegated-stats parser + RCHK binary snapshot format (mmap load).
- Domain→country dispatcher: ccTLD (248 IANA entries), IDN Punycode, geo-gTLDs, PSL.
- DNS resolver wrapper with 5min LRU cache, custom DNS servers, IPv4-mapped unmap.
- Classifier merging domain and IP signals with confidence tiers
  (`high`, `medium-domain-id-offshore-host`, `medium-generic-tld-id-host`,
  `medium-domain-cc-mismatch`, `low-dns-failed`, `ip-only`, `unknown`).
- CLI subcommands: `check`, `update-db`, `cache info|clear`, `bench`, `serve`,
  `healthcheck`, `bootstrap`, `version`.
- HTTP API: `/v1/check`, `/v1/batch` (max 1000), `/healthz`, `/readyz`, `/metrics`
  with Prometheus, request-ID, per-IP rate limiting, graceful 15s shutdown.
- Optional ASN enrichment (`--mmdb`): MaxMind/DB-IP Lite reader, Team Cymru DNS
  client, ID carrier org-name booster (TELKOM/BIZNET/INDIHOME/LINKNET/CBN).
- Config precedence: flag > env `REGIONCHECKER_*` > YAML > defaults.
- Bootstrap subcommand replaces `entrypoint.sh` (distroless-compatible).
- E2E test suite (8 golden cases), 170 unit tests across 14 packages.
- CI workflows: lint + multi-OS test (ci.yml), release (release.yml),
  security weekly (security.yml: govulncheck + Trivy).

### Changed
- Dockerfile: Go 1.25, dropped shell `entrypoint.sh` shim.
- `.goreleaser.yaml`: removed Homebrew tap (simple binary releases only).

### Infrastructure
- Deploy scaffolding: Dockerfile (distroless nonroot), docker-compose, k8s manifests,
  goreleaser (3 OS × 2 arch, cosign, syft SBOM), Makefile (build, docker, release).

[Unreleased]: https://github.com/binsarjr/regionchecker/compare/v0.2.2...HEAD
[0.2.2]: https://github.com/binsarjr/regionchecker/releases/tag/v0.2.2
[0.2.1]: https://github.com/binsarjr/regionchecker/releases/tag/v0.2.1
[0.2.0]: https://github.com/binsarjr/regionchecker/releases/tag/v0.2.0
[0.1.0]: https://github.com/binsarjr/regionchecker/releases/tag/v0.1.0
