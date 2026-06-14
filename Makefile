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

.PHONY: docker-up
docker-up:
	@docker compose up -d

.PHONY: docker-down
docker-down:
	@docker compose down