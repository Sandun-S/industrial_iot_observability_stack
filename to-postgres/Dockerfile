FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o to-postgres .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl
WORKDIR /app
COPY --from=builder /app/to-postgres .

ENV INFLUX_URL=http://influxdb:8086
ENV INFLUX_DB=iiot
ENV POSTGRES_URL=""
ENV SYNC_INTERVAL=60
ENV SITE=default
ENV API_PORT=8089
EXPOSE 8089

ENTRYPOINT ["./to-postgres"]
