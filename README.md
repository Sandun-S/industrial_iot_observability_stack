# Industrial IoT Observability Stack

**MQTT → InfluxDB → Grafana** — Open source IoT observability for homelab and industrial environments.

[![Build and Push](https://github.com/Sandun-S/industrial-iot-observability-stack/actions/workflows/build-push.yml/badge.svg)](https://github.com/Sandun-S/industrial-iot-observability-stack/actions/workflows/build-push.yml)

## Overview

A lightweight, Docker Swarm-native observability stack that collects data from MQTT-enabled sensors and devices, stores it in InfluxDB, and visualizes it in Grafana — all with automatic dashboard creation.

- **📡 MQTT Reader** — Subscribes to MQTT topics, extracts values via JSON path or CSV mapping, writes to InfluxDB
- **💾 InfluxDB v1** — Time-series database for sensor readings
- **📈 Grafana** — Pre-provisioned dashboards with auto-discovery
- **🌐 Web UI** — Manage readers, sensors, and dashboards from a browser

## Architecture

```
MQTT Broker → MQTT Reader (Go) → InfluxDB v1 → Grafana
                   ↑                  ↑
              Web UI (Go) ← embedded static frontend
```

## Quick Start

See the [deploy repository](https://github.com/Sandun-S/industrial-iot-observability-stack-deploy) for one-command setup:

```bash
git clone https://github.com/Sandun-S/industrial-iot-observability-stack-deploy.git
cd industrial-iot-observability-stack-deploy
./scripts/setup.sh
```

## Repository Structure

```
├── mqtt-reader/          # Go MQTT→InfluxDB reader service
│   ├── config/           # YAML config loading
│   ├── influx/           # InfluxDB line protocol client
│   ├── mqtt/             # MQTT message handler + topic matching
│   ├── logger/           # Logrus-based logger
│   └── Dockerfile        # Multi-stage (golang → alpine)
│
├── web-ui/               # Go web backend + embedded frontend
│   ├── handlers/         # API handlers (readers, sensors, influx, grafana)
│   ├── grafana/          # Dashboard JSON template generator
│   ├── docker/           # Docker Swarm service management
│   ├── config/           # Environment config
│   ├── static/           # Embedded HTML/CSS/JS frontend
│   └── Dockerfile        # Multi-stage build
│
├── .github/workflows/    # CI/CD pipelines
│   ├── build-push.yml    # Multi-arch build + push to ghcr.io
│   └── release.yml       # Semantic version releases
│
└── scripts/              # Development scripts
    └── local-dev.sh      # Local Docker Compose dev environment
```

## Development

### Prerequisites
- Go 1.22+
- Docker

### Build

```bash
# MQTT Reader
cd mqtt-reader
go mod tidy
CGO_ENABLED=0 go build -o mqtt-reader .

# Web UI
cd web-ui
go mod tidy
CGO_ENABLED=0 go build -o web-ui .
```

### Run Locally

```bash
# Start InfluxDB for testing
docker run -d --name influxdb-test -p 8086:8086 \
  -e INFLUXDB_DB=iiot influxdb:1.8

# Run MQTT Reader (needs MQTT broker + config)
cd mqtt-reader
CONFIG_PATH=config/example.yaml go run .

# Run Web UI
cd web-ui
PORT=8080 INFLUX_URL=http://localhost:8086 go run .
```

## CI/CD

This repo uses GitHub Actions for:
- **Multi-arch builds** — `linux/amd64` and `linux/arm64` (Raspberry Pi)
- **Matrix builds** — mqtt-reader and web-ui built in parallel
- **GitHub Container Registry** — Images pushed to `ghcr.io`
- **Integration tests** — Docker compose + InfluxDB verification
- **Semantic releases** — Triggered by `v*` tags

## License

MIT — see [LICENSE](LICENSE) file.

---

Built with ❤️ for the Industrial IoT community.
