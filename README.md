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
- Docker + Docker Compose

### Quick Start (full stack with one command)

```bash
# Clone and start everything (Mosquitto + InfluxDB + Grafana + MQTT Reader + Web UI)
git clone https://github.com/Sandun-S/industrial-iot-observability-stack.git
cd industrial-iot-observability-stack
./scripts/local-dev.sh up
```

After ~30 seconds:
- **Web UI** → http://localhost:8080
- **Grafana** → http://localhost:3000 (admin/admin)
- **InfluxDB** → http://localhost:8086
- **MQTT** → localhost:1883

All commands:
```bash
./scripts/local-dev.sh up       # Start full stack (build + deploy)
./scripts/local-dev.sh stop     # Stop and remove everything
./scripts/local-dev.sh restart  # Restart all services
./scripts/local-dev.sh logs     # Tail all logs in real time
./scripts/local-dev.sh test     # Publish test MQTT data to verify pipeline
```

### Build Individual Components

```bash
# MQTT Reader
cd mqtt-reader && go mod tidy && CGO_ENABLED=0 go build -o mqtt-reader .

# Web UI
cd web-ui && go mod tidy && CGO_ENABLED=0 go build -o web-ui .
```

### Run Components Individually (outside Docker)

When running outside Docker, use `localhost` instead of Docker service names:

```bash
# Start dependencies via Docker
docker run -d --name mqtt-test -p 1883:1883 eclipse-mosquitto:2
docker run -d --name influxdb-test -p 8086:8086 -e INFLUXDB_DB=iiot influxdb:1.8

# Run MQTT Reader with localhost config
cd mqtt-reader
CONFIG_PATH=config/example.yaml go run .
# (edit config/example.yaml to use tcp://localhost:1883 and http://localhost:8086)

# Run Web UI
cd web-ui
PORT=8080 INFLUX_URL=http://localhost:8086 go run .
```

### Running the MQTT Simulator

```bash
pipx run --spec paho-mqtt python3 examples/mqtt-simulator.py --host localhost --interval 5
# Or: pip install --break-system-packages paho-mqtt
# Or: python3 -m venv .venv && source .venv/bin/activate && pip install paho-mqtt
```

### Grafana Service Account Token (optional)

The Web UI can auto-create Grafana dashboards via the Grafana API. To enable this:

1. Open Grafana → **Administration** → **Users and access** → **Service accounts** → **Add service account**
2. Display name: `Admin role`, Role: **Admin**
3. Click **Create** → **Add service account token** → **Generate token**
4. Copy the token and set it in the Web UI Settings page, or via env var:

```bash
# In docker-compose.dev.yml or docker-compose.yml:
GRAFANA_SERVICE_ACCOUNT_TOKEN: "glsa_your_token_here"
```

Without a token, the Web UI can still list dashboards (read-only via Grafana's anonymous access). The token is only needed for auto-creating dashboards.

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
