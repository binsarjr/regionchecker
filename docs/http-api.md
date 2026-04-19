# HTTP API

Start with `regionchecker serve --listen :8080`.

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/v1/check?host=...` | Single classify |
| POST | `/v1/batch` | Bulk classify (max 1000) |
| GET | `/healthz` | Liveness probe |
| GET | `/readyz` | DB loaded + fresh (age ≤ 48h) |
| GET | `/metrics` | Prometheus exposition |

## `GET /v1/check`

Query params:

| Param | Example | Notes |
|-------|---------|-------|
| `host` | `example.com` or `8.8.8.8` | Required |
| `fast` | `1` | Skip enrichment |
| `country` | `ID` | Filter |
| `confidence` | `medium+` | Filter |

Response (JSON, one Result row):

```json
{
  "input": "example.com",
  "type": "domain",
  "resolved": ["203.0.113.42"],
  "domain_country": null,
  "domain_suffix_type": "generic",
  "ip_country": "US",
  "asn": 64496,
  "asn_org": "EXAMPLE-AS",
  "asn_country": null,
  "cert_country": "US",
  "registry": "arin",
  "final_country": "US",
  "confidence": "high-ssl-cert",
  "reason": "TLS leaf Subject.C matches IP geo"
}
```

## `POST /v1/batch`

Request body (JSON):

```json
{
  "hosts": ["example.com", "alpha.example", "8.8.8.8"],
  "country": "ID",
  "confidence": "medium+"
}
```

Max 1000 hosts. Response is a JSON array of Result rows in input order.

## Middleware

- Request-ID (ULID) echoed back in `X-Request-Id`.
- `log/slog` JSON access log.
- Per-IP token-bucket rate limit (`x/time/rate`) keyed by LRU.
- Graceful shutdown: `signal.NotifyContext` + 15s drain window.

## Metrics

`regionchecker_http_requests_total{path,code}`, `regionchecker_http_request_duration_seconds{path}`, plus all classifier / cache metrics listed in [architecture.md](architecture.md).

## Sample curl

```bash
curl 'http://localhost:8080/v1/check?host=example.com'
curl -X POST -H 'content-type: application/json' \
  -d '{"hosts":["example.com","example.co.id"]}' \
  http://localhost:8080/v1/batch
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics | head
```
