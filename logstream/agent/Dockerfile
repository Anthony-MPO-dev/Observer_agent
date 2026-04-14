FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -ldflags="-w -s" -o agent .

# ─── Runtime stage ───────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S logstream && \
    adduser -S -G logstream logstream

ENV TZ=America/Sao_Paulo

COPY --from=builder /build/agent /agent

USER logstream

EXPOSE 9090

ENTRYPOINT ["/agent"]
