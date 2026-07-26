# syntax=docker/dockerfile:1
# Explicit image tags are used instead of digests because this repository does not
# maintain a verified digest ledger. Pin digests in the release pipeline once
# they have been recorded and reviewed there.
FROM --platform=$BUILDPLATFORM node:22.15.1-alpine3.21 AS front-builder
WORKDIR /app/frontend

# Keep dependency installation in its own layer: npm ci validates package-lock.json
# and produces the exact dependency tree captured by the lockfile.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.3-alpine3.22 AS backend-builder
WORKDIR /app

ARG TARGETARCH
ARG TARGETVARIANT
# Supply a reviewed release tag and the SHA-256 of the matching architecture
# asset. Intentionally no defaults: an unpinned Cronet binary must not build.
ARG CRONET_VERSION
ARG CRONET_SHA256

ENV CGO_ENABLED=0 \
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE" \
    GOARCH=$TARGETARCH \
    GOOS=linux \
    CC=gcc

RUN apk add --no-cache \
    ca-certificates \
    curl \
    gcc \
    libc-dev \
    make \
    git

# Keep Go module resolution cacheable independently from application sources.
COPY go.mod go.sum ./
RUN go mod download

# Cronet is a pre-built external binary, so pin both its release tag and digest.
# CRONET_SHA256 is architecture-specific; provide the digest for TARGETARCH.
RUN set -eux; \
    case "$TARGETARCH" in amd64|arm64|arm) ;; *) echo "unsupported Cronet architecture: $TARGETARCH" >&2; exit 1 ;; esac; \
    test -n "$CRONET_VERSION"; \
    test "${#CRONET_SHA256}" -eq 64; \
    printf '%s' "$CRONET_SHA256" | grep -Eq '^[0-9A-Fa-f]{64}$'; \
    cronet_url="https://github.com/SagerNet/cronet-go/releases/download/${CRONET_VERSION}/libcronet-linux-${TARGETARCH}.so"; \
    echo "Downloading pinned Cronet release ${CRONET_VERSION} for ${TARGETARCH}"; \
    curl --fail --location --retry 3 --proto '=https' --tlsv1.2 --output /tmp/libcronet.so "$cronet_url"; \
    printf '%s  %s\n' "$CRONET_SHA256" /tmp/libcronet.so | sha256sum -c -; \
    install -m 0755 /tmp/libcronet.so /app/libcronet.so; \
    rm -f /tmp/libcronet.so

COPY . ./
COPY --from=front-builder /app/frontend/dist/ /app/frontend/dist/

RUN set -eux; \
    if [ "$TARGETARCH" = "arm" ]; then export GOARM=7; [ "$TARGETVARIANT" = "v6" ] && export GOARM=6; fi; \
    go build -ldflags="-w -s" \
      -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,with_tailscale" \
      -o sui main.go

FROM alpine:3.22.6 AS runtime
LABEL org.opencontainers.image.authors="alireza7@gmail.com"
ENV TZ=Asia/Tehran
WORKDIR /app

# Do not use apk --upgrade: it makes the runtime package set drift between builds.
# Root is retained because bind-mounted db/cert directories may be host-owned and
# nftables requires NET_ADMIN (constrained in docker-compose.yml).
RUN apk add --no-cache bash tzdata ca-certificates nftables
COPY --from=backend-builder /app/sui /app/libcronet.so /app/
COPY entrypoint.sh /app/
RUN chmod 0755 /app/entrypoint.sh
ENTRYPOINT ["./entrypoint.sh"]
