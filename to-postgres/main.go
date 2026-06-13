// to-postgres — InfluxDB → PostgreSQL exporter
//
// Standalone container. Reads InfluxDB v1, writes to PostgreSQL/TimescaleDB.
// Configurable via HTTP API (called by Web UI settings page) or env vars.
//
// Env vars:
//   POSTGRES_URL   — PostgreSQL connection URL (empty = wait for API config)
//   INFLUX_URL     — InfluxDB v1 URL (default: http://influxdb:8086)
//   INFLUX_DB      — InfluxDB database (default: iiot)
//   SYNC_INTERVAL  — seconds between syncs (default: 60)
//   SITE           — site label (default: "default")
//   API_PORT       — config API port (default: 8089)

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type Config struct {
	Enabled      bool   `json:"enabled"`
	PostgresURL  string `json:"postgres_url"`
	SyncInterval int    `json:"sync_interval"`
	Site         string `json:"site"`
}

var (
	cfg    Config
	cfgMu  sync.RWMutex
	db     *sql.DB
	dbMu   sync.Mutex
	stopCh chan struct{}
	loopMu sync.Mutex
)

func main() {
	influxURL := getenv("INFLUX_URL", "http://influxdb:8086")
	influxDB := getenv("INFLUX_DB", "iiot")
	apiPort := getenv("API_PORT", "8089")

	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[to-postgres] ")

	cfg = Config{
		Enabled:      false,
		PostgresURL:  os.Getenv("POSTGRES_URL"),
		SyncInterval: getenvInt("SYNC_INTERVAL", 60),
		Site:         getenv("SITE", "default"),
	}
	if cfg.PostgresURL != "" {
		cfg.Enabled = true
	}

	log.Printf("InfluxDB: %s/%s", influxURL, influxDB)
	if cfg.Enabled {
		log.Printf("PostgreSQL: %s (every %ds)", maskPassword(cfg.PostgresURL), cfg.SyncInterval)
		startLoop(influxURL, influxDB)
	} else {
		log.Println("POSTGRES_URL not set — waiting for config via Web UI")
	}

	// ── Config API ───────────────────────────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/config", configHandler(influxURL, influxDB))

	log.Printf("API on :%s", apiPort)
	log.Fatal(http.ListenAndServe(":"+apiPort, corsMiddleware(mux)))
}

func configHandler(influxURL, influxDB string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			cfgMu.RLock()
			c := cfg
			cfgMu.RUnlock()
			c.PostgresURL = maskPassword(c.PostgresURL)
			json.NewEncoder(w).Encode(map[string]any{
				"config":      c,
				"db_connected": db != nil,
			})

		case http.MethodPost, http.MethodPut:
			var newCfg Config
			if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
				return
			}
			applyConfig(influxURL, influxDB, newCfg)
			cfgMu.RLock()
			c := cfg
			cfgMu.RUnlock()
			c.PostgresURL = maskPassword(c.PostgresURL)
			json.NewEncoder(w).Encode(map[string]any{
				"message": "config updated",
				"config":  c,
			})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		}
	}
}

func applyConfig(influxURL, influxDB string, newCfg Config) {
	loopMu.Lock()
	if stopCh != nil {
		close(stopCh)
		stopCh = nil
	}
	loopMu.Unlock()

	dbMu.Lock()
	if db != nil {
		db.Close()
		db = nil
	}
	dbMu.Unlock()

	if newCfg.SyncInterval <= 0 {
		newCfg.SyncInterval = 60
	}
	if newCfg.Site == "" {
		newCfg.Site = "default"
	}

	if newCfg.Enabled && newCfg.PostgresURL != "" {
		newDB, err := sql.Open("postgres", newCfg.PostgresURL)
		if err != nil {
			log.Printf("ERROR: connect: %v", err)
			newCfg.Enabled = false
		} else {
			newDB.SetMaxOpenConns(3)
			newDB.SetMaxIdleConns(1)
			if err := newDB.Ping(); err != nil {
				log.Printf("ERROR: ping: %v", err)
				newDB.Close()
				newCfg.Enabled = false
			} else {
				dbMu.Lock()
				db = newDB
				dbMu.Unlock()
				ensureTables(db)
				log.Println("PostgreSQL connected")
			}
		}
	}

	cfgMu.Lock()
	cfg = newCfg
	cfgMu.Unlock()

	if cfg.Enabled && db != nil {
		startLoop(influxURL, influxDB)
		log.Println("Sync started")
	} else {
		log.Println("Exporter disabled")
	}
}

