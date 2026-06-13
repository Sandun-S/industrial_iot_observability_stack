package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// InfluxHandler holds configuration for InfluxDB proxy.
type InfluxHandler struct {
	InfluxURL string
	InfluxDB  string
}

// NewInfluxHandler creates a new handler for InfluxDB proxy requests.
func NewInfluxHandler(influxURL, influxDB string) *InfluxHandler {
	return &InfluxHandler{
		InfluxURL: strings.TrimRight(influxURL, "/"),
		InfluxDB:  influxDB,
	}
}

// Health checks if InfluxDB is reachable.
func (h *InfluxHandler) Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(h.InfluxURL + "/ping")
		if err != nil {
			respondJSON(w, http.StatusOK, map[string]any{
				"status":  "unreachable",
				"error":   err.Error(),
				"url":     h.InfluxURL,
				"database": h.InfluxDB,
			})
			return
		}
		defer resp.Body.Close()

		respondJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"url":      h.InfluxURL,
			"database": h.InfluxDB,
			"ping":     resp.StatusCode == 204,
		})
	}
}

// Measurements lists all measurements in the InfluxDB database.
func (h *InfluxHandler) Measurements() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := h.queryInflux("SHOW MEASUREMENTS")
		if err != nil {
			respondError(w, http.StatusBadGateway, "failed to query InfluxDB: "+err.Error())
			return
		}
		respondJSON(w, http.StatusOK, result)
	}
}

// Query runs an arbitrary InfluxQL query and returns the result.
func (h *InfluxHandler) Query() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			respondError(w, http.StatusBadRequest, "?q= query parameter required")
			return
		}

		result, err := h.queryInflux(q)
		if err != nil {
			respondError(w, http.StatusBadGateway, "query failed: "+err.Error())
			return
		}
		respondJSON(w, http.StatusOK, result)
	}
}

// LatestData returns the latest readings from InfluxDB.
func (h *InfluxHandler) LatestData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		measurement := r.URL.Query().Get("measurement")
		if measurement == "" {
			// Get latest from all measurements
			measurements, err := h.getMeasurements()
			if err != nil {
				respondError(w, http.StatusBadGateway, err.Error())
				return
			}

			var allResults []map[string]any
			for _, m := range measurements {
				results, err := h.queryInflux(fmt.Sprintf(
					`SELECT * FROM "%s" ORDER BY time DESC LIMIT 200`, m))
				if err == nil && len(results) > 0 {
					// Deduplicate: keep latest per (device, reading)
					seen := map[string]bool{}
					for _, r := range results {
						device := fmt.Sprint(r["device"])
						reading := fmt.Sprint(r["reading"])
						key := m + "/" + device + "/" + reading
						if seen[key] {
							continue
						}
						seen[key] = true
						r["measurement"] = m
						allResults = append(allResults, r)
					}
				}
			}
			respondJSON(w, http.StatusOK, allResults)
			return
		}

		result, err := h.queryInflux(fmt.Sprintf(
			`SELECT * FROM "%s" ORDER BY time DESC LIMIT 200`, measurement))
		if err != nil {
			respondError(w, http.StatusBadGateway, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, result)
	}
}

// TagValues returns tag values for a measurement and tag key.
// Supports optional filter_key and filter_val for WHERE clause filtering.
func (h *InfluxHandler) TagValues() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		measurement := r.URL.Query().Get("measurement")
		tagKey := r.URL.Query().Get("tag")
		if measurement == "" || tagKey == "" {
			respondError(w, http.StatusBadRequest, "?measurement= and ?tag= required")
			return
		}

		query := fmt.Sprintf(`SHOW TAG VALUES FROM "%s" WITH KEY = "%s"`, measurement, tagKey)
		filterKey := r.URL.Query().Get("filter_key")
		filterVal := r.URL.Query().Get("filter_val")
		if filterKey != "" && filterVal != "" {
			query = fmt.Sprintf(`SHOW TAG VALUES FROM "%s" WITH KEY = "%s" WHERE "%s" = '%s'`,
				measurement, tagKey, filterKey, filterVal)
		}

		result, err := h.queryInflux(query)
		if err != nil {
			respondError(w, http.StatusBadGateway, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, result)
	}
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (h *InfluxHandler) queryInflux(q string) ([]map[string]any, error) {
	queryURL := fmt.Sprintf("%s/query", h.InfluxURL)
	params := url.Values{}
	params.Set("db", h.InfluxDB)
	params.Set("q", q)
	params.Set("epoch", "ms")

	resp, err := http.Get(queryURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("influxdb returned %d: %s", resp.StatusCode, string(body))
	}

	var influxResp struct {
		Results []struct {
			Series []struct {
				Name    string          `json:"name"`
				Columns []string        `json:"columns"`
				Values  [][]interface{} `json:"values"`
			} `json:"series"`
			Error string `json:"error"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &influxResp); err != nil {
		return nil, fmt.Errorf("parse influx response: %w", err)
	}

	var result []map[string]any
	for _, r := range influxResp.Results {
		if r.Error != "" {
			return nil, fmt.Errorf("influxdb error: %s", r.Error)
		}
		for _, s := range r.Series {
			for _, vals := range s.Values {
				row := make(map[string]any)
				for i, col := range s.Columns {
					if i < len(vals) {
						row[col] = vals[i]
					}
				}
				result = append(result, row)
			}
		}
	}

	return result, nil
}

func (h *InfluxHandler) getMeasurements() ([]string, error) {
	results, err := h.queryInflux("SHOW MEASUREMENTS")
	if err != nil {
		return nil, err
	}

	var measurements []string
	for _, row := range results {
		if name, ok := row["name"]; ok {
			measurements = append(measurements, fmt.Sprint(name))
		}
	}
	return measurements, nil
}
