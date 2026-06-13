package config

import "os"

// Config holds all configuration for the Web UI backend.
type Config struct {
	Port         string
	InfluxURL    string
	InfluxDB     string
	GrafanaURL   string
	GrafanaToken string // optional service account token for dashboard creation
	ConfigDir    string // directory for reader YAML config files
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:         getenv("PORT", "8080"),
		InfluxURL:    getenv("INFLUX_URL", "http://influxdb:8086"),
		InfluxDB:     getenv("INFLUX_DB", "iiot"),
		GrafanaURL:   getenv("GRAFANA_URL", "http://grafana:3000"),
		GrafanaToken: getenv("GRAFANA_SERVICE_ACCOUNT_TOKEN", ""),
		ConfigDir:    getenv("CONFIG_DIR", "/configs"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
