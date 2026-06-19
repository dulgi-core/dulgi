# -----------------------------------------------------------------------------
# Dulgi node image — multi-stage, distroless-style final image for a small,
# low-attack-surface runtime.
# -----------------------------------------------------------------------------

ARG GO_VERSION=1.23

# ---- build stage ------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache git make build-base linux-headers

WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Build the static binary.
COPY . .
RUN CGO_ENABLED=1 LDFLAGS="-linkmode external -extldflags '-static'" \
    make build DB_BACKEND=goleveldb \
 && cp build/dulgid /usr/local/bin/dulgid

# ---- runtime stage ----------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates jq curl bash \
 && addgroup -S dulgi && adduser -S -G dulgi -h /home/dulgi dulgi

COPY --from=build /usr/local/bin/dulgid /usr/local/bin/dulgid

USER dulgi
WORKDIR /home/dulgi

ENV DAEMON_NAME=dulgid \
    DAEMON_HOME=/home/dulgi/.dulgi

# P2P, RPC, gRPC, gRPC-web, API, Prometheus.
EXPOSE 26656 26657 9090 9091 1317 26660

ENTRYPOINT ["dulgid"]
CMD ["start"]