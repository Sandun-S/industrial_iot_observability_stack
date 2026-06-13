package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReaderListItem is the API response for a single reader in the list.
type ReaderListItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Broker      string `json:"broker"`
	Sensors     int    `json:"sensors"`
	ConfigFile  string `json:"config_file"`
	Status      string `json:"status"` // "running", "unknown"
}

// ReadersHandler returns a list of configured MQTT readers.
func ReadersHandler(configDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		entries, err := os.ReadDir(configDir)
		if err != nil {
			// Return empty list if config dir doesn't exist yet
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
				Status:     "unknown",
			}

			// Try to parse YAML to extract metadata
			data, err := os.ReadFile(configDir + "/" + name)
			if err == nil {
				var cfg struct {
					Reader struct {
						Name        string `yaml:"name"`
						Description string `yaml:"description"`
					} `yaml:"reader"`
					MQTT struct {
						Broker string `yaml:"broker"`
					} `yaml:"mqtt"`
					Sensors []any `yaml:"sensors"`
				}
				if yaml.Unmarshal(data, &cfg) == nil {
					item.Name = cfg.Reader.Name
					item.Description = cfg.Reader.Description
					item.Broker = cfg.MQTT.Broker
					item.Sensors = len(cfg.Sensors)
				}
			}

			if item.Name == "" {
				item.Name = strings.TrimSuffix(name, ".yaml")
				item.Name = strings.TrimSuffix(item.Name, ".yml")
			}

			readers = append(readers, item)
		}

		respondJSON(w, http.StatusOK, readers)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{
		Error:   http.StatusText(status),
		Message: msg,
	})
}
