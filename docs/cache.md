# Cache

## Layout

```
$XDG_CACHE_HOME/regionchecker/
  raw/
    nro-delegated-stats
    delegated-apnic-latest
    publicsuffix.dat
    *.meta                       # JSON: etag, last_modified, fetched_at, sha256, bytes
  parsed/
    ipv4-ranges.bin              # RCHK snapshot
    ipv6-ranges.bin
    asn-ranges.bin
    schema_version
  tlscert/<sha256>.json           # leaf Subject.C + expiry
  rdap/<sha256>.json              # registrant block
  ctlog/<sha256>.json             # crt.sh historical certs
  wayback/<sha256>.json           # archive snapshot url + body hash
  lock/update.lock                # cross-process flock
  tmp/                            # staging for atomic writes
```

## Atomic write

Every write follows `tmp/<key>.partial` → `fsync(file)` → `rename → raw/<key>` → `fsync(dir)`. `os.Rename` is atomic only on the same filesystem, so the tmp dir must live under the cache root, not `os.TempDir()`.

Directory `fsync` is POSIX-only; on Windows it is a no-op.

## Conditional GET

`cache.Fetcher` sends `If-None-Match` + `If-Modified-Since` derived from the meta sidecar.

- `304 Not Modified` → bump `meta.fetched_at`, return existing bytes.
- `200 OK` → write tmp, atomic rename, update meta.

Required invariant: the `readMetaOk` helper verifies both the meta sidecar **and** the raw file exist before sending conditional headers. A 304 without a body on disk is unrecoverable without a full refetch.

## TTL state machine

| Age | Behavior |
|-----|----------|
| missing | Cold fetch. `--offline` → `ErrNoData`. |
| <24h | Fresh. No network. |
| 24–72h | Serve cache, async conditional GET. |
| >72h | Sync refresh. Fail → serve + warn, `readyz=503`, `ErrDBStale` on demand. |

## Parsed snapshot (RCHK)

```
magic "RCHK" | version u32 | count u32 | raw_sha256[32] | reserved[16]
rows:
  ipv4 row = 12 B (start u32, end u32, cc[2], registry u8, status u8)
  ipv6 row = 36 B
```

Load target <20 ms for ~700k rows via `mmap` + `unsafe.Slice` reinterpret. On `raw_sha256` mismatch the snapshot is rebuilt from raw before service continues.

## Concurrency

- Per-key `sync.Mutex` guards in-process collisions.
- `gofrs/flock` on `lock/update.lock` guards cross-process collisions (updater sidecar vs server).
- `x/sync/singleflight` keyed by URL collapses N callers → 1 HTTP hit. Keep each `Source.URL` in 1:1 correspondence with its cache key; never share a URL across keys.

## Cold start

1. Sweep orphan `tmp/` files older than 5 min.
2. If `raw/` missing and `--offline` is off → trigger `update-db`.
3. Verify parsed `raw_sha256` matches the raw file; mismatch → rebuild.
4. mmap parsed snapshots.

## Enrichment layer caches (7-day TTL)

TLS cert, RDAP, CT log, and Wayback layers each maintain their own sharded filesystem cache keyed by `sha256(host)`. Entries expire after 7 days; reads outside the TTL trigger refetch. Files are written atomically using the same tmp-rename pattern.

## UX commands

| Command | Effect |
|---------|--------|
| `cache info` | Print meta only (fast, no body read) |
| `cache clear` | Wipe `raw/` + `parsed/` + enrichment caches |
| `update-db --force` | Ignore ETag, full refetch |
| `check --offline` | Never hit network, error on miss |
