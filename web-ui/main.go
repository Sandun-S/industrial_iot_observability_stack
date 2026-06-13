// Industrial IoT Observability Stack — Web UI Backend
//
// REST API + embedded static frontend for managing the IIoT stack.
// Everything is managed from the browser — no SSH needed.
//
// Env vars:
//   PORT           — HTTP port (default: 8080)
//   INFLUX_URL     — InfluxDB v1 URL (default: http://influxdb:8086)
//   INFLUX_DB      — InfluxDB database name (default: iiot)
//   GRAFANA_URL    — Grafana URL (default: http://grafana:3000)
//   CONFIG_DIR     — Reader YAML config directory (default: /configs)

package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Sandun-S/industrial-iot-observability-stack/web-ui/config"
	"github.com/Sandun-S/industrial-iot-observability-stack/web-ui/handlers"
)

//go:embed static
var staticFS embed.FS

func main() {
	cfg := config.Load()

	log.Printf("=== IIoT Web UI Backend ===")
	log.Printf("Port:       %s", cfg.Port)
	log.Printf("InfluxDB:   %s (db=%s)", cfg.InfluxURL, cfg.InfluxDB)
	log.Printf("Grafana:    %s", cfg.GrafanaURL)
	log.Printf("Config Dir: %s", cfg.ConfigDir)

	os.MkdirAll(cfg.ConfigDir, 0755)

	// ── Handlers ──────────────────────────────────────────────────────────
	influxHandler := handlers.NewInfluxHandler(cfg.InfluxURL, cfg.InfluxDB)
	grafanaHandler := handlers.NewGrafanaHandler(cfg.GrafanaURL, cfg.GrafanaToken)
	readersHandler := handlers.NewReadersHandler(cfg.ConfigDir)

	mux := http.NewServeMux()

	// Health & Status
	mux.HandleFunc("/api/health", handlers.HealthHandler())
	mux.HandleFunc("/api/status", handlers.StatusHandler(influxHandler, grafanaHandler))

	// Reader management — full CRUD
	mux.Handle("/api/readers", readersHandler)
	mux.Handle("/api/readers/", readersHandler)

	// InfluxDB proxy
	mux.HandleFunc("/api/influx/health", influxHandler.Health())
	mux.HandleFunc("/api/influx/measurements", influxHandler.Measurements())
	mux.HandleFunc("/api/influx/query", influxHandler.Query())
	mux.HandleFunc("/api/influx/latest", influxHandler.LatestData())
	mux.HandleFunc("/api/influx/tags", influxHandler.TagValues())

	// Grafana dashboard management
	mux.HandleFunc("/api/grafana/health", grafanaHandler.Health())
	mux.HandleFunc("/api/grafana/dashboards", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			grafanaHandler.ListDashboards()(w, r)
		case http.MethodPost:
			grafanaHandler.CreateDashboard()(w, r)
		case http.MethodDelete:
			grafanaHandler.RemoveDashboard()(w, r)
		default:
			respondMethodNotAllowed(w)
		}
	})
	mux.HandleFunc("/api/grafana/datasource", grafanaHandler.EnsureDatasource())

	// Docker service info (read-only via Docker socket)
	mux.HandleFunc("/api/services", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Service management is available via the Docker socket.",
			"hint":    "Use 'docker service ls' on the host or check the status endpoint.",
		})
	})

	// ── Static files (embedded frontend) ───────────────────────────────────
	staticSub, _ := fs.Sub(staticFS, "static")
	fileServer := http.FileServer(http.FS(staticSub))
	mux.Handle("/", fileServer)

	// ── Start ──────────────────────────────────────────────────────────────
	handler := corsMiddleware(mux)
	log.Printf("Listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// ── Middleware ──────────────────────────────────────────────────────────────

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Response helpers ────────────────────────────────────────────────────────

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respondMethodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Write([]byte(`{"error":"Method Not Allowed"}`))
}

// stripPrefix helps with prefix-based routing
func stripPrefix(p, s string) string {
	return strings.TrimPrefix(s, p)
}
