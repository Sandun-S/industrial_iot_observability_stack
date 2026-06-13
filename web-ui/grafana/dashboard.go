package grafana

import "fmt"

// DeviceGroup represents a device and its readings for dashboard generation.
type DeviceGroup struct {
	Device   string        `json:"device"`
	Readings []ReadingInfo `json:"readings"`
}

// ReadingInfo holds metadata about one reading field.
type ReadingInfo struct {
	Key  string `json:"key"`
	Unit string `json:"unit,omitempty"`
}

// GenerateDashboard creates an industrial-style Grafana dashboard.
//
// Layout:
//   - Title banner at top
//   - One section per device with colored header
//   - Stat panels in a grid row per device (up to 6 per row)
//   - Each stat panel queries latest value filtered by device + reading tags
//   - Color thresholds for temperature/pressure/percentage readings
func GenerateDashboard(title string, measurement string, devices []DeviceGroup) map[string]any {
	panels := []any{}
	panelID := 1
	yPos := 0

	// ── Title banner ────────────────────────────────────────────────────
	panels = append(panels, map[string]any{
		"id":      panelID,
		"type":    "text",
		"gridPos": map[string]any{"h": 3, "w": 24, "x": 0, "y": yPos},
		"options": map[string]any{
			"mode": "html",
			"content": fmt.Sprintf(
				`<h1 style="background-color:#1f4e8a;color:white;font-size:280%%;margin:0;padding:6px"><center>%s</center></h1>`,
				title,
			),
		},
	})
	panelID++
	yPos += 3

	// Header colors cycle for visual variety
	headerColors := []string{"#3d6db3", "#4a7c2b", "#8a3a2b", "#5a4a8a", "#2b6d8a", "#8a6d2b"}

	for di, dev := range devices {
		deviceName := dev.Device
		if deviceName == "" || deviceName == "—" {
			continue
		}

		// ── Device header ────────────────────────────────────────────────
		color := headerColors[di%len(headerColors)]
		panels = append(panels, map[string]any{
			"id":      panelID,
			"type":    "text",
			"gridPos": map[string]any{"h": 2, "w": 24, "x": 0, "y": yPos},
			"options": map[string]any{
				"mode": "html",
				"content": fmt.Sprintf(
					`<h2 style="background-color:%s;color:white;font-size:130%%;margin:0;padding:4px"><center>%s</center></h2>`,
					color, deviceName,
				),
			},
		})
		panelID++
		yPos += 2

		// ── Stat panels in grid ──────────────────────────────────────────
		colsPerRow := 6
		readingList := dev.Readings
		if len(readingList) == 0 {
			readingList = []ReadingInfo{{Key: "value"}}
		}

		for i, r := range readingList {
			col := (i % colsPerRow) * 4
			rowOffset := (i / colsPerRow) * 5

			panels = append(panels, statPanel(
				panelID,
				measurement,
				deviceName,
				r.Key,
				r.Unit,
				col,
				yPos+rowOffset,
			))
			panelID++
		}

		rowsUsed := ((len(readingList) - 1) / colsPerRow) + 1
		yPos += rowsUsed * 5

		// ── Time-series chart per device ─────────────────────────────────
		panels = append(panels, timeSeriesRow(
			panelID,
			measurement,
			deviceName,
			readingList,
			yPos,
		))
		panelID++
		yPos += 10
	}

	// Generate stable UID
	uid := "iiot-" + sanitizeForUID(title)

	return map[string]any{
		"dashboard": map[string]any{
			"uid":           uid,
			"title":         title,
			"tags":          []string{"iiot", "auto-generated"},
			"timezone":      "browser",
			"refresh":       "10s",
			"schemaVersion": 39,
			"panels":        panels,
			"templating": map[string]any{
				"list": []any{},
			},
		},
		"overwrite": true,
	}
}

// timeSeriesRow creates a full-width time-series panel showing all readings for a device.
func timeSeriesRow(id int, measurement, device string, readings []ReadingInfo, y int) map[string]any {
	var targets []any
	for _, r := range readings {
		targets = append(targets, map[string]any{
			"refId":        r.Key,
			"datasource":   map[string]any{"type": "influxdb", "uid": "InfluxDB"},
			"query":        fmt.Sprintf(`SELECT "%s" FROM "%s" WHERE "device"='%s' AND $timeFilter`, r.Key, measurement, device),
			"rawQuery":     true,
			"resultFormat": "time_series",
			"alias":        r.Key,
		})
	}

	return map[string]any{
		"id":      id,
		"type":    "timeseries",
		"title":   fmt.Sprintf("%s — Trend", device),
		"gridPos": map[string]any{"h": 10, "w": 24, "x": 0, "y": y},
		"targets": targets,
		"fieldConfig": map[string]any{
			"defaults": map[string]any{
				"custom": map[string]any{
					"drawStyle":         "line",
					"lineInterpolation": "smooth",
					"fillOpacity":       10,
					"showPoints":        "auto",
				},
			},
		},
	}
}

