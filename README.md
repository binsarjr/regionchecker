# regionchecker

Offline-first IP / domain → country classifier. Go CLI + HTTP service.

## Features
- IP → country via RIR delegated files (APNIC, ARIN, RIPE NCC, LACNIC, AFRINIC; NRO combined).
- Domain → country via ccTLD + IDN Punycode + geographic gTLD + Public Suffix List.
- **Early-exit ladder** — cheapest signals first, returns on first confident answer.
- ASN brand heuristic (offline, µs) — MaxMind / DB-IP / ipinfo MMDB auto-detected.
- TLS cert Subject.Country enrichment (OV/EV certs, ~200ms).
- RDAP registrant-country enrichment (gTLD registry → registrar chain, ~500-2000ms).
- Disk cache for TLS + RDAP (7-day TTL, sha256 keyed).
- Confidence tiers: `high`, `high-asn-brand`, `high-ssl-cert`, `high-rdap-registrant`, `medium-domain-id-offshore-host`, `medium-generic-tld-id-host`, `medium-domain-cc-mismatch`, `low-dns-failed`, `ip-only`, `unknown`.
- Bogon / reserved range pre-filter (RFC1918, CGNAT, loopback, link-local, multicast, docs).
- Conditional GET cache (ETag, If-Modified-Since), atomic writes, mmap-backed parsed snapshot.
- CLI + HTTP API + Prometheus metrics.
- Production hardened: distroless Docker, multi-arch, SBOM, cosign.

## Quick start

```bash
make build-linux              # static Linux amd64 + arm64 binaries
./bin/regionchecker update-db
./bin/regionchecker check 8.8.8.8
./bin/regionchecker check -o json tokopedia.com
# Default enrichment enabled: auto-MMDB + TLS cert + RDAP
# → final_country: ID, confidence: high-ssl-cert (~800ms cold, ~5ms cached)

# Strict offline/fast mode (skip TLS + RDAP):
./bin/regionchecker check --fast tokopedia.com
# → ip-only (US in this case, since IP geo doesn't know the brand)

# Opt out per source:
./bin/regionchecker check --no-rdap tokopedia.com    # TLS cert only
./bin/regionchecker check --no-cert tokopedia.com    # RDAP only
```

### Early-exit ladder (host branch)
1. **ccTLD + IP agree** → `high` (returns ~ms)
2. **ccTLD ≠ IP**, `.id` → `medium-domain-id-offshore-host`
3. **ccTLD ≠ IP**, other → `medium-domain-cc-mismatch`
4. **Generic TLD** → ASN brand (offline, µs) → `high-asn-brand`
5. **Generic TLD** → TLS cert Subject.C (~200ms) → `high-ssl-cert`
6. **Generic TLD** → RDAP registrant (~500-2000ms) → `high-rdap-registrant`
7. **Generic TLD + IP=ID** → `medium-generic-tld-id-host`
8. Single-signal fallback / unknown

### Auto-MMDB paths
`$REGIONCHECKER_MMDB` → config `mmdb_path` → `$XDG_CACHE_HOME/regionchecker/asn.mmdb` → `~/.cache/regionchecker/asn.mmdb` → `/usr/share/GeoIP/GeoLite2-ASN.mmdb`.

## VPS deploy

```bash
make package-linux VERSION=v0.1.0
scp dist/regionchecker-v0.1.0-linux-amd64.tar.gz user@vps:/tmp/
ssh user@vps 'tar xzf /tmp/regionchecker-*.tar.gz -C /usr/local/bin/'
```

Binary adalah static (CGO off, pure Go DNS resolver), jalan di semua Linux kernel umum tanpa glibc dependency.

## Docker

```bash
docker compose up -d
curl http://localhost:8080/v1/check?host=tokopedia.com
```

## Licensing of data sources
- RIR delegated files: public domain / RIR terms, free commercial use.
- Public Suffix List: Mozilla MPL.
- Bundle optional: DB-IP Lite (CC BY 4.0) untuk city/ASN.

## Documentation
- Design: [`tasks/plan.md`](tasks/plan.md)
- R&D: [`tasks/rnd.md`](tasks/rnd.md)
- Phases: [`tasks/todo.md`](tasks/todo.md)

## License
MIT — see [`LICENSE`](LICENSE).
