# regionchecker — R&D Report

Tujuan: CLI Go yang menerima IP/domain → output country/region (+ optional ASN, city). Offline-first, zero-signup default.

---

## 1. Sumber Data: Kategori & Trade-off

| Sumber | Jenis | Akurasi | Lisensi | Signup | Offline | Rekomendasi |
|---|---|---|---|---|---|---|
| **RIR delegated files** (APNIC, ARIN, RIPE, LACNIC, AFRINIC, NRO combined) | Country only | ~100% country-of-allocation | Public, free komersial | Tidak | Ya | **PRIMARY** |
| **MaxMind GeoLite2** | Country+City+ASN | 99.8% country, 60–80% city | EULA proprietary (2019+) | Ya (license key) | Ya | Skip — friction redistribusi |
| **DB-IP Lite MMDB** | Country+City+ASN | Bagus country, lemah mobile | CC BY 4.0 (redistributable) | Tidak | Ya | **SECONDARY bundle** |
| **IP2Location LITE** | Country+City+lat/lng | Setara MaxMind country | CC BY-SA 4.0 | Ya | Ya | Alternatif DB-IP |
| **ipinfo.io** | Country+ASN+company+privacy flags | Tinggi | Free 50k/bulan | Opt (API key) | Tidak | Online enrich |
| **ip-api.com** | Full geo | Sedang | Free 45 req/min, non-komersial | Tidak | Tidak | Skip (limit ketat) |
| **Team Cymru DNS** (`origin.asn.cymru.com`) | ASN only | Tinggi | Free | Tidak | Tidak | ASN fallback |

**Kesimpulan arsitektur**: primary = NRO combined delegated file (country), optional bundle DB-IP Lite MMDB (city+ASN), optional `--online` untuk ipinfo/Cymru.

---

## 2. RIR Delegated File — Format

URL combined (semua RIR jadi satu):
- `https://ftp.ripe.net/pub/stats/ripencc/nro-stats/latest/nro-delegated-stats`
- `https://www.nro.net/wp-content/uploads/delegated-stats/nro-extended-stats` (with opaque-id)

Per-RIR (kalau mau granular):
- APNIC: `https://ftp.apnic.net/stats/apnic/delegated-apnic-latest`
- ARIN: `https://ftp.arin.net/pub/stats/arin/delegated-arin-latest`
- RIPE: `https://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-latest`
- LACNIC: `https://ftp.lacnic.net/pub/stats/lacnic/delegated-lacnic-latest`
- AFRINIC: `https://ftp.afrinic.net/pub/stats/afrinic/delegated-afrinic-latest`

### Format baris (pipe-delimited)

Version:
```
2|apnic|20260417|78234|19830613|20260417|+1000
```

Summary:
```
apnic|*|ipv4|*|52341|summary
```

Record:
```
apnic|CN|ipv4|1.0.1.0|256|20110414|allocated
apnic|AU|ipv6|2001:dc0::|32|20020801|allocated
apnic|KR|asn|9318|1|20021219|allocated
```

Field: `registry | cc (ISO-3166-1 alpha-2) | type (asn|ipv4|ipv6) | start | value | date | status | [opaque-id]`

### Konversi start+value → CIDR

- **IPv4**: `value` = jumlah address (power of 2). `prefix = 32 - log2(value)`.
  - `1.0.0.0, 256` → `/24`
  - Non-power-of-2 (legacy): decompose ke multiple CIDR atau simpan sbg range `[start, start+value-1]`.
- **IPv6**: `value` = prefix length langsung. `2001:dc0::, 32` → `/32`.

### Ukuran & update
- Combined NRO: ~15–20 MB, ~700–900k records.
- Update: daily (~01:00–04:00 UTC).
- Server support `If-Modified-Since` + `ETag` → conditional GET → 304 hemat bandwidth.

---

## 3. Pra-lookup: Bogon / Reserved Ranges

Cek **sebelum** hit dataset — output kategori `private`/`reserved`/`loopback`/`cgnat`/`multicast` langsung tanpa misleading country result.

