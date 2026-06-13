# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o mqtt-reader .

# ── Runtime stage ───────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /app/mqtt-reader .

# Default config path — override via Docker config/volume mount
ENV CONFIG_PATH=/config/reader.yaml
ENV LOG_LEVEL=info

ENTRYPOINT ["./mqtt-reader"]
