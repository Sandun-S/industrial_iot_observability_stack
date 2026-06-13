package grafana

import "fmt"

// GenerateDashboard creates a Grafana dashboard JSON model for the given measurements.
//
// The generated dashboard includes:
//   - One row per measurement
//   - Time-series panel per field
//   - Stat panel for latest value
//   - Template variables for filtering by device and reading
//   - Auto-refresh every 10 seconds
func GenerateDashboard(title string, measurements, fields, tags []string) map[string]any {
	panels := []any{}
	panelID := 1
	yPos := 0

	for _, m := range measurements {
		// Row title
		panels = append(panels, map[string]any{
			"id":      panelID,
			"type":    "row",
			"title":   m,
			"gridPos": map[string]any{"h": 1, "w": 24, "x": 0, "y": yPos},
		})
		panelID++
		yPos++

		// If fields are specified, create one panel per field
		if len(fields) > 0 {
			for _, f := range fields {
				panels = append(panels,
					timeSeriesPanel(panelID, m, f, 0, yPos),
					statPanel(panelID+1, m, f, 12, yPos),
				)
				panelID += 2
				yPos += 8
			}
		} else {
			// Generic query: SELECT * FROM measurement
			panels = append(panels,
				timeSeriesPanel(panelID, m, "*", 0, yPos),
				statPanel(panelID+1, m, "last", 12, yPos),
			)
			panelID += 2
			yPos += 8
		}
	}

	// Template variables
	templating := map[string]any{
		"list": []any{
			// Variable: measurement selector
			map[string]any{
				"name":       "measurement",
				"type":       "query",
				"datasource": "InfluxDB",
				"query":      "SHOW MEASUREMENTS",
				"refresh":    1, // on dashboard load
				"multi":      false,
				"includeAll": false,
				"label":      "Measurement",
			},
			// Variable: device selector
			map[string]any{
				"name":       "device",
				"type":       "query",
				"datasource": "InfluxDB",
				"query":      `SHOW TAG VALUES FROM "$measurement" WITH KEY = "device"`,
				"refresh":    1,
				"multi":      true,
				"includeAll": true,
				"label":      "Device",
			},
			// Variable: reading selector
			map[string]any{
				"name":       "reading",
				"type":       "query",
				"datasource": "InfluxDB",
				"query":      `SHOW TAG VALUES FROM "$measurement" WITH KEY = "reading"`,
				"refresh":    1,
				"multi":      true,
				"includeAll": true,
				"label":      "Reading",
			},
		},
	}

	return map[string]any{
		"dashboard": map[string]any{
			"title":      title,
			"tags":       []string{"iiot", "auto-generated"},
			"timezone":   "browser",
			"refresh":    "10s",
			"schemaVersion": 36,
			"panels":     panels,
			"templating": templating,
		},
		"overwrite": false,
	}
}

func timeSeriesPanel(id int, measurement, field string, x, y int) map[string]any {
	title := fmt.Sprintf("%s: %s", measurement, field)
	var target string
	if field == "*" {
		target = fmt.Sprintf(`SELECT * FROM "%s" WHERE $timeFilter`, measurement)
	} else {
		target = fmt.Sprintf(`SELECT "%s" FROM "%s" WHERE $timeFilter`, field, measurement)
	}

	return map[string]any{
		"id":      id,
		"type":    "timeseries",
		"title":   title,
		"gridPos": map[string]any{"h": 8, "w": 12, "x": x, "y": y},
		"targets": []any{
			map[string]any{
				"refId":      "A",
				"datasource": map[string]any{"type": "influxdb", "uid": "InfluxDB"},
				"query":      target,
				"rawQuery":   true,
				"resultFormat": "time_series",
			},
		},
		"fieldConfig": map[string]any{
			"defaults": map[string]any{
				"custom": map[string]any{
					"drawStyle":         "line",
					"lineInterpolation": "smooth",
					"fillOpacity":       15,
					"showPoints":        "auto",
				},
			},
		},
	}
}

func statPanel(id int, measurement, field string, x, y int) map[string]any {
	title := fmt.Sprintf("%s: Latest %s", measurement, field)
	var target string
	if field == "last" {
		target = fmt.Sprintf(`SELECT * FROM "%s" ORDER BY time DESC LIMIT 1`, measurement)
	} else {
		target = fmt.Sprintf(`SELECT "%s" FROM "%s" ORDER BY time DESC LIMIT 1`, field, measurement)
	}

	return map[string]any{
		"id":      id,
		"type":    "stat",
		"title":   title,
		"gridPos": map[string]any{"h": 4, "w": 3, "x": x, "y": y + 8},
		"targets": []any{
			map[string]any{
				"refId":      "A",
				"datasource": map[string]any{"type": "influxdb", "uid": "InfluxDB"},
				"query":      target,
				"rawQuery":   true,
				"resultFormat": "time_series",
			},
		},
		"options": map[string]any{
			"reduceOptions": map[string]any{
				"values": false,
				"calcs":  []string{"lastNotNull"},
			},
		},
	}
}
