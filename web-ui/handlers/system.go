package handlers

import (
	"net/http"
)

// HealthHandler returns a simple health check.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "iiot-web-ui",
			"version": "0.1.0",
		})
	}
}

// StatusHandler returns system status for all components.
func StatusHandler(influxHandler *InfluxHandler, grafanaHandler *GrafanaHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := map[string]any{
			"web_ui":  "ok",
			"version": "0.1.0",
		}

		// Check InfluxDB
		influxStatus := "unknown"
		if resp, err := http.Get(influxHandler.InfluxURL + "/ping"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == 204 {
				influxStatus = "ok"
			} else {
				influxStatus = "error"
			}
		} else {
			influxStatus = "unreachable"
		}
		status["influxdb"] = map[string]any{
			"status": influxStatus,
			"url":    influxHandler.InfluxURL,
			"db":     influxHandler.InfluxDB,
		}

		// Check Grafana
		grafanaStatus := "unknown"
		if resp, err := http.Get(grafanaHandler.GrafanaURL + "/api/health"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				grafanaStatus = "ok"
			} else {
				grafanaStatus = "error"
			}
		} else {
			grafanaStatus = "unreachable"
		}
		status["grafana"] = map[string]any{
			"status": grafanaStatus,
			"url":    grafanaHandler.GrafanaURL,
		}

		respondJSON(w, http.StatusOK, status)
	}
}
