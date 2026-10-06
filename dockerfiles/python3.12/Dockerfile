# syntax=docker/dockerfile:1
# SPDX-License-Identifier: GPL-3.0-or-later

# ##CERTIFY_BUILDER_START##
FROM ubuntu:24.04 AS certify
RUN apt-get update -qq \
    && apt-get install -y --no-install-recommends ca-certificates \
    && update-ca-certificates \
    && rm -rf /var/lib/apt/lists/*
# ##CERTIFY_BUILDER_END##

FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# No third-party dependencies (empty go.sum): nothing to download or vendor.
COPY go.mod go.sum ./

COPY cmd/      cmd/
COPY internal/ internal/

RUN --mount=type=cache,target=/root/.cache/go-build \
    GOFLAGS=-mod=readonly GOPROXY=off CGO_ENABLED=0 GOOS=linux go build -a -o fusion-runner ./cmd/runner/

FROM python:3.12-slim

RUN useradd -m -u 1000 runner \
    && mkdir -p /tmp/venv \
    && chown runner:runner /tmp/venv

# ##CERTIFY_COPY_START##
COPY --from=certify /etc/ssl/certs /etc/ssl/certs
# ##CERTIFY_COPY_END##

COPY --from=builder /workspace/fusion-runner /fusion-runner

USER runner

ENTRYPOINT ["/fusion-runner"]
