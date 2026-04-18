# syntax=docker/dockerfile:1.7

# ---------- Stage 1: builder ----------
FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git ca-certificates tzdata && update-ca-certificates

WORKDIR /src

# Cached module layer
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath \
        -ldflags "-s -w \
          -X main.version=${VERSION} \
          -X main.commit=${COMMIT} \
          -X main.buildDate=${BUILD_DATE}" \
        -o /out/regionchecker ./cmd/regionchecker

# ---------- Stage 2: runtime ----------
# Pin to digest for reproducibility (update when rolling base image)
FROM gcr.io/distroless/static-debian12:nonroot@sha256:d71f4b239be2d412017b798a0a401c44c3049a3ca454838473a4c32ed076bfea

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="regionchecker" \
      org.opencontainers.image.description="Offline-first IP/domain to country classifier" \
      org.opencontainers.image.source="https://github.com/binsarjr/regionchecker" \
      org.opencontainers.image.url="https://github.com/binsarjr/regionchecker" \
      org.opencontainers.image.documentation="https://github.com/binsarjr/regionchecker/blob/main/README.md" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.vendor="binsarjr" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.base.name="gcr.io/distroless/static-debian12:nonroot"

COPY --from=builder /out/regionchecker /usr/local/bin/regionchecker
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# uid 65532 = distroless "nonroot"
USER 65532:65532

ENV RC_CACHE_DIR=/var/cache/regionchecker \
    RC_LISTEN=:8080 \
    RC_LOG_LEVEL=info

VOLUME ["/var/cache/regionchecker"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
    CMD ["/usr/local/bin/regionchecker", "healthcheck", "--addr", "http://127.0.0.1:8080/healthz"]

ENTRYPOINT ["/usr/local/bin/regionchecker"]
CMD ["serve", "--listen", ":8080", "--cache-dir", "/var/cache/regionchecker"]
