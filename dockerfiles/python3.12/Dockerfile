# SPDX-License-Identifier: GPL-3.0-or-later
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/      cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -a -o fusion-runner ./cmd/runner/

FROM python:3.12-slim

RUN useradd -m -u 1000 runner \
    && mkdir -p /tmp/venv \
    && chown runner:runner /tmp/venv

COPY --from=builder /workspace/fusion-runner /fusion-runner

USER runner

ENTRYPOINT ["/fusion-runner"]
