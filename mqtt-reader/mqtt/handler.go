package mqtt

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tidwall/gjson"

	"github.com/Sandun-S/industrial-iot-observability-stack/mqtt-reader/config"
	"github.com/Sandun-S/industrial-iot-observability-stack/mqtt-reader/influx"
)

// TopicRule binds a sensor config to a specific MQTT topic and field mapping.
type TopicRule struct {
	Sensor     config.SensorConfig
	Topic      string
	FieldKey   string // for JSON fields
	JSONPath   string // for JSON fields
	ColumnIdx  int    // for CSV columns
	ColumnType string // for CSV columns
	IsCSV      bool
}

// BuildRules creates topic rules from a reader config's sensors.
func BuildRules(cfg *config.ReaderConfig) []TopicRule {
	var rules []TopicRule

	for _, s := range cfg.Sensors {
		topic := s.Topic

		if s.CSV != nil {
			// CSV sensor: one rule per column
			for _, col := range s.CSV.Columns {
				rules = append(rules, TopicRule{
					Sensor:     s,
					Topic:      topic,
					FieldKey:   col.Key,
					ColumnIdx:  col.Index,
					ColumnType: col.Type,
					IsCSV:      true,
				})
			}
		} else {
			// JSON sensor: one rule per field mapping
			for _, f := range s.Fields {
				rules = append(rules, TopicRule{
					Sensor:   s,
					Topic:    topic,
					FieldKey: f.Key,
					JSONPath: f.JSONPath,
					IsCSV:    false,
				})
			}
		}
	}

	return rules
}

// UniqueTopics returns deduplicated topic strings from a set of rules.
func UniqueTopics(rules []TopicRule) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rules {
		if !seen[r.Topic] {
			seen[r.Topic] = true
			out = append(out, r.Topic)
		}
	}
	return out
}

// ExtractTags processes {{topic.N}} placeholders in tag values,
// replacing them with segments from the actual MQTT topic.
// e.g. "sensors/+/temperature" matching "sensors/rack1/temperature"
// with tag "sensor_id: {{topic.1}}" → "sensor_id: rack1"
func ExtractTags(tagMap map[string]string, topic string) map[string]string {
	if len(tagMap) == 0 {
		return nil
	}

	segments := strings.Split(topic, "/")
	result := make(map[string]string, len(tagMap))

	for k, v := range tagMap {
		val := v
		// Replace {{topic.N}} placeholders
		for {
			start := strings.Index(val, "{{topic.")
			if start == -1 {
				break
			}
			end := strings.Index(val[start:], "}}")
			if end == -1 {
				break
			}
			placeholder := val[start : start+end+2]
			// Extract index from "{{topic.N}}"
			idxStr := strings.TrimPrefix(placeholder, "{{topic.")
			idxStr = strings.TrimSuffix(idxStr, "}}")
			idx, err := strconv.Atoi(idxStr)
			replacement := placeholder // default: leave as-is
			if err == nil && idx >= 0 && idx < len(segments) {
				replacement = segments[idx]
			}
			val = strings.Replace(val, placeholder, replacement, 1)
		}
		result[k] = val
	}

	return result
}

