# Flowcharts

Mermaid diagrams for the runtime flow. Render in any GitHub/GitLab/Obsidian viewer.

## 1. Top-level dispatch

```mermaid
flowchart TD
    A[Input string] --> B{Parse as netip.Addr?}
    B -- yes --> C[IP branch]
    B -- no  --> D[idna.Lookup.ToASCII + lowercase]
    D --> E{Valid host?}
    E -- no  --> Z[unknown row + reason]
    E -- yes --> F[Host branch]
    C --> R[Result]
    F --> R
    R --> O[Writer: text / JSON / CSV]
```

## 2. IP branch

```mermaid
flowchart TD
    A[netip.Addr] --> U[addr.Unmap]
    U --> B{bogon.Match?}
    B -- hit --> P[Category: reserved / private / cgnat / loopback / linklocal / multicast]
    B -- miss --> R[rir.LookupIP binary search]
    R --> C{cc found?}
    C -- no  --> N[FinalCountry empty, tier=unknown]
    C -- yes --> I[IPCountry, Registry]
    I --> M{MMDB loaded?}
    M -- yes --> A2[asn.ASN -> ASN, ASNOrg, ASNCountry]
    M -- no  --> S[skip]
    A2 --> T[tier = ip-only]
    S  --> T
    P  --> T
    N  --> T
```

## 3. Host branch — early-exit ladder

```mermaid
flowchart TD
    H[host ascii] --> S1[domain.Country: ccTLD / IDN / geo-gTLD / PSL]
    H --> S2[resolver.Resolve v4 + v6 parallel]
    S1 --> D{domainCC set?}
    S2 --> IP[ip.LookupIP per addr]
    D -- yes, IP=domainCC --> T1[high]
    D -- yes, IP != domainCC, domainCC=ID --> T2[medium-domain-id-offshore-host]
    D -- yes, IP != domainCC, other --> T3[medium-domain-cc-mismatch]
    D -- no, generic TLD --> L[Enrichment ladder]
    L --> L1{ASN brand regex?}
    L1 -- hit --> T4[high-asn-brand]
    L1 -- miss --> L2{TLS cert Subject.C?}
    L2 -- hit --> T5[high-ssl-cert]
    L2 -- miss --> L3{Content scan high score?}
    L3 -- hit --> T6[high-content-scan]
    L3 -- miss --> L4{RDAP registrant, not privacy-proxy?}
    L4 -- hit --> T7[high-rdap-registrant]
    L4 -- miss --> L5{crt.sh CT log Subject.C?}
    L5 -- hit --> T8[high-ct-log]
    L5 -- miss --> L6{Wayback snapshot content scan?}
    L6 -- hit --> T9[medium-wayback-snapshot]
    L6 -- miss --> L7{IP geo = ID?}
    L7 -- yes --> T10[medium-generic-tld-id-host]
    L7 -- no  --> T11[ip-only / low-dns-failed / unknown]
```

## 4. Apex fallback

```mermaid
flowchart LR
    H[subdomain.alpha.example.com] --> R{Any enrichment signal?}
    R -- yes --> OK[return tier as normal]
    R -- no  --> P[publicsuffix.EffectiveTLDPlusOne]
    P --> A[alpha.example.com]
    A --> L[Re-run enrichment ladder on apex]
    L --> T[best tier from apex, reason = apex-fallback]
```

## 5. Cache TTL state machine

```mermaid
stateDiagram-v2
    [*] --> Missing
    Missing --> Fresh: first fetch (200)
    Fresh --> Fresh: age < 24h, no network
    Fresh --> Warm: age >= 24h
    Warm --> Fresh: async conditional GET 304/200 success
    Warm --> Stale: age > 72h
    Stale --> Fresh: sync refresh ok
    Stale --> StaleServed: refresh fail (serve + warn, readyz=503)
    StaleServed --> Fresh: next successful refresh
    Missing --> NoData: --offline flag set
```

## 6. Cache conditional GET + atomic write

```mermaid
sequenceDiagram
    participant C as Caller
    participant F as cache.Fetcher
    participant SF as singleflight
    participant FS as Filesystem
    participant H as HTTP origin

    C->>F: Fetch(url, key)
    F->>SF: Do(url)
    SF->>F: single execution per url
    F->>FS: read meta (etag, last-modified, sha256)
    F->>H: GET url + If-None-Match + If-Modified-Since
    alt 304 Not Modified
        H-->>F: 304
        F->>FS: bump meta.fetched_at
        F-->>C: cached bytes
    else 200 OK
        H-->>F: 200 + body
        F->>FS: write tmp/<key>.partial + fsync
        F->>FS: rename tmp -> raw/<key>
        F->>FS: fsync raw dir
        F->>FS: write meta sidecar
        F-->>C: body
    end
```

## 7. HTTP server lifecycle

```mermaid
flowchart TD
    S[serve subcommand] --> B[bootstrap: load RCHK snapshot via mmap]
    B --> R{readyz}
    R -- DB fresh --> L[http.Server.ListenAndServe]
    R -- DB stale --> L2[ListenAndServe, readyz=503 until refresh]
    L --> M[middleware: request-ID, slog, rate-limit per IP LRU]
    M --> H[handlers: /v1/check /v1/batch /healthz /readyz /metrics]
    H --> Sig{SIGTERM / SIGINT?}
    Sig -- yes --> D[signal.NotifyContext + 15s drain]
    D --> X[flush metrics, close resolver cache, exit 0]
```