**IPv4**: `0.0.0.0/8`, `10.0.0.0/8`, `100.64.0.0/10` (CGNAT), `127.0.0.0/8`, `169.254.0.0/16`, `172.16.0.0/12`, `192.0.0.0/24`, `192.0.2.0/24`, `192.88.99.0/24`, `192.168.0.0/16`, `198.18.0.0/15`, `198.51.100.0/24`, `203.0.113.0/24`, `224.0.0.0/4`, `240.0.0.0/4`, `255.255.255.255/32`.

**IPv6**: `::/128`, `::1/128`, `::ffff:0:0/96`, `64:ff9b::/96`, `100::/64`, `2001::/32` (Teredo), `2001:db8::/32`, `fc00::/7` (ULA), `fe80::/10`, `ff00::/8`.

---

## 4. Stack Go (rekomendasi)

| Komponen | Pilihan | Alasan |
|---|---|---|
| IP type | stdlib `net/netip` | value type, zero-alloc, 3x faster parse dari `net.IP` |
| Lookup struct | sorted `[]struct{start,end uint32; cc [2]byte}` + `sort.Search` | ~50–100 ns/lookup untuk 200k entries, zero deps |
| Alt lookup | `github.com/gaissmai/bart` atau `kentik/patricia` | kalau mau CIDR trie native |
| MMDB reader | `github.com/oschwald/maxminddb-golang` | untuk DB-IP Lite bundle |
| Delegated parser | manual `bufio.Scanner` + `strings.Split(line, "|")` | tidak ada lib mature, trivial |
| HTTP | stdlib `net/http` + `If-Modified-Since`/`ETag` | zero dep |
| CLI | `urfave/cli/v2` | ringan, subcommands clean (`check`, `update-db`, `bench`) |
| Cache | `os.UserCacheDir()` → `~/.cache/regionchecker/` | XDG compliant |
| Concurrency | `golang.org/x/sync/errgroup` + `SetLimit` | bulk lookup banyak host |
| DNS | `net.DefaultResolver.LookupNetIP(ctx,"ip",host)` + `x/net/idna` | IPv4+IPv6 serentak, IDN support |
| Output | stdlib `encoding/json` + `encoding/csv` | flag `-o text|json|csv` |

---

## 5. Flow Program

```
input (ip|domain|stdin list)
  │
  ├─ domain? → idna.ToASCII → LookupNetIP → iterate semua Addr
  │
  ▼
netip.Addr
  │
  ├─ bogon match? → output kategori reserved, stop
  │
  ├─ lookup sorted slice (country) ──► { cc, registry, allocated_date, status }
  │
  ├─ --online? → ipinfo / Cymru ASN enrich
  │
  └─ --mmdb bundled? → city + ASN
  │
  ▼
format output (text/json/csv)
```

---

## 6. Edge Cases & Test Golden Set

| IP | Expected | Catatan |
|---|---|---|
| `8.8.8.8` | US | Google |
| `1.1.1.1` | AU (RIR) / US (routing) | Cloudflare anycast — RIR bilang AU |
| `114.114.114.114` | CN | |
| `2001:4860:4860::8888` | US | Google IPv6 |
| `10.0.0.1` | private | bogon early-exit |
| `100.64.0.1` | cgnat | bogon |
| `fe80::1` | link-local | bogon |
| `::ffff:8.8.8.8` | US (via v4-mapped) | unwrap ke v4 |

**Dokumentasikan**: anycast IP menunjukkan country-of-allocation, bukan edge router terdekat — batasan fundamental RIR data.

---

## 7. Subcommands Usulan

```
regionchecker check <ip|domain> [--online] [-o text|json|csv]
regionchecker check -f hosts.txt                 # bulk dari file
regionchecker update-db [--source nro|apnic|...] # refresh cache
regionchecker bench                              # lookup latency report
```

---

## 8. Batasan yang harus disclose

1. Country = **allocation country**, bukan lokasi fisik server saat ini.
2. Tidak ada city/ISP tanpa MMDB bundle.
3. Anycast IP (Cloudflare, Google DNS) → single country padahal global.
4. Recent allocations (< 24h) belum masuk file.
5. Legacy blocks pre-RIR bisa status `ietf`/`reserved` tanpa cc.

---

## 9. Domain-based Classification (ccTLD + IDN + SLD)

**Prinsip**: domain suffix = weak signal of intent/target audience, bukan lokasi server. Join dengan IP-based check supaya tangkap dua arah (domain `.id` offshore-hosted + domain `.com` hosted di ID).

