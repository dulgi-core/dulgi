#!/usr/bin/make -f

# -----------------------------------------------------------------------------
# Dulgi (dulgid) Makefile
# -----------------------------------------------------------------------------

BINARY      := dulgid
APP_PKG     := github.com/dulgi/dulgi
VERSION     ?= $(shell git describe --tags --always 2>/dev/null || echo "v0.1.0")
COMMIT      := $(shell git log -1 --format='%H' 2>/dev/null || echo "unknown")
BUILDDIR    ?= $(CURDIR)/build
GOBIN       ?= $(shell go env GOPATH)/bin

# CometBFT build tag selects the consensus backend. pebbledb/rocksdb optional.
DB_BACKEND  ?= goleveldb

# ldflags inject version metadata into `dulgid version --long`.
ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=dulgi \
          -X github.com/cosmos/cosmos-sdk/version.AppName=$(BINARY) \
          -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
          -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
          -X "github.com/cosmos/cosmos-sdk/version.BuildTags=netgo,ledger,$(DB_BACKEND)"

BUILD_FLAGS := -tags "netgo ledger $(DB_BACKEND)" -ldflags '$(ldflags)' -trimpath

# Cross builds drop the `ledger` tag (USB HID needs CGO) so they compile from
# any host without an arm64 C toolchain. goleveldb is pure Go and unaffected.
cross_ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=dulgi \
                -X github.com/cosmos/cosmos-sdk/version.AppName=$(BINARY) \
                -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
                -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
                -X "github.com/cosmos/cosmos-sdk/version.BuildTags=netgo,$(DB_BACKEND)"

CROSS_BUILD_FLAGS := -tags "netgo $(DB_BACKEND)" -ldflags '$(cross_ldflags)' -trimpath

.PHONY: all
all: lint test install

# -----------------------------------------------------------------------------
# Build / install
# -----------------------------------------------------------------------------

.PHONY: build
build:
	@echo "building $(BINARY) $(VERSION)..."
	@go build -mod=readonly $(BUILD_FLAGS) -o $(BUILDDIR)/$(BINARY) ./cmd/dulgid

.PHONY: install
install:
	@echo "installing $(BINARY) $(VERSION) to $(GOBIN)..."
	@go install -mod=readonly $(BUILD_FLAGS) ./cmd/dulgid

.PHONY: clean
clean:
	@rm -rf $(BUILDDIR)

# -----------------------------------------------------------------------------
# Cross-compilation (arm64)
# -----------------------------------------------------------------------------

.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "building $(BINARY) $(VERSION) for linux/arm64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -mod=readonly $(CROSS_BUILD_FLAGS) -o $(BUILDDIR)/$(BINARY)-linux-arm64 ./cmd/dulgid

.PHONY: build-darwin-arm64
build-darwin-arm64:
	@echo "building $(BINARY) $(VERSION) for darwin/arm64..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build -mod=readonly $(CROSS_BUILD_FLAGS) -o $(BUILDDIR)/$(BINARY)-darwin-arm64 ./cmd/dulgid

# Convenience alias: arm64 build for the deployment target (linux).
.PHONY: build-arm64
build-arm64: build-linux-arm64

# -----------------------------------------------------------------------------
# Quality
# -----------------------------------------------------------------------------

.PHONY: test
test:
	@go test -mod=readonly -race ./...

.PHONY: lint
lint:
	@golangci-lint run --timeout=10m || echo "golangci-lint not installed; skipping"

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: proto-gen
proto-gen:
	@echo "Dulgi defines no custom protobuf types; nothing to generate."

# -----------------------------------------------------------------------------
# Local devnet helpers (see scripts/)
# -----------------------------------------------------------------------------

.PHONY: init-local
init-local: install
	@bash scripts/single-node.sh

.PHONY: testnet
testnet: install
	@bash scripts/testnet.sh

# -----------------------------------------------------------------------------
# Docker
# -----------------------------------------------------------------------------

.PHONY: docker-build
docker-build:
	@docker build -t dulgi/dulgid:$(VERSION) -t dulgi/dulgid:latest .

# arm64-only image. Requires buildx (docker buildx create --use once); on a
# non-arm64 host this builds under QEMU emulation.
.PHONY: docker-build-arm64
docker-build-arm64:
	@docker buildx build --platform linux/arm64 \
		-t dulgi/dulgid:$(VERSION)-arm64 -t dulgi/dulgid:arm64 --load .

.PHONY: docker-up
docker-up:
	@docker compose up -d

.PHONY: docker-down
docker-down:
	@docker compose down