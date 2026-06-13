// Industrial IoT Observability Stack — Web UI Backend
//
// Provides a REST API for managing MQTT reader configs, querying InfluxDB,
// and managing Grafana dashboards. Serves an embedded static frontend.
//
// Env vars:
//
//	PORT           — HTTP port (default: 8080)
//	INFLUX_URL     — InfluxDB v1 URL (default: http://influxdb:8086)
//	INFLUX_DB      — InfluxDB database name (default: iiot)
//	GRAFANA_URL    — Grafana URL (default: http://grafana:3000)
//	GRAFANA_SERVICE_ACCOUNT_TOKEN — optional Grafana API token
//	CONFIG_DIR     — Reader YAML config directory (default: /configs)
package main

import (
	"embed"
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

	// ── Ensure config directory exists ─────────────────────────────────────
	if err := os.MkdirAll(cfg.ConfigDir, 0755); err != nil {
		log.Printf("WARNING: failed to create config dir %s: %v", cfg.ConfigDir, err)
	}

	// ── Setup handlers ─────────────────────────────────────────────────────
	influxHandler := handlers.NewInfluxHandler(cfg.InfluxURL, cfg.InfluxDB)
	grafanaHandler := handlers.NewGrafanaHandler(cfg.GrafanaURL, cfg.GrafanaToken)

	mux := http.NewServeMux()

	// ── API Routes ─────────────────────────────────────────────────────────
	// Health
	mux.HandleFunc("/api/health", handlers.HealthHandler())
	mux.HandleFunc("/api/status", handlers.StatusHandler(influxHandler, grafanaHandler))

	// Readers
	mux.HandleFunc("/api/readers", handlers.ReadersHandler(cfg.ConfigDir))

	// Sensors (within a reader)
	mux.HandleFunc("/api/readers/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/sensors") {
			if r.Method == http.MethodPost {
				handlers.AddSensorHandler(cfg.ConfigDir)(w, r)
			} else {
				handlers.SensorsHandler(cfg.ConfigDir)(w, r)
			}
			return
		}
		// TODO: individual reader CRUD
		respond404(w)
	})

	// InfluxDB
	mux.HandleFunc("/api/influx/health", influxHandler.Health())
	mux.HandleFunc("/api/influx/measurements", influxHandler.Measurements())
	mux.HandleFunc("/api/influx/query", influxHandler.Query())
	mux.HandleFunc("/api/influx/latest", influxHandler.LatestData())
	mux.HandleFunc("/api/influx/tags", influxHandler.TagValues())

	// Grafana
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

	// ── Static files (embedded) ────────────────────────────────────────────
	staticSub, _ := fs.Sub(staticFS, "static")
	fileServer := http.FileServer(http.FS(staticSub))
	mux.Handle("/", fileServer)

	// ── CORS middleware ────────────────────────────────────────────────────
	handler := corsMiddleware(mux)

	// ── Start server ───────────────────────────────────────────────────────
	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// corsMiddleware adds permissive CORS headers for local/homelab use.
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

func respond404(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error":"Not Found"}`))
}

func respondMethodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Write([]byte(`{"error":"Method Not Allowed"}`))
}