### 9.1 ccTLD → ISO 3166-1 alpha-2
Source: IANA Root Zone DB (https://www.iana.org/domains/root/db) + all-TLD list `https://data.iana.org/TLD/tlds-alpha-by-domain.txt`. Tidak ada file resmi ccTLD→ISO mapping, jadi **hardcode map** (~250 entries, jarang berubah).

Majority 1:1 (`.id`=ID, `.jp`=JP, `.sg`=SG, `.de`=DE). **Exceptions wajib**:

| ccTLD | ISO | Catatan |
|---|---|---|
| `.uk` | GB | ISO = GB bukan UK |
| `.ac` | SH | Ascension; sering dijual global (akademik) |
| `.io` | IO | BIOT; de facto gTLD tech |
| `.tv` | TV | Tuvalu; dipakai media global |
| `.me` | ME | Montenegro; personal brand global |
| `.co` | CO | Colombia; alt `.com` |
| `.ly` | LY | Libya; bit.ly dsb |
| `.tk .ml .ga .cf .gq` | — | Freenom, unreliable |
| `.su` | — | Legacy USSR, bukan ISO |
| `.eu .asia` | — | Regional, bukan country |

**Weird ccTLDs** (`.io .tv .me .co .ly .ai .ws`) → tag **low-confidence** country signal.

### 9.2 IDN ccTLDs (Punycode)
Normalize dulu pakai `golang.org/x/net/idna`:
```go
ascii, _ := idna.Lookup.ToASCII("пример.рф") // → пример.xn--p1ai
```

| IDN | Punycode | ISO |
|---|---|---|
| `.рф` | `xn--p1ai` | RU |
| `.中国` / `.中國` | `xn--fiqs8s` / `xn--fiqz9s` | CN |
| `.香港` | `xn--j6w193g` | HK |
| `.台湾` / `.台灣` | `xn--kprw13d` / `xn--kpry57d` | TW |
| `.한국` | `xn--3e0b707e` | KR |
| `.ไทย` | `xn--o3cw4h` | TH |
| `.مصر` | `xn--wgbh1c` | EG |
| `.السعودية` | `xn--mgberp4a5d4ar` | SA |
| `.امارات` | `xn--mgbaam7a8h` | AE |
| `.ايران` | `xn--mgba3a4f16a` | IR |
| `.ישראל` | `xn--4dbrk0ce` | IL |
| `.ελ` | `xn--qxam` | GR |
| `.срб` | `xn--90a3ac` | RS |

Store Punycode as key (`"xn--p1ai":"RU"`), convert input hostname to ASCII dulu.

### 9.3 Indonesian SLDs (PANDI)
`.co.id`, `.ac.id`, `.go.id`, `.or.id`, `.net.id`, `.sch.id`, `.mil.id`, `.web.id`, `.my.id`, `.biz.id`, `.ponpes.id`, `.desa.id`.

**Untuk country classification cukup cek last label.** `www.unpad.ac.id` → last=`id` → ID. SLD enumeration hanya perlu kalau butuh *registrable domain* (eTLD+1), untuk itu pakai PSL.

Pola mirip di negara lain: `.co.uk/.ac.uk` (UK), `.com.au/.edu.au` (AU), `.co.jp/.ac.jp` (JP), `.com.sg/.edu.sg` (SG).

### 9.4 Public Suffix List (PSL)
- Source: `https://publicsuffix.org/list/public_suffix_list.dat` (Mozilla)
- Go: `golang.org/x/net/publicsuffix` (snapshot built-in, update via go.mod)
```go
etld, _ := publicsuffix.PublicSuffix("www.example.co.id")        // "co.id"
reg, _  := publicsuffix.EffectiveTLDPlusOne("www.example.co.id") // "example.co.id"
```
**Batasan**: PSL hanya beri boundary registrable, **bukan** country mapping. Country tetap dari last-label hardcoded map.

### 9.5 Geographic gTLDs
- Regional: `.eu`, `.asia` — no country.
- City/region gTLD → strong hint: `.tokyo`→JP, `.berlin`→DE, `.london`→GB, `.paris`→FR, `.nyc`→US, `.moscow`→RU, `.wien`→AT, `.sydney`→AU. Simpan di secondary map.
- Untuk Indonesia: **tidak ada `.jakarta`/`.bali`** di root zone (per 2026-04). Cuma `.id` + SLDs.
- Generic (`.com .net .org .info .biz .xyz .top .online .site`) → no country, **must** fall back to IP.

### 9.6 Brand TLDs
`.google .apple .bmw` dsb → skip, rely on IP.

### 9.7 Decision Tree `is_indonesia(host)`
```go
func ClassifyID(host string) (country, confidence string) {
    ascii, err := idna.Lookup.ToASCII(strings.TrimSuffix(host, "."))
    if err != nil { return "", "unknown" }
    ascii = strings.ToLower(ascii)

    labels := strings.Split(ascii, ".")
    last := labels[len(labels)-1]
    cc := ccTLDMap[last] // "id" → "ID", "xn--p1ai" → "RU"

    ips, _ := net.DefaultResolver.LookupNetIP(ctx, "ip", ascii)
    ipCC := ""
    for _, ip := range ips {
        if c := rirLookup(ip); c != "" { ipCC = c; break }
    }

    switch {
    case cc == "ID" && ipCC == "ID":
        return "ID", "high"
    case cc == "ID" && ipCC != "" && ipCC != "ID":
        return "ID", "medium-domain-id-offshore-host"
    case cc == "" && ipCC == "ID":
        return "ID", "medium-generic-tld-id-host"
    case cc == "ID":
        return "ID", "low-dns-failed"
    default:
        return ipCC, "ip-only"
    }
}
```

**Rule**: classify as ID kalau **either** domain suffix `id` **or** resolved IP di ID range. Confidence tier menjelaskan mismatch (offshore hosting / generic TLD). User yang mau tangkap "semua dari ID" pakai union kedua signal.

### 9.8 Signal Tambahan (tie-breaker, opsional)
- WHOIS registrant country — rate-limited, per-registrar inconsistent, slow. Skip default.
- HTTP `Content-Language`, `Server` header.
- TLS cert `Subject.C` / `Subject.O`.
- HTML `<html lang="id">`.
- **ASN org name regex** (`TELKOM`, `BIZNET`, `INDIHOME`, `LINKNET`, `CBN`) → booster ID-specific. Worth adding ke RIR lookup karena gratis.

### 9.9 Go Implementation Hints
- Hardcode `ccTLDMap` + `idnTLDMap` + `geoGTLDMap` (3 maps, ~300 entries total) → tidak pakai dep library countries.
- `golang.org/x/net/publicsuffix` untuk eTLD+1 extraction.
- `golang.org/x/net/idna` untuk IDN ASCII normalize (pakai `idna.Lookup` profile — lenient).
- Refresh monthly: fetch `tlds-alpha-by-domain.txt` → diff dengan map → log unknown TLD (trigger manual review).
- DNS resolver timeout (3–5s), handle NXDOMAIN → `dns-failed` confidence tier.

---

## 10. Output Schema (updated)

```json
{
  "input": "tokopedia.com",
  "type": "domain",
  "resolved": ["49.0.109.161"],
  "domain_country": null,
  "domain_suffix_type": "generic",
  "ip_country": "ID",
  "asn": null,
  "registry": "apnic",
  "final_country": "ID",
  "confidence": "medium-generic-tld-id-host",
  "reason": "generic TLD .com resolved to ID-allocated IP range"
}
```

---

## Rekomendasi Final (updated)

**MVP Phase 1**:
- NRO combined delegated file (IP → country).
- Hardcoded `ccTLDMap` + IDN Punycode map + geo gTLD map (domain → country).
- `golang.org/x/net/publicsuffix` + `idna` untuk parsing.
- Bogon pre-filter.
- Decision tree merge domain-signal + ip-signal → confidence tier.
- CLI `check`/`update-db`.

**Phase 2**: Bundle DB-IP Lite MMDB untuk city/ASN offline. ASN org-name regex booster (`TELKOM` dsb).

**Phase 3**: `--online` flag → ipinfo.io + Team Cymru DNS ASN fallback. TLS cert + HTML lang signal untuk tie-breaker.

**Use case "capture all ID"**:
```
regionchecker check -f hosts.txt --country ID --confidence medium+ -o json
```
→ output semua host yang domain ID **OR** IP di ID allocation, dengan confidence ≥ medium.

Tunggu approval sebelum code.
