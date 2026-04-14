# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Cache dependencies first.
COPY go.mod go.sum* ./
RUN go mod download

# Copy source and build.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=mod -ldflags="-w -s" -o /server .

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && \
    adduser  -S appuser -G appgroup && \
    mkdir -p /logs /data && \
    chown -R appuser:appgroup /logs /data

COPY --from=builder /server /server

USER appuser

EXPOSE 50051 8080

VOLUME ["/logs", "/data"]

ENTRYPOINT ["/server"]
