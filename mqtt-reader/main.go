// Industrial IoT Observability Stack — MQTT Reader
//
// Reads YAML config → subscribes to MQTT topics → extracts values via JSON path
// or CSV column mapping → writes directly to InfluxDB v1 using line protocol.
//
// Each reader instance handles ONE config file (one MQTT broker endpoint).
// For multiple brokers, deploy separate reader containers.
//
// Env vars:
//
//	CONFIG_PATH  — Path to YAML config file (required)
//	LOG_LEVEL    — debug, info, warn, error (default: info)
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/Sandun-S/industrial-iot-observability-stack/mqtt-reader/config"
	"github.com/Sandun-S/industrial-iot-observability-stack/mqtt-reader/influx"
	"github.com/Sandun-S/industrial-iot-observability-stack/mqtt-reader/logger"
	mqtthandler "github.com/Sandun-S/industrial-iot-observability-stack/mqtt-reader/mqtt"
)

func main() {
	log := logger.New("mqtt-reader")

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH env var is required — point to a reader YAML config file")
	}

	// ── Load YAML config ──────────────────────────────────────────────────
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Infof("Loaded config: reader=%s broker=%s sensors=%d",
		cfg.Reader.Name, cfg.MQTT.Broker, len(cfg.Sensors))

	// ── Setup InfluxDB client ─────────────────────────────────────────────
	influxClient := influx.NewClient(
		cfg.InfluxDB.URL,
		cfg.InfluxDB.Database,
		cfg.InfluxDB.Username,
		cfg.InfluxDB.Password,
		log,
	)

	// Health-check InfluxDB
	if err := influxClient.Health(); err != nil {
		log.Warnf("InfluxDB not reachable at %s: %v — will retry on writes", cfg.InfluxDB.URL, err)
	} else {
		log.Infof("InfluxDB reachable at %s (db=%s)", cfg.InfluxDB.URL, cfg.InfluxDB.Database)
	}

	// ── Build topic rules ─────────────────────────────────────────────────
	rules := mqtthandler.BuildRules(cfg)
	if len(rules) == 0 {
		log.Fatal("No topic rules generated — check sensor configs")
	}

	topics := mqtthandler.UniqueTopics(rules)
	log.Infof("Prepared %d rules across %d unique topics", len(rules), len(topics))

	// ── MQTT client ID ────────────────────────────────────────────────────
	clientID := cfg.MQTT.ClientID
	if clientID == "" {
		clientID = mqtthandler.ClientID(cfg.Reader.Name)
	}

	// ── Connect to MQTT broker ────────────────────────────────────────────
	// On-connect handler: subscribe to all topics
	onConnect := func(c mqtt.Client) {
		log.Infof("Connected to MQTT broker %s", cfg.MQTT.Broker)
		filters := make(map[string]byte, len(topics))
		for _, t := range topics {
			filters[t] = cfg.MQTT.QoS
			log.Infof("Subscribing: %s (qos=%d)", t, cfg.MQTT.QoS)
		}

		token := c.SubscribeMultiple(filters, func(_ mqtt.Client, msg mqtt.Message) {
			n := mqtthandler.HandleMessage(msg, rules, influxClient, cfg.Reader.Name,
				func(format string, args ...any) {
					log.Debugf(format, args...)
				})
			if n > 0 {
				log.Debugf("Wrote %d data points from topic %s", n, msg.Topic())
			}
		})
		token.Wait()
		if token.Error() != nil {
			log.Warnf("Subscribe error: %v", token.Error())
		}
	}

	onLost := func(_ mqtt.Client, err error) {
		log.Warnf("MQTT connection lost: %v — will reconnect", err)
	}

	client := mqtthandler.ConnectWithRetry(
		cfg.MQTT.Broker, clientID,
		cfg.MQTT.Username, cfg.MQTT.Password,
		cfg.MQTT.QoS, cfg.MQTT.ReconnectDelay,
		onConnect, onLost,
		log.Infof,
	)

	log.Infof("Running — reader=%s subscribed to %d topics", cfg.Reader.Name, len(topics))

	// ── Wait for shutdown signal ──────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Infof("Received signal %v — disconnecting", sig)

	client.Disconnect(5000) // 5s quiesce
	log.Infof("Shutdown complete")

	// Print a separator so logs are easy to find in Docker output
	fmt.Fprintln(os.Stderr, "—— mqtt-reader stopped ——")
}
