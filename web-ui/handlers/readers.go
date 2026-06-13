package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── API Types ───────────────────────────────────────────────────────────────

type ReaderListItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Broker      string `json:"broker"`
	Sensors     int    `json:"sensors"`
	ConfigFile  string `json:"config_file"`
	Status      string `json:"status"`
}

type CreateReaderRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Broker      string `json:"broker"`
	ClientID    string `json:"client_id"`
	QoS         byte   `json:"qos"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	InfluxURL   string `json:"influx_url"`
	InfluxDB    string `json:"influx_db"`
}

// ── Handler ─────────────────────────────────────────────────────────────────

type ReadersHandler struct {
	ConfigDir string
}

func NewReadersHandler(configDir string) *ReadersHandler {
	return &ReadersHandler{ConfigDir: configDir}
}

// ServeHTTP routes reader requests.
func (h *ReadersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/readers")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" && r.Method == http.MethodGet:
		h.list(w, r)
	case path == "" && r.Method == http.MethodPost:
		h.create(w, r)
	case strings.HasSuffix(path, "/sensors") && r.Method == http.MethodGet:
		h.listSensors(w, r, strings.TrimSuffix(path, "/sensors"))
	case strings.HasSuffix(path, "/sensors") && r.Method == http.MethodPost:
		h.addSensor(w, r, strings.TrimSuffix(path, "/sensors"))
	case strings.HasSuffix(path, "/restart") && r.Method == http.MethodPost:
		h.restart(w, r, strings.TrimSuffix(path, "/restart"))
	case !strings.Contains(path, "/") && r.Method == http.MethodGet:
		h.get(w, r, path)
	case !strings.Contains(path, "/") && r.Method == http.MethodDelete:
		h.delete(w, r, path)
	default:
		respondError(w, http.StatusNotFound, "not found: "+r.URL.Path)
	}
}

// ── Operations ──────────────────────────────────────────────────────────────

func (h *ReadersHandler) list(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.ConfigDir)
	if err != nil {
		respondJSON(w, http.StatusOK, []ReaderListItem{})
		return
	}

	var readers []ReaderListItem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		item := ReaderListItem{
			ConfigFile: name,
			Status:     "configured",
		}

		data, err := os.ReadFile(h.ConfigDir + "/" + name)
		if err == nil {
			var cfg readerYAML
			if yaml.Unmarshal(data, &cfg) == nil {
				if n, ok := cfg.Reader["name"].(string); ok { item.Name = n }
				if d, ok := cfg.Reader["description"].(string); ok { item.Description = d }
				if b, ok := cfg.MQTT["broker"].(string); ok { item.Broker = b }
				item.Sensors = len(cfg.Sensors)
			}
		}

		if item.Name == "" {
			item.Name = strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
		}

		readers = append(readers, item)
	}

	respondJSON(w, http.StatusOK, readers)
}

func (h *ReadersHandler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateReaderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Broker == "" {
		respondError(w, http.StatusBadRequest, "broker is required")
		return
	}

	// Generate safe filename
	filename := sanitizeFilename(req.Name) + ".yaml"
	filePath := h.ConfigDir + "/" + filename

	// Check if already exists
	if _, err := os.Stat(filePath); err == nil {
		respondError(w, http.StatusConflict, "reader '"+req.Name+"' already exists")
		return
	}

	// Build YAML config from request
	cfg := readerYAML{
		Reader: map[string]any{
			"name":        req.Name,
			"description": req.Description,
		},
		MQTT: map[string]any{
			"broker":          req.Broker,
			"client_id":       firstNonEmpty(req.ClientID, sanitizeFilename(req.Name)),
			"qos":             firstNonEmptyByte(req.QoS, 1),
			"username":        req.Username,
			"password":        req.Password,
			"reconnect_delay": 10,
		},
		InfluxDB: map[string]any{
			"url":      firstNonEmpty(req.InfluxURL, "http://influxdb:8086"),
			"database": firstNonEmpty(req.InfluxDB, "iiot"),
		},
		Sensors: []any{},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to marshal config: "+err.Error())
		return
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to write config: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"message":     "reader created",
		"name":        req.Name,
		"config_file": filename,
		"hint":        "Redeploy the stack or restart the reader service to apply changes.",
	})
}

func (h *ReadersHandler) get(w http.ResponseWriter, r *http.Request, name string) {
	cfg, filename, err := h.findConfig(name)
	if err != nil {
		respondError(w, http.StatusNotFound, "reader not found: "+name)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"name":        cfg.Reader["name"],
		"description": cfg.Reader["description"],
		"broker":      cfg.MQTT["broker"],
		"client_id":   cfg.MQTT["client_id"],
		"qos":         cfg.MQTT["qos"],
		"influx_url":  cfg.InfluxDB["url"],
		"influx_db":   cfg.InfluxDB["database"],
		"sensors":     cfg.Sensors,
		"config_file": filename,
	})
}

func (h *ReadersHandler) delete(w http.ResponseWriter, r *http.Request, name string) {
	_, filename, err := h.findConfig(name)
	if err != nil {
		respondError(w, http.StatusNotFound, "reader not found: "+name)
		return
	}

	if err := os.Remove(h.ConfigDir + "/" + filename); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "reader deleted — remove the corresponding Docker service if deployed",
		"name":    name,
	})
}

func (h *ReadersHandler) restart(w http.ResponseWriter, r *http.Request, name string) {
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "To restart, redeploy the stack: ./scripts/deploy.sh or docker service update --force iiot_mqtt-reader-" + sanitizeFilename(name),
		"name":    name,
	})
}

// ── Sensors ─────────────────────────────────────────────────────────────────

func (h *ReadersHandler) listSensors(w http.ResponseWriter, r *http.Request, name string) {
	cfg, _, err := h.findConfig(name)
	if err != nil {
		respondError(w, http.StatusNotFound, "reader not found: "+name)
		return
	}

	sensors := cfg.Sensors
	if sensors == nil {
		sensors = []any{}
	}
	respondJSON(w, http.StatusOK, sensors)
}

func (h *ReadersHandler) addSensor(w http.ResponseWriter, r *http.Request, name string) {
	cfg, filename, err := h.findConfig(name)
	if err != nil {
		respondError(w, http.StatusNotFound, "reader not found: "+name)
		return
	}

	var sensor map[string]any
	if err := json.NewDecoder(r.Body).Decode(&sensor); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	if s, _ := sensor["name"].(string); s == "" {
		respondError(w, http.StatusBadRequest, "sensor.name is required")
		return
	}
	if t, _ := sensor["topic"].(string); t == "" {
		respondError(w, http.StatusBadRequest, "sensor.topic is required")
		return
	}
	if m, _ := sensor["measurement"].(string); m == "" {
		respondError(w, http.StatusBadRequest, "sensor.measurement is required")
		return
	}

	cfg.Sensors = append(cfg.Sensors, sensor)

	if err := h.saveConfig(filename, cfg); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "sensor added — restart reader to apply",
		"reader":  name,
	})
}

// ── Helpers ─────────────────────────────────────────────────────────────────

type readerYAML struct {
	Reader   map[string]any `yaml:"reader"`
	MQTT     map[string]any `yaml:"mqtt"`
	InfluxDB map[string]any `yaml:"influxdb"`
	Sensors  []any          `yaml:"sensors"`
}

func (h *ReadersHandler) findConfig(name string) (*readerYAML, string, error) {
	nameLower := sanitizeFilename(name)

	for _, ext := range []string{".yaml", ".yml"} {
		path := h.ConfigDir + "/" + nameLower + ext
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg readerYAML
		if yaml.Unmarshal(data, &cfg) == nil {
			return &cfg, nameLower + ext, nil
		}
	}

	// Try scanning all YAML files for matching reader name
	entries, _ := os.ReadDir(h.ConfigDir)
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		data, err := os.ReadFile(h.ConfigDir + "/" + e.Name())
		if err != nil {
			continue
		}
		var cfg readerYAML
		if yaml.Unmarshal(data, &cfg) != nil {
			continue
		}
		if rn, _ := cfg.Reader["name"].(string); rn == name || sanitizeFilename(rn) == nameLower {
			return &cfg, e.Name(), nil
		}
	}

	return nil, "", fmt.Errorf("not found")
}

func (h *ReadersHandler) saveConfig(filename string, cfg *readerYAML) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(h.ConfigDir+"/"+filename, data, 0644)
}

// ── Utility ─────────────────────────────────────────────────────────────────

func sanitizeFilename(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	// Remove characters that are problematic in filenames
	result := ""
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result += string(c)
		}
	}
	if result == "" {
		return "reader"
	}
	return result
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstNonEmptyByte(a, b byte) byte {
	if a != 0 {
		return a
	}
	return b
}
