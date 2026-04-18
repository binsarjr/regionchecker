SHELL      := /bin/bash
BINARY     := regionchecker
PKG        := ./cmd/regionchecker
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)
IMAGE      ?= ghcr.io/binsarjr/regionchecker
PLATFORMS  ?= linux/amd64,linux/arm64

.PHONY: help build build-linux build-linux-amd64 build-linux-arm64 build-all package-linux test lint bench docker-build docker-push release clean tidy fmt vet

help:
	@awk 'BEGIN{FS=":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n",$$1,$$2}' $(MAKEFILE_LIST)

build: ## Build local static binary (host OS/arch)
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

build-linux-amd64: ## Static Linux amd64 (baseline x86-64, jalan di kernel lama/umum VPS)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 \
	  go build -trimpath -ldflags '$(LDFLAGS) -extldflags "-static"' \
	  -tags netgo,osusergo \
	  -o bin/$(BINARY)-linux-amd64 $(PKG)

build-linux-arm64: ## Static Linux arm64 (untuk ARM VPS / Ampere / Graviton)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
	  go build -trimpath -ldflags '$(LDFLAGS) -extldflags "-static"' \
	  -tags netgo,osusergo \
	  -o bin/$(BINARY)-linux-arm64 $(PKG)

build-linux: build-linux-amd64 build-linux-arm64 ## Build semua varian Linux untuk VPS deploy
	@echo "--- Binaries ---"
	@ls -lh bin/$(BINARY)-linux-* 2>/dev/null || true
	@file bin/$(BINARY)-linux-* 2>/dev/null || true

build-all: build-linux ## Alias build-linux
	@ls -lh bin/

package-linux: build-linux ## Bundle tar.gz per arch (siap scp ke VPS)
	@mkdir -p dist
	@tar -C bin -czf dist/$(BINARY)-$(VERSION)-linux-amd64.tar.gz $(BINARY)-linux-amd64
	@tar -C bin -czf dist/$(BINARY)-$(VERSION)-linux-arm64.tar.gz $(BINARY)-linux-arm64
	@cd dist && sha256sum $(BINARY)-$(VERSION)-linux-*.tar.gz > SHA256SUMS
	@ls -lh dist/

test: ## Run tests with race detector
	go test -race -count=1 -covermode=atomic -coverprofile=coverage.out ./...

lint: ## Run golangci-lint
	golangci-lint run --timeout 5m ./...

bench: ## Run benchmarks
	go test -run=^$$ -bench=. -benchmem ./...

fmt: ## Format sources
	gofmt -s -w .
	goimports -w .

vet: ## go vet
	go vet ./...

tidy: ## Tidy go.mod
	go mod tidy

docker-build: ## Multi-arch image via buildx (does not push)
	docker buildx build \
	  --platform $(PLATFORMS) \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  --build-arg BUILD_DATE=$(BUILD_DATE) \
	  --tag $(IMAGE):$(VERSION) \
	  --tag $(IMAGE):latest \
	  --load \
	  .

docker-push: ## Multi-arch build and push
	docker buildx build \
	  --platform $(PLATFORMS) \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  --build-arg BUILD_DATE=$(BUILD_DATE) \
	  --tag $(IMAGE):$(VERSION) \
	  --tag $(IMAGE):latest \
	  --sbom=true \
	  --provenance=true \
	  --push \
	  .

release: ## Full release via goreleaser
	goreleaser release --clean

release-snapshot: ## Snapshot build without publish
	goreleaser release --snapshot --clean --skip=publish

clean: ## Remove build artefacts
	rm -rf bin dist coverage.out
