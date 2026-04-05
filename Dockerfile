# ─── Build stage ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies first so this layer is cached when only source changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source.
COPY . .

# Version injected at build time. Defaults to "dev"; override with:
#   docker build --build-arg APP_VERSION=go-v0.1.0 ...
ARG APP_VERSION=dev

# Build all three binaries. CGO_ENABLED=0 produces a fully static binary
# (modernc.org/sqlite is pure Go — no C runtime needed).
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w -X main.version=${APP_VERSION}" \
      -o /out/aicarousel-server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -o /out/aicarousel-setup ./cmd/setup && \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -o /out/aicarousel-apikey ./cmd/apikey

# ─── Runtime stage ───────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: needed for TLS to external AI providers.
# tzdata: optional, ensures time zones work correctly.
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy compiled binaries.
COPY --from=builder /out/aicarousel-server  /usr/local/bin/aicarousel-server
COPY --from=builder /out/aicarousel-setup   /usr/local/bin/aicarousel-setup
COPY --from=builder /out/aicarousel-apikey  /usr/local/bin/aicarousel-apikey

# Copy models.json — used as the default models config at runtime.
COPY models.json /app/models.json

# Data directory; the named volume is mounted here by compose.
RUN mkdir -p /app/data

EXPOSE 7123

ENTRYPOINT ["aicarousel-server"]
