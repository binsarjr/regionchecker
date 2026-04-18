# Changelog

All notable changes to this project will be documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/binsarjr/regionchecker/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/binsarjr/regionchecker/releases/tag/v0.1.0
