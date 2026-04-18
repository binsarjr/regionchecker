# regionchecker — Gotchas & Lessons

Log mistake patterns + rules preventing repeat. Review before each session.

## Anycast IP documentation
- `1.1.1.1` APNIC-allocated to AU, routed globally via Cloudflare. Golden test MUST assert AU (allocation), NOT US (routing).
- Rule: always describe country = "allocation country", never "server location".

## Distroless no-shell
- `gcr.io/distroless/static-debian12:nonroot` has no `/bin/sh`. `entrypoint.sh` shim fails silently.
- Rule: port shell logic to `regionchecker bootstrap` Go subcommand.

## IPv4-mapped IPv6
- `::ffff:8.8.8.8` must be unwrapped to v4 before bogon/RIR lookup.
- Rule: always call `addr.Unmap()` before dispatch.

## non-power-of-2 IPv4 ranges
- Legacy APNIC blocks can have value=768 (3×256). Split into `/24 + /23` aligned.
- Rule: `builder.go` must handle via largest-aligned-block decomposition.

## Cache atomicity
- `os.Rename` is atomic ONLY on same filesystem. Cache tmp dir MUST live under cache root.
- Rule: never use `os.TempDir()` for staging cache writes.

## Go DNS resolver quirk on Linux
- Default resolver reads `/etc/resolv.conf` but static binary with `-tags netgo` uses pure-Go resolver → check resolv.conf parse errors.
- Rule: if `netgo` tag enabled, document behavior + provide `--dns-servers` flag.

## PSL snapshot vs runtime
- `golang.org/x/net/publicsuffix` embeds snapshot at build time. Stale if build is old.
- Rule: rebuild binary monthly or pin explicit version.

## Go module floor bumped by deps
- `gofrs/flock@v0.13.0` declares `go 1.25.0`; `go mod tidy` raises the module's `go` directive accordingly. Original plan pinned `go 1.23`.
- Rule: when adopting a dep, inspect its `go.mod` floor first. If the plan's minimum is lower, either pin an older compatible dep version or update the plan.

## Singleflight keyed on URL not key
- `cache.Fetcher.SF.Do` is keyed by URL. Two callers requesting same URL with different cache keys would collapse and both write the same payload under different keys — currently a non-issue because URL→key is 1:1 per source, but document the invariant.
- Rule: keep Source.URL and Source.Key in 1:1 correspondence; never share a URL across cache keys.

## 304 requires both body and meta on disk
- If meta exists but raw body was manually deleted, a conditional GET returning 304 is a cache miss we can't recover from without a full refetch.
- Rule: `readMetaOk` verifies both sidecar AND raw file exist before sending conditional headers.

## Directory fsync portability
- `os.File.Sync()` on directory handles is POSIX-specific. Windows returns `EINVAL`.
- Rule: `syncDir` is a no-op on `runtime.GOOS == "windows"`.
