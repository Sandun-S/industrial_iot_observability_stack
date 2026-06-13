package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Sandun-S/industrial-iot-observability-stack/web-ui/grafana"
)

// GrafanaHandler manages Grafana dashboard operations via the Grafana REST API.
type GrafanaHandler struct {
	GrafanaURL   string
	ServiceToken string // optional API token
}

// NewGrafanaHandler creates a new Grafana API handler.
func NewGrafanaHandler(grafanaURL, serviceToken string) *GrafanaHandler {
	return &GrafanaHandler{
		GrafanaURL:   strings.TrimRight(grafanaURL, "/"),
		ServiceToken: serviceToken,
	}
}

// Health checks Grafana connectivity.
func (h *GrafanaHandler) Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(h.GrafanaURL + "/api/health")
		if err != nil {
			respondJSON(w, http.StatusOK, map[string]any{
				"status": "unreachable",
				"error":  err.Error(),
				"url":    h.GrafanaURL,
			})
			return
		}
		defer resp.Body.Close()

		var health map[string]any
		json.NewDecoder(resp.Body).Decode(&health)

		respondJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"url":    h.GrafanaURL,
			"health": health,
		})
	}
}

// ListDashboards returns all dashboards from Grafana.
func (h *GrafanaHandler) ListDashboards() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dashboards, err := h.grafanaGet("/api/search?type=dash-db")
		if err != nil {
			respondError(w, http.StatusBadGateway, "failed to list dashboards: "+err.Error())
			return
		}
		respondJSON(w, http.StatusOK, dashboards)
	}
}

// CreateDashboard auto-generates a dashboard for specified measurements.
func (h *GrafanaHandler) CreateDashboard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Title        string   `json:"title"`
			Measurements []string `json:"measurements"`
			Fields       []string `json:"fields,omitempty"`
			Tags         []string `json:"tags,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		if len(req.Measurements) == 0 {
			respondError(w, http.StatusBadRequest, "at least one measurement is required")
			return
		}

		if req.Title == "" {
			req.Title = "IIoT: " + strings.Join(req.Measurements, ", ")
		}

		dashboard := grafana.GenerateDashboard(req.Title, req.Measurements, req.Fields, req.Tags)
		dashboardJSON, err := json.Marshal(dashboard)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to marshal dashboard: "+err.Error())
			return
		}

		// Create dashboard via Grafana API
		result, err := h.grafanaPost("/api/dashboards/db", dashboard)
		if err != nil {
			respondError(w, http.StatusBadGateway, "failed to create dashboard: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, map[string]any{
			"message":    "dashboard created",
			"dashboard":  result,
			"grafana_url": fmt.Sprintf("%s/d/%s", h.GrafanaURL, getDashboardUID(result)),
			"json":       json.RawMessage(dashboardJSON),
		})
	}
}

// EnsureDatasource verifies or creates the InfluxDB datasource in Grafana.
func (h *GrafanaHandler) EnsureDatasource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if InfluxDB datasource already exists
		datasources, err := h.grafanaGet("/api/datasources")
		if err != nil {
			respondError(w, http.StatusBadGateway, "failed to list datasources: "+err.Error())
			return
		}

		dsList, ok := datasources.([]any)
		if ok {
			for _, ds := range dsList {
				dsMap, ok := ds.(map[string]any)
				if ok && dsMap["type"] == "influxdb" {
					respondJSON(w, http.StatusOK, map[string]any{
						"status":  "exists",
						"message": "InfluxDB datasource already configured",
						"id":      dsMap["id"],
					})
					return
				}
			}
		}

		// Create InfluxDB datasource
		ds := map[string]any{
			"name":      "InfluxDB",
			"type":      "influxdb",
			"url":       "http://influxdb:8086",
			"access":    "proxy",
			"database":  "iiot",
			"isDefault": true,
			"jsonData": map[string]any{
				"httpMode": "GET",
			},
		}

		result, err := h.grafanaPost("/api/datasources", ds)
		if err != nil {
			respondError(w, http.StatusBadGateway, "failed to create datasource: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, map[string]any{
			"status":  "created",
			"message": "InfluxDB datasource created",
			"result":  result,
		})
	}
}

// RemoveDashboard deletes a dashboard by UID.
func (h *GrafanaHandler) RemoveDashboard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := r.URL.Query().Get("uid")
		if uid == "" {
			respondError(w, http.StatusBadRequest, "?uid= parameter required")
			return
		}

		err := h.grafanaDelete("/api/dashboards/uid/" + uid)
		if err != nil {
			respondError(w, http.StatusBadGateway, "failed to delete dashboard: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "dashboard deleted",
			"uid":     uid,
		})
	}
}

// ── Grafana API Helpers ──────────────────────────────────────────────────────

func (h *GrafanaHandler) grafanaGet(path string) (any, error) {
	req, err := http.NewRequest("GET", h.GrafanaURL+path, nil)
	if err != nil {
		return nil, err
	}
	return h.doGrafanaRequest(req)
}

func (h *GrafanaHandler) grafanaPost(path string, body any) (any, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", h.GrafanaURL+path, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return h.doGrafanaRequest(req)
}

func (h *GrafanaHandler) grafanaDelete(path string) error {
	req, err := http.NewRequest("DELETE", h.GrafanaURL+path, nil)
	if err != nil {
		return err
	}
	_, err = h.doGrafanaRequest(req)
	return err
}

func (h *GrafanaHandler) doGrafanaRequest(req *http.Request) (any, error) {
	if h.ServiceToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.ServiceToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("grafana returned %d: %s", resp.StatusCode, string(body))
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}
	return result, nil
}

func getDashboardUID(result any) string {
	if m, ok := result.(map[string]any); ok {
		if uid, ok := m["uid"].(string); ok {
			return uid
		}
	}
	return ""
}