// statPanel creates a stat panel for a specific device+reading combination.
// Uses tag-based filtering for precise queries.
func statPanel(id int, measurement, device, reading, unit string, x, y int) map[string]any {
	title := reading
	if unit != "" {
		title = fmt.Sprintf("%s (%s)", reading, unit)
	}

	// Build query: SELECT last(value) filtered by device and reading tags
	query := fmt.Sprintf(
		`SELECT "%s" FROM "%s" WHERE "device"='%s' AND "reading"='%s' ORDER BY time DESC LIMIT 1`,
		reading, measurement, device, reading,
	)

	// Determine unit and thresholds based on reading name
	unitCfg := getUnitConfig(reading, unit)

	panel := map[string]any{
		"id":      id,
		"type":    "stat",
		"title":   title,
		"gridPos": map[string]any{"h": 5, "w": 4, "x": x, "y": y},
		"targets": []any{
			map[string]any{
				"refId":        "A",
				"datasource":   map[string]any{"type": "influxdb", "uid": "InfluxDB"},
				"query":        query,
				"rawQuery":     true,
				"resultFormat": "time_series",
			},
		},
		"options": map[string]any{
			"colorMode":            "background",
			"graphMode":            "none",
			"justifyMode":          "auto",
			"orientation":          "horizontal",
			"textMode":             "auto",
			"wideLayout":           true,
			"showPercentChange":    false,
			"percentChangeColorMode": "standard",
			"reduceOptions": map[string]any{
				"calcs":  []string{"lastNotNull"},
				"fields": "",
				"values": false,
			},
		},
		"fieldConfig": map[string]any{
			"defaults": map[string]any{
				"unit":      unitCfg.unit,
				"decimals":  unitCfg.decimals,
				"mappings":  unitCfg.mappings,
				"thresholds": map[string]any{
					"mode": "absolute",
					"steps": unitCfg.thresholds,
				},
			},
			"overrides": []any{},
		},
		"maxDataPoints": 100,
	}

	// Add value mappings for status-type readings
	if unitCfg.valueMap != nil {
		panel["fieldConfig"].(map[string]any)["defaults"].(map[string]any)["mappings"] = unitCfg.valueMap
	}

	return panel
}

// unitConfig holds Grafana unit/display configuration for a reading.
type unitConfig struct {
	unit       string
	decimals   int
	mappings   []any
	thresholds []any
	valueMap   []any
}

func getUnitConfig(reading, configuredUnit string) unitConfig {
	cfg := unitConfig{
		unit:     "none",
		decimals: 1,
		thresholds: []any{
			map[string]any{"color": "blue", "value": nil},
		},
	}

	// Use configured unit if provided
	if configuredUnit != "" {
		cfg.unit = mapUnit(configuredUnit)
	}

	// Apply color thresholds based on reading name patterns
	name := reading

	if containsAny(name, "temp", "temperature") {
		cfg.unit = "celsius"
		cfg.decimals = 1
		cfg.thresholds = []any{
			map[string]any{"color": "blue", "value": nil},
			map[string]any{"color": "green", "value": -50},
			map[string]any{"color": "orange", "value": 50},
			map[string]any{"color": "red", "value": 80},
		}
	} else if containsAny(name, "pressure", "bar") {
		cfg.unit = "pressurebar"
		cfg.decimals = 2
		cfg.thresholds = []any{
			map[string]any{"color": "blue", "value": nil},
			map[string]any{"color": "green", "value": 0},
			map[string]any{"color": "orange", "value": 12},
			map[string]any{"color": "red", "value": 16},
		}
	} else if containsAny(name, "speed", "rpm") {
		cfg.unit = "rotrpm"
		cfg.decimals = 0
	} else if containsAny(name, "pct", "percent", "capacity", "humidity") {
		cfg.unit = "percent"
		cfg.decimals = 0
	} else if containsAny(name, "power", "watt", "kw", "energy") {
		cfg.unit = "watt"
		cfg.decimals = 1
	} else if containsAny(name, "voltage", "volt") {
		cfg.unit = "volt"
		cfg.decimals = 1
	} else if containsAny(name, "current", "amp") {
		cfg.unit = "amp"
		cfg.decimals = 2
	} else if containsAny(name, "status", "state", "running") {
		cfg.unit = "none"
		cfg.decimals = 0
		cfg.valueMap = []any{
			map[string]any{
				"type": "special",
				"options": map[string]any{
					"match": "null+nan",
					"result": map[string]any{"text": "N/A", "color": "text"},
				},
			},
		}
		cfg.thresholds = []any{
			map[string]any{"color": "transparent", "value": nil},
		}
	} else if containsAny(name, "frequency", "hz") {
		cfg.unit = "hertz"
		cfg.decimals = 1
	}

	return cfg
}

// mapUnit converts a user-friendly unit to Grafana's unit format.
func mapUnit(u string) string {
	m := map[string]string{
		"°C": "celsius", "C": "celsius", "celsius": "celsius",
		"°F": "fahrenheit", "F": "fahrenheit",
		"%": "percent", "percent": "percent",
		"W": "watt", "kW": "watt", "watt": "watt",
		"V": "volt", "volt": "volt",
		"A": "amp", "amp": "amp",
		"bar": "pressurebar", "psi": "pressurepsi",
		"RPM": "rotrpm", "rpm": "rotrpm",
		"Hz": "hertz", "hertz": "hertz",
		"ppm": "ppm",
	}
	if v, ok := m[u]; ok {
		return v
	}
	return "none"
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// sanitizeForUID converts a title to a valid Grafana UID.
func sanitizeForUID(s string) string {
	result := ""
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result += string(c)
		} else if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else if c == ' ' || c == '_' || c == ':' || c == ',' {
			result += "-"
		}
	}
	for len(result) > 0 && result[0] == '-' {
		result = result[1:]
	}
	for len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	if result == "" {
		result = "dashboard"
	}
	return result
}
