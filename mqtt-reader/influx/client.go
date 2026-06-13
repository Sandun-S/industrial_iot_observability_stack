package influx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// Client writes data points to InfluxDB v1 using line protocol.
type Client struct {
	URL      string // e.g. "http://influxdb:8086"
	Database string
	Username string
	Password string
	log      *logrus.Logger
	mu       sync.Mutex
}

// DataPoint is a single time-series data point for InfluxDB.
type DataPoint struct {
	Measurement string            // InfluxDB measurement (table)
	Tags        map[string]string // tag key → value pairs
	Fields      map[string]string // field key → value pairs (values are string-formatted)
	Time        int64             // Unix microseconds
}

// NewClient creates a new InfluxDB client.
func NewClient(url, database, username, password string, log *logrus.Logger) *Client {
	return &Client{
		URL:      url,
		Database: database,
		Username: username,
		Password: password,
		log:      log,
	}
}

// Write sends a batch of data points to InfluxDB.
func (c *Client) Write(points []DataPoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var url string
	if c.Username != "" && c.Password != "" {
		url = fmt.Sprintf("%s/write?db=%s&u=%s&p=%s&precision=u",
			c.URL, c.Database, c.Username, c.Password)
	} else {
		url = fmt.Sprintf("%s/write?db=%s&precision=u",
			c.URL, c.Database)
	}

	var data strings.Builder
	for _, pt := range points {
		measurement := escapeLP(pt.Measurement, false)

		// Build tag set
		var tagParts []string
		for k, v := range pt.Tags {
			tagParts = append(tagParts, escapeLP(k, true)+"="+escapeLP(v, true))
		}
		tagSet := strings.Join(tagParts, ",")

		// Build field set
		var fieldParts []string
		for k, v := range pt.Fields {
			fieldParts = append(fieldParts, escapeLP(k, false)+"="+v)
		}
		fieldSet := strings.Join(fieldParts, ",")

		if tagSet != "" {
			data.WriteString(fmt.Sprintf("%s,%s %s %d\n", measurement, tagSet, fieldSet, pt.Time))
		} else {
			data.WriteString(fmt.Sprintf("%s %s %d\n", measurement, fieldSet, pt.Time))
		}
	}

	if data.Len() == 0 {
		return nil
	}

	c.log.Debugf("InfluxDB write URL: %s", url)
	c.log.Debugf("InfluxDB line protocol:\n%s", data.String())

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(data.String()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("post to influxdb: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode/100 != 2 {
		c.log.Errorf("InfluxDB write failed: status=%d body=%s data=%s",
			resp.StatusCode, string(body), data.String())
		return fmt.Errorf("influxdb write status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Health checks if InfluxDB is reachable.
func (c *Client) Health() error {
	resp, err := http.Get(c.URL + "/ping")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		return fmt.Errorf("influxdb ping returned status %d", resp.StatusCode)
	}
	return nil
}

// escapeLP escapes special characters for InfluxDB line protocol.
// In measurement names: escape comma and space.
// In tag keys/values: also escape equals sign.
func escapeLP(s string, isTag bool) string {
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, " ", `\ `)
	if isTag {
		s = strings.ReplaceAll(s, "=", `\=`)
	}
	return s
}