// HandleMessage processes an incoming MQTT message against the configured rules,
// builds InfluxDB data points, and writes them via the influx client.
// Returns the number of data points written.
func HandleMessage(
	msg mqtt.Message,
	rules []TopicRule,
	influxClient *influx.Client,
	readerName string,
	debugLog func(string, ...any),
) int {
	topic := msg.Topic()
	payload := string(msg.Payload())

	// Check if this topic matches any rule
	var matchedRules []TopicRule
	for _, rule := range rules {
		if MatchTopic(rule.Topic, topic) {
			matchedRules = append(matchedRules, rule)
		}
	}

	if len(matchedRules) == 0 {
		return 0
	}

	// Group data points by (sensor name) to batch-write per sensor
	type groupKey struct {
		measurement string
		tags        string // serialized tag set for grouping
	}
	groups := map[groupKey][]influx.DataPoint{}

	for _, rule := range matchedRules {
		var value string
		var ok bool

		if rule.IsCSV {
			// Parse CSV payload
			delimiter := ","
			if rule.Sensor.CSV != nil && rule.Sensor.CSV.Delimiter != "" {
				delimiter = rule.Sensor.CSV.Delimiter
			}
			columns := strings.Split(payload, delimiter)
			if rule.ColumnIdx >= len(columns) {
				continue // column index out of range
			}
			value = strings.TrimSpace(columns[rule.ColumnIdx])
			ok = value != ""
		} else {
			// Extract value via GJSON path
			jsonPath := rule.JSONPath
			if jsonPath == "" || jsonPath == "." {
				// Take the raw payload as the value
				if gjson.Valid(payload) {
					value = payload
					ok = true
				}
			} else {
				result := gjson.Get(payload, jsonPath)
				if result.Exists() {
					value = result.String()
					ok = true
				}
			}
		}

		if !ok {
			continue
		}

		// Build tags: resolve {{topic.N}} placeholders
		tags := ExtractTags(rule.Sensor.Tags, topic)
		if tags == nil {
			tags = make(map[string]string)
		}
		// Always add these standard tags
		tags["device"] = rule.Sensor.Name
		tags["reading"] = rule.FieldKey
		tags["reader"] = readerName

		// Serialize tags for grouping
		var tagParts []string
		for k, v := range tags {
			tagParts = append(tagParts, k+"="+v)
		}
		tagSig := strings.Join(tagParts, ",")

		gk := groupKey{
			measurement: rule.Sensor.Measurement,
			tags:        tagSig,
		}

		groups[gk] = append(groups[gk], influx.DataPoint{
			Measurement: rule.Sensor.Measurement,
			Tags:        tags,
			Fields:      map[string]string{rule.FieldKey: value},
			Time:        time.Now().UnixMicro(),
		})
	}

	// Write each group
	count := 0
	for _, pts := range groups {
		if err := influxClient.Write(pts); err != nil {
			debugLog("InfluxDB write error for sensor %s: %v", pts[0].Tags["device"], err)
		} else {
			count += len(pts)
		}
	}

	return count
}

// MatchTopic checks if an MQTT topic matches a subscription filter.
// Supports MQTT wildcards: "+" (single level) and "#" (multi-level).
// Matches the MQTT 3.1.1 specification.
func MatchTopic(filter, topic string) bool {
	if filter == topic {
		return true
	}

	fp := strings.Split(filter, "/")
	tp := strings.Split(topic, "/")

	for i, f := range fp {
		if f == "#" {
			// Multi-level wildcard matches everything from here
			return true
		}
		if i >= len(tp) {
			return false // topic has fewer levels
		}
		if f != "+" && f != tp[i] {
			return false
		}
	}

	// Filter and topic must have same number of levels
	return len(fp) == len(tp)
}

// ConnectWithRetry connects to an MQTT broker with retries and visible logging.
func ConnectWithRetry(brokerURL, clientID, username, password string, qos byte, reconnectDelay int,
	onConnect mqtt.OnConnectHandler, onLost mqtt.ConnectionLostHandler,
	log func(string, ...any)) mqtt.Client {

	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetConnectRetry(false). // we handle retries manually for visible logging
		SetOnConnectHandler(onConnect).
		SetConnectionLostHandler(onLost)

	if username != "" {
		opts.SetUsername(username)
	}
	if password != "" {
		opts.SetPassword(password)
	}

	if reconnectDelay <= 0 {
		reconnectDelay = 10
	}

	client := mqtt.NewClient(opts)

	for {
		log("Connecting to MQTT broker %s ...", brokerURL)
		token := client.Connect()
		token.Wait()
		if token.Error() == nil {
			return client
		}
		log("MQTT connect failed: %v — retrying in %ds", token.Error(), reconnectDelay)
		time.Sleep(time.Duration(reconnectDelay) * time.Second)
	}
}

// ClientID generates a default MQTT client ID from the reader name.
func ClientID(name string) string {
	sanitized := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	return fmt.Sprintf("iiot-%s", sanitized)
}
