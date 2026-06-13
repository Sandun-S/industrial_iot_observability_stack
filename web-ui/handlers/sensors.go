package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

// SensorInfo is the API representation of a sensor in a reader config.
type SensorInfo struct {
	Name        string            `json:"name"`
	Topic       string            `json:"topic"`
	Measurement string            `json:"measurement"`
	Fields      int               `json:"fields"`
	CSV         bool              `json:"csv"`
	Tags        map[string]string `json:"tags"`
}

// SensorsHandler returns all sensors for a specific reader.
func SensorsHandler(configDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		// Extract reader name from path: /api/readers/:name/sensors
		readerName := extractReaderName(r.URL.Path)
		if readerName == "" {
			respondError(w, http.StatusBadRequest, "reader name required")
			return
		}

		cfg, err := loadReaderConfig(configDir, readerName)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}

		var sensors []SensorInfo
		for _, s := range cfg.Sensors {
			si := SensorInfo{
				Name:        s.Name,
				Topic:       s.Topic,
				Measurement: s.Measurement,
				Fields:      len(s.Fields),
				CSV:         s.CSV != nil,
				Tags:        s.Tags,
			}
			sensors = append(sensors, si)
		}

		respondJSON(w, http.StatusOK, sensors)
	}
}

// AddSensorHandler adds a new sensor to a reader config.
func AddSensorHandler(configDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		readerName := extractReaderName(r.URL.Path)
		if readerName == "" {
			respondError(w, http.StatusBadRequest, "reader name required")
			return
		}

		var sensor SensorAddRequest
		if err := json.NewDecoder(r.Body).Decode(&sensor); err != nil {
			respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		cfg, err := loadReaderConfig(configDir, readerName)
		if err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}

		// Build new sensor from request
		newSensor := struct {
			Name        string            `yaml:"name"`
			Topic       string            `yaml:"topic"`
			Measurement string            `yaml:"measurement"`
			Fields      []any             `yaml:"fields,omitempty"`
			CSV         any               `yaml:"csv,omitempty"`
			Tags        map[string]string `yaml:"tags,omitempty"`
		}{
			Name:        sensor.Name,
			Topic:       sensor.Topic,
			Measurement: sensor.Measurement,
			Tags:        sensor.Tags,
		}

		for _, f := range sensor.Fields {
			newSensor.Fields = append(newSensor.Fields, map[string]any{
				"key":       f.Key,
				"json_path": f.JSONPath,
				"type":      f.Type,
				"unit":      f.Unit,
			})
		}

		if sensor.CSV != nil {
			var columns []map[string]any
			for _, c := range sensor.CSV.Columns {
				columns = append(columns, map[string]any{
					"key":   c.Key,
					"index": c.Index,
					"type":  c.Type,
					"unit":  c.Unit,
				})
			}
			newSensor.CSV = map[string]any{
				"delimiter":   sensor.CSV.Delimiter,
				"skip_header": sensor.CSV.SkipHeader,
				"columns":     columns,
			}
		}

		cfg.Sensors = append(cfg.Sensors, newSensor)

		if err := saveReaderConfig(configDir, readerName, cfg); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, map[string]string{
			"message": "sensor added",
			"reader":  readerName,
		})
	}
}

// SensorAddRequest is the request body for adding a sensor.
type SensorAddRequest struct {
	Name        string            `json:"name"`
	Topic       string            `json:"topic"`
	Measurement string            `json:"measurement"`
	Fields      []FieldRequest    `json:"fields"`
	CSV         *CSVRequest       `json:"csv,omitempty"`
	Tags        map[string]string `json:"tags"`
}

type FieldRequest struct {
	Key      string `json:"key"`
	JSONPath string `json:"json_path"`
	Type     string `json:"type"`
	Unit     string `json:"unit"`
}

type CSVRequest struct {
	Delimiter  string        `json:"delimiter"`
	SkipHeader bool          `json:"skip_header"`
	Columns    []ColumnReq   `json:"columns"`
}

type ColumnReq struct {
	Key   string `json:"key"`
	Index int    `json:"index"`
	Type  string `json:"type"`
	Unit  string `json:"unit"`
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func extractReaderName(path string) string {
	// Path format: /api/readers/:name/sensors or /api/readers/:name/sensors/...
	parts := splitPath(path)
	for i, p := range parts {
		if p == "readers" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func splitPath(path string) []string {
	parts := []string{}
	for _, p := range stringsSplit(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func stringsSplit(s, sep string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if string(c) == sep {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func loadReaderConfig(configDir, readerName string) (*readerConfigRaw, error) {
	// Try with .yaml and .yml extensions
	for _, ext := range []string{".yaml", ".yml"} {
		path := configDir + "/" + readerName + ext
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var cfg readerConfigRaw
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			continue
		}
		return &cfg, nil
	}

	// Try exact filename if it has an extension
	if stringsContains(readerName, ".") {
		path := configDir + "/" + readerName
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg readerConfigRaw
			if yaml.Unmarshal(data, &cfg) == nil {
				return &cfg, nil
			}
		}
	}

	return nil, os.ErrNotExist
}

func saveReaderConfig(configDir, readerName string, cfg *readerConfigRaw) error {
	path := configDir + "/" + readerName + ".yaml"
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

type readerConfigRaw struct {
	Reader   map[string]any `yaml:"reader"`
	MQTT     map[string]any `yaml:"mqtt"`
	InfluxDB map[string]any `yaml:"influxdb"`
	Sensors  []any          `yaml:"sensors"`
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
