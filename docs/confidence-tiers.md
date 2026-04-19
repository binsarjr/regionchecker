# Confidence Tiers

Each `Result.Confidence` is one of the tiers below. Tiers are ordered rough-strongest-first.

## High tiers

| Tier | Fires when |
|------|-----------|
| `high` | ccTLD present and IP geo matches the same country. Strongest signal. |
| `high-asn-brand` | IP ASN org matches a known brand regex (offline, µs). Catches brands hosted abroad. |
| `high-ssl-cert` | TLS leaf `Subject.Country` resolves a country. OV/EV certs are CA-validated; DV certs usually miss. |
| `high-content-scan` | Rescues privacy-proxied sites. HTTP body hits per-country detectors (lang attr, currency, legal entity markers, phone prefixes, city names) above threshold. |
| `high-rdap-registrant` | RDAP registry → registrar chain returns a registrant country that is **not** a known privacy proxy (Cloudflare, Domains By Proxy, WhoisGuard, etc.). |
| `high-ct-log` | crt.sh Certificate Transparency history returns a historical OV/EV cert with a `Subject.Country`. Useful for legacy enterprises now on DV. |

## Medium tiers

| Tier | Fires when |
|------|-----------|
| `medium-domain-id-offshore-host` | ccTLD is `.id` but resolved IP geo is a different country. Common pattern for Indonesian brands behind CDNs. |
| `medium-domain-cc-mismatch` | ccTLD is some other country, IP geo differs. Report the ccTLD country with caveat. |
| `medium-generic-tld-id-host` | Generic TLD (`.com` etc) but IP geo is ID and no richer signal fired. |
| `medium-wayback-snapshot` | Last-resort rescue: Wayback Machine nearest archived snapshot passes content-scan detectors above threshold. |

## Low / informational

| Tier | Fires when |
|------|-----------|
| `low-dns-failed` | ccTLD gave a country hint but DNS failed to resolve. |
| `ip-only` | Input was a raw IP, or host had no ccTLD / enrichment hit but a resolvable IP. |
| `unknown` | No signal produced a country. Row still emitted (never-error rule) with a diagnostic `reason`. |

## Ladder enforcement

The host branch returns on the first confident layer; lower-priority layers are not invoked. This means the `Result` only carries the field(s) of the winning layer plus whatever was computed before the short-circuit. For example, a `high-ssl-cert` result will usually have `CertCountry` set but not `RegistrantCountry` or `CTLogCountry`.

## Example mapping

| Input | Likely tier | Why |
|-------|-------------|-----|
| `8.8.8.8` | `ip-only` | Raw IP |
| `1.1.1.1` | `ip-only`, country AU | RIR says AU (allocation), not US (routing) |
| `example.co.id` (resolves to ID IP) | `high` | ccTLD + IP agree |
| `example.co.id` (resolves to US IP) | `medium-domain-id-offshore-host` | ccTLD ID, IP elsewhere |
| `example.com` (US brand, US IP) | `high-ssl-cert` or `high-rdap-registrant` | Generic TLD, enrichment resolves |
| `subdomain.alpha.example` | Apex-fallback path | PSL → `alpha.example`, re-run ladder |
| Dead domain with archived page | `medium-wayback-snapshot` | Wayback rescore |
