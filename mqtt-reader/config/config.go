package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ── Top-Level Config ─────────────────────────────────────────────────────────

// ReaderConfig is the top-level YAML config for one MQTT reader instance.
type ReaderConfig struct {
	Reader   ReaderMeta     `yaml:"reader"`
	MQTT     MQTTConfig     `yaml:"mqtt"`
	InfluxDB InfluxDBConfig `yaml:"influxdb"`
	Sensors  []SensorConfig `yaml:"sensors"`
}

// ReaderMeta holds metadata about this reader instance.
type ReaderMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ── MQTT ─────────────────────────────────────────────────────────────────────

// MQTTConfig holds connection details for the MQTT broker.
type MQTTConfig struct {
	Broker         string `yaml:"broker"`          // e.g. "tcp://192.168.1.100:1883"
	ClientID       string `yaml:"client_id"`       // MQTT client identifier
	QoS            byte   `yaml:"qos"`             // 0, 1, or 2
	Username       string `yaml:"username"`         // optional
	Password       string `yaml:"password"`         // optional
	ReconnectDelay int    `yaml:"reconnect_delay"` // seconds, 0 = default 10s
}

// ── InfluxDB ─────────────────────────────────────────────────────────────────

// InfluxDBConfig holds connection details for the target InfluxDB v1.
type InfluxDBConfig struct {
	URL      string `yaml:"url"`      // e.g. "http://influxdb:8086"
	Database string `yaml:"database"` // e.g. "iiot"
	Username string `yaml:"username"` // optional
	Password string `yaml:"password"` // optional
}

// ── Sensors ──────────────────────────────────────────────────────────────────

// SensorConfig defines one sensor (or device) whose data arrives via MQTT.
type SensorConfig struct {
	Name        string            `yaml:"name"`
	Topic       string            `yaml:"topic"`       // MQTT topic (supports + and # wildcards)
	Measurement string            `yaml:"measurement"`  // InfluxDB measurement name
	Fields      []FieldMapping    `yaml:"fields"`       // JSON path → field key mappings
	CSV         *CSVConfig        `yaml:"csv,omitempty"` // CSV column mapping (alternative to fields)
	Tags        map[string]string `yaml:"tags"`         // static InfluxDB tags
}

// FieldMapping maps a JSON path to an InfluxDB field key.
type FieldMapping struct {
	Key      string `yaml:"key"`       // InfluxDB field key (e.g. "temperature_c")
	JSONPath string `yaml:"json_path"` // GJSON expression (e.g. "temperature", "supply_air.temp")
	Type     string `yaml:"type"`      // "float", "int", "string", "bool"
	Unit     string `yaml:"unit"`      // optional unit (e.g. "°C", "%", "W")
}

// CSVConfig defines how to parse CSV payloads from MQTT messages.
type CSVConfig struct {
	Delimiter  string      `yaml:"delimiter"`   // default ","
	SkipHeader bool        `yaml:"skip_header"` // skip first row if true
	Columns    []ColumnMap `yaml:"columns"`     // column index → field key mapping
}

// ColumnMap maps a CSV column index to an InfluxDB field key.
type ColumnMap struct {
	Key   string `yaml:"key"`   // InfluxDB field key
	Index int    `yaml:"index"` // zero-based column index
	Type  string `yaml:"type"`  // "float", "int", "string"
	Unit  string `yaml:"unit"`  // optional unit
}

// ── Loading ──────────────────────────────────────────────────────────────────

// Load reads and parses a YAML config file.
func Load(path string) (*ReaderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg ReaderConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	// ── Apply defaults ───────────────────────────────────────────────────
	if cfg.MQTT.QoS == 0 {
		cfg.MQTT.QoS = 1
	}
	if cfg.MQTT.ReconnectDelay == 0 {
		cfg.MQTT.ReconnectDelay = 10
	}
	if cfg.Reader.Name == "" {
		cfg.Reader.Name = cfg.MQTT.ClientID
	}

	for i := range cfg.Sensors {
		if cfg.Sensors[i].CSV != nil && cfg.Sensors[i].CSV.Delimiter == "" {
			cfg.Sensors[i].CSV.Delimiter = ","
		}
	}

	// ── Validate ─────────────────────────────────────────────────────────
	if cfg.MQTT.Broker == "" {
		return nil, fmt.Errorf("mqtt.broker is required")
	}
	if cfg.InfluxDB.URL == "" {
		return nil, fmt.Errorf("influxdb.url is required")
	}
	if cfg.InfluxDB.Database == "" {
		return nil, fmt.Errorf("influxdb.database is required")
	}
	if len(cfg.Sensors) == 0 {
		return nil, fmt.Errorf("at least one sensor is required")
	}

	for i, s := range cfg.Sensors {
		if s.Name == "" {
			return nil, fmt.Errorf("sensors[%d].name is required", i)
		}
		if s.Topic == "" {
			return nil, fmt.Errorf("sensors[%d].topic is required (sensor: %s)", i, s.Name)
		}
		if s.Measurement == "" {
			return nil, fmt.Errorf("sensors[%d].measurement is required (sensor: %s)", i, s.Name)
		}
		if len(s.Fields) == 0 && s.CSV == nil {
			return nil, fmt.Errorf("sensors[%d] requires either fields or csv (sensor: %s)", i, s.Name)
		}
		if len(s.Fields) > 0 && s.CSV != nil {
			return nil, fmt.Errorf("sensors[%d] cannot have both fields and csv (sensor: %s)", i, s.Name)
		}
	}

	return &cfg, nil
}

// LoadDir loads all .yaml and .yml files from a directory.
func LoadDir(dir string) ([]*ReaderConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read config dir: %w", err)
	}

	var configs []*ReaderConfig
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 5 {
			continue
		}
		ext := name[len(name)-5:]
		if ext != ".yaml" && name[len(name)-4:] != ".yml" {
			continue
		}

		cfg, err := Load(dir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", name, err)
		}
		configs = append(configs, cfg)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no YAML config files found in %s", dir)
	}

	return configs, nil
}
