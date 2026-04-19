# CLI

## Subcommands

| Command | Purpose |
|---------|---------|
| `check [host|ip ...]` | Single or bulk classify |
| `update-db` | Refresh RIR delegated cache |
| `cache info` | List cache keys, ages, sha256, sizes |
| `cache clear` | Wipe raw + parsed cache |
| `serve` | Start HTTP server |
| `bench` | Lookup latency percentiles |
| `healthcheck` | Exit 0/1 probe (Docker HEALTHCHECK) |
| `bootstrap` | Pre-warm cache + exec next cmd |
| `version` | Build info |

## Common flags on `check`

| Flag | Default | Description |
|------|---------|-------------|
| `-o text\|json\|csv` | `text` | Output format |
| `-f <file>` | — | Read hosts from file (one per line) |
| `--timeout 5s` | `5s` | Per-host overall timeout |
| `--concurrency N` | CPU count | Parallel workers for bulk |
| `--country ID` | — | Filter rows to country code |
| `--confidence medium+` | — | Filter by tier floor (`high`, `medium+`, `low+`) |
| `--offline` | off | Fail instead of network fetch on cache miss |
| `--fast` | off | Skip all enrichment (TLS / RDAP / content / CT / Wayback) |
| `--no-cert` | off | Skip TLS cert layer |
| `--no-scan` | off | Skip content-scan layer |
| `--no-rdap` | off | Skip RDAP layer |
| `--no-ctlog` | off | Skip crt.sh CT log layer |
| `--no-wayback` | off | Skip Wayback Machine layer |
| `--cert-timeout 3s` | `3s` | TLS dial timeout |
| `--scan-timeout 3s` | `3s` | HTTP content fetch timeout |
| `--rdap-timeout 3s` | `3s` | RDAP request timeout |
| `--ctlog-timeout 3s` | `3s` | crt.sh query timeout |
| `--wayback-timeout 3s` | `3s` | Wayback fetch timeout |
| `--mmdb <path>` | auto | MaxMind / DB-IP / ipinfo ASN MMDB |
| `--dns-servers host:port,...` | system | Override resolver |

## Examples

```bash
# Raw IP
./bin/regionchecker check 8.8.8.8

# Host with enrichment default-on
./bin/regionchecker check -o json example.com

# Strict offline / fast (IP geo only for host)
./bin/regionchecker check --fast example.com

# Batch from file, filter ID + medium or better, JSON
./bin/regionchecker check -f hosts.txt --country ID --confidence medium+ -o json

# Refresh delegated data
./bin/regionchecker update-db --source nro

# Inspect cache
./bin/regionchecker cache info
```

## Auto-MMDB resolution order

1. `--mmdb <path>` flag
2. `$REGIONCHECKER_MMDB` env
3. `mmdb_path` in config YAML
4. `$XDG_CACHE_HOME/regionchecker/asn.mmdb`
5. `~/.cache/regionchecker/asn.mmdb`
6. `/usr/share/GeoIP/GeoLite2-ASN.mmdb`

First readable match wins. Absent → ASN enrichment silently skipped.

## Config precedence

`flag` > `env REGIONCHECKER_*` > `$XDG_CONFIG_HOME/regionchecker/config.yaml` > compiled defaults.
