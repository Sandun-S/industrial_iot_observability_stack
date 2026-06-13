# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Industrial IoT Observability Stack — open-source MQTT → InfluxDB → Grafana pipeline with auto-provisioned dashboards and a browser-based management UI. Runs on Docker Swarm or Compose on amd64/arm64 (Raspberry Pi).

## Build & Run

```bash
# Full local dev stack (Docker Compose — all services):
./scripts/local-dev.sh up       # Build + start everything
./scripts/local-dev.sh stop     # Stop everything
./scripts/local-dev.sh logs     # Tail logs
./scripts/local-dev.sh test     # Publish test MQTT data

# Build individual Go binaries:
cd mqtt-reader && go mod tidy && CGO_ENABLED=0 go build -o mqtt-reader .
cd web-ui && go mod tidy && CGO_ENABLED=0 go build -o web-ui .
cd to-postgres && go mod tidy && CGO_ENABLED=0 go build -o to-postgres .
```

## Architecture

Three Go services, all multi-stage Docker builds (`golang:1.22-alpine` → `alpine:3.19`), `CGO_ENABLED=0` for static binaries:

**mqtt-reader/** — Subscribes to MQTT topics (Eclipse Paho), extracts values via GJSON paths or CSV column index, writes to InfluxDB v1 using line protocol. Config is a single YAML file per reader instance. One reader per MQTT broker endpoint.

**web-ui/** — Go HTTP server (port 8080) + embedded static frontend (Go `embed`). Manages MQTT reader configs (YAML read/write), proxies InfluxDB queries, creates/updates Grafana dashboards via Grafana REST API. Docker socket mounted for service management.

**to-postgres/** — Standalone exporter. Reads InfluxDB, writes to PostgreSQL/TimescaleDB. HTTP config API on port 8089. Idles if `POSTGRES_URL` is empty.

## Key Patterns

- **Port 8080** — Web UI backend + static frontend
- **Port 3000** — Grafana (provisioned with InfluxDB datasource)
- **Port 8086** — InfluxDB v1.8 (database: `iiot`, no auth)
- **Port 1883** — Mosquitto MQTT broker
- **Port 8089** — to-postgres exporter config API
- All services on `iiot-net` (overlay) or `iiot-dev` (bridge) Docker network
- InfluxDB line protocol format: `measurement,device=X,reading=Y field_key=value timestamp`
- Grafana dashboard auto-creation uses stable UIDs (`iiot-<sanitized-title>`) with `overwrite: true`
- Web UI frontend: ES modules, hash router, no framework — plain HTML/CSS/JS
- CI/CD: GitHub Actions, multi-arch (amd64+arm64 via QEMU), push to ghcr.io
