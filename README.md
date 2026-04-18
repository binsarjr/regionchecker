# regionchecker

Offline-first IP / domain → country classifier. Go CLI + HTTP service.

## Features
- IP → country via RIR delegated files (APNIC, ARIN, RIPE NCC, LACNIC, AFRINIC; NRO combined).
- Domain → country via ccTLD + IDN Punycode + geographic gTLD + Public Suffix List.
- Merge decision with confidence tiers (`high`, `medium-domain-id-offshore-host`, `medium-generic-tld-id-host`, `low-dns-failed`, `ip-only`).
- Bogon / reserved range pre-filter (RFC1918, CGNAT, loopback, link-local, multicast, docs).
- Conditional GET cache (ETag, If-Modified-Since), atomic writes, mmap-backed parsed snapshot.
- CLI + HTTP API + Prometheus metrics.
- Production hardened: distroless Docker, multi-arch, SBOM, cosign.

## Quick start

```bash
make build-linux              # static Linux amd64 + arm64 binaries
./bin/regionchecker update-db
./bin/regionchecker check 8.8.8.8
./bin/regionchecker check tokopedia.com --country ID -o json
```

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
