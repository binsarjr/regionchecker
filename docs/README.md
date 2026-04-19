# regionchecker — Documentation

Offline-first IP / domain → country classifier. Go CLI + HTTP service.

## Index

| Doc | Scope |
|-----|-------|
| [architecture.md](architecture.md) | Components, interfaces, data flow |
| [flowchart.md](flowchart.md) | Mermaid diagrams: branches, ladder, cache |
| [cli.md](cli.md) | Subcommands + flags |
| [http-api.md](http-api.md) | REST endpoints |
| [confidence-tiers.md](confidence-tiers.md) | Tier definitions + when each fires |
| [cache.md](cache.md) | Layout, atomic write, TTL, conditional GET |

## One-line pitch

Given an IP or host, return a country code with a confidence tier. IP branch uses RIR-delegated data (offline, binary search, ~19 ns/op). Host branch runs an early-exit ladder of ccTLD → ASN brand → TLS cert → content scan → RDAP → CT log → Wayback, returning on the first confident signal.

## Examples use dummy hostnames

Docs use `example.com`, `example.co.id`, `alpha.example`, `пример.example` (placeholder IDN). Real IPs (`8.8.8.8`, `1.1.1.1`) kept — public test anchors, not brands.