func startLoop(influxURL, influxDB string) {
	loopMu.Lock()
	defer loopMu.Unlock()
	if stopCh != nil {
		return
	}

	cfgMu.RLock()
	interval := cfg.SyncInterval
	cfgMu.RUnlock()

	ch := make(chan struct{})
	stopCh = ch

	go func() {
		log.Printf("Syncing every %ds", interval)
		doSync(influxURL, influxDB)
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				doSync(influxURL, influxDB)
			case <-ch:
				log.Println("Loop stopped")
				return
			}
		}
	}()
}

func doSync(influxURL, influxDB string) {
	cfgMu.RLock()
	site := cfg.Site
	cfgMu.RUnlock()

	dbMu.Lock()
	d := db
	dbMu.Unlock()
	if d == nil {
		return
	}

	measurements, err := queryMeasurements(influxURL, influxDB)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-5 * time.Minute)
	count := 0
	for _, m := range measurements {
		q := fmt.Sprintf(`SELECT * FROM "%s" WHERE time > %d000000000`, m, cutoff.Unix())
		rows, err := queryInflux(influxURL, influxDB, q)
		if err != nil {
			continue
		}
		for _, row := range rows {
			device := fmt.Sprint(row["device"])
			reading := fmt.Sprint(row["reading"])
			rawTime := row["time"]
			val, ok := row[reading]
			if !ok || val == nil {
				continue
			}
			var fv float64
			switch v := val.(type) {
			case float64:
				fv = v
			case json.Number:
				fv, _ = v.Float64()
			default:
				continue
			}
			var t time.Time
			switch v := rawTime.(type) {
			case float64:
				t = time.Unix(0, int64(v)*int64(time.Millisecond))
			case string:
				t, _ = time.Parse(time.RFC3339, v)
			default:
				t = time.Now()
			}
			d.Exec(`INSERT INTO iiot_export (site,device,reading,measurement,value,logged_time)
				VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
				site, device, reading, m, fv, t)
			count++
		}
	}
	if count > 0 {
		log.Printf("Synced %d rows", count)
	}
}

func queryMeasurements(u, db string) ([]string, error) {
	rows, err := queryInflux(u, db, "SHOW MEASUREMENTS")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, r := range rows {
		if n, ok := r["name"]; ok {
			out = append(out, fmt.Sprint(n))
		}
	}
	return out, nil
}

func queryInflux(u, db, q string) ([]map[string]any, error) {
	queryURL := fmt.Sprintf("%s/query?db=%s&epoch=ms&q=%s", u, db, url.QueryEscape(q))
	resp, err := http.Get(queryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var ir struct {
		Results []struct {
			Series []struct {
				Columns []string        `json:"columns"`
				Values  [][]interface{} `json:"values"`
			} `json:"series"`
			Error string `json:"error"`
		} `json:"results"`
	}
	json.Unmarshal(body, &ir)
	var out []map[string]any
	for _, r := range ir.Results {
		if r.Error != "" {
			return nil, fmt.Errorf(r.Error)
		}
		for _, s := range r.Series {
			for _, vals := range s.Values {
				row := map[string]any{}
				for i, c := range s.Columns {
					if i < len(vals) {
						row[c] = vals[i]
					}
				}
				out = append(out, row)
			}
		}
	}
	return out, nil
}

func ensureTables(d *sql.DB) {
	d.Exec(`CREATE TABLE IF NOT EXISTS iiot_export (
		id BIGSERIAL PRIMARY KEY, site TEXT DEFAULT '', device TEXT NOT NULL,
		reading TEXT NOT NULL, measurement TEXT NOT NULL, value DOUBLE PRECISION,
		logged_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		inserted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(site,device,reading,measurement,logged_time))`)
	d.Exec(`CREATE INDEX IF NOT EXISTS idx_iie_time ON iiot_export (logged_time DESC)`)
	d.Exec(`SELECT create_hypertable('iiot_export','logged_time',if_not_exists=>TRUE)`)
}

func maskPassword(dsn string) string {
	if i := strings.Index(dsn, "://"); i > 0 {
		r := dsn[i+3:]
		if a := strings.Index(r, "@"); a > 0 {
			up := r[:a]
			if c := strings.Index(up, ":"); c > 0 {
				return dsn[:i+3] + up[:c] + ":****" + r[a:]
			}
		}
	}
	return dsn
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func getenvInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return d
}
