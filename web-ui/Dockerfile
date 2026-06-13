# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o web-ui .

# ── Runtime stage ───────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /app/web-ui .

# Default config — override via environment
ENV PORT=8080
ENV INFLUX_URL=http://influxdb:8086
ENV INFLUX_DB=iiot
ENV GRAFANA_URL=http://grafana:3000
ENV GRAFANA_SERVICE_ACCOUNT_TOKEN=""
ENV CONFIG_DIR=/configs

EXPOSE 8080

ENTRYPOINT ["./web-ui"]
