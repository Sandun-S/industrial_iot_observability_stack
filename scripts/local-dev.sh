#!/usr/bin/env bash
# Industrial IoT Observability Stack — Local Development Helper
#
# Starts the full stack locally via Docker Compose for development and testing.
#
# Usage:
#   ./scripts/local-dev.sh          # Start everything
#   ./scripts/local-dev.sh stop     # Stop everything
#   ./scripts/local-dev.sh logs     # Tail logs

set -euo pipefail
cd "$(dirname "$0")/.."

case "${1:-up}" in
  up|start)
    echo "=== Starting IIoT Dev Stack ==="
    docker compose -f docker-compose.dev.yml up -d --build
    echo ""
    echo "Waiting for services..."
    sleep 5
    echo ""
    echo "═══════════════════════════════════════════"
    echo "  📡 Web UI:    http://localhost:8080"
    echo "  📈 Grafana:   http://localhost:3000  (admin/admin)"
    echo "  💾 InfluxDB:  http://localhost:8086"
    echo "  📨 MQTT:      localhost:1883"
    echo "═══════════════════════════════════════════"
    echo ""
    echo "Publish test data:"
    echo "  docker exec \$(docker ps -qf name=mosquitto) mosquitto_pub -t 'iiot/test' -m '{\"value\": 42.5}'"
    echo ""
    echo "View logs:  ./scripts/local-dev.sh logs"
    echo "Stop:       ./scripts/local-dev.sh stop"
    ;;

  stop|down)
    echo "=== Stopping IIoT Dev Stack ==="
    docker compose -f docker-compose.dev.yml down -v
    ;;

  logs)
    docker compose -f docker-compose.dev.yml logs -f
    ;;

  restart)
    echo "=== Restarting IIoT Dev Stack ==="
    docker compose -f docker-compose.dev.yml restart
    ;;

  test)
    echo "=== Publishing test MQTT data ==="
    MOSQUITTO_CID=$(docker ps -qf "name=mosquitto" | head -1)
    if [ -z "$MOSQUITTO_CID" ]; then
      echo "ERROR: mosquitto container not found. Run: ./scripts/local-dev.sh up"
      exit 1
    fi
    for i in 1 2 3 4 5; do
      VALUE=$(echo "scale=1; 20+$RANDOM/1000" | bc 2>/dev/null || echo "23.$((RANDOM % 10))")
      docker exec "$MOSQUITTO_CID" mosquitto_pub -t 'iiot/test' -m "{\"value\": $VALUE}" && echo "  [OK] iiot/test -> {\"value\": $VALUE}"
      sleep 1
    done
    echo ""
    echo "Test data published. Verify:"
    echo "  curl -s 'http://localhost:8086/query?db=iiot' --data-urlencode 'q=SELECT * FROM \"environment\" ORDER BY time DESC LIMIT 5'"
    echo "  Open http://localhost:3000 -> Explore -> InfluxDB -> SELECT * FROM environment"
    ;;

  *)
    echo "Usage: $0 {up|stop|logs|restart|test}"
    exit 1
    ;;
esac
