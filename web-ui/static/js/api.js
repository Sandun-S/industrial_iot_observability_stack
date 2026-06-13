/**
 * IIoT Observability Stack — API Client
 *
 * Talks to the Web UI backend at the same origin (port 8080).
 * The backend proxies InfluxDB and Grafana requests.
 * No auth required — designed for isolated homelab/industrial networks.
 */

const API = {
  baseUrl: '',

  async _fetch(method, path, body) {
    const url = `${this.baseUrl}${path}`;
    const headers = { 'Content-Type': 'application/json' };
    const opts = { method, headers };
    if (body) opts.body = JSON.stringify(body);

    console.log(`[API] ${method} ${url}`);
    const res = await fetch(url, opts);
    const data = await res.json().catch(() => ({}));

    if (!res.ok) {
      throw new Error(data.message || data.error || `HTTP ${res.status}`);
    }

    return data;
  },

  _get(path)  { return this._fetch('GET', path); },
  _post(path, body) { return this._fetch('POST', path, body); },
  _put(path, body)  { return this._fetch('PUT', path, body); },
  _delete(path)     { return this._fetch('DELETE', path); },

  // ── Health ───────────────────────────────────────────────────────────────
  health()       { return this._get('/api/health'); },
  status()       { return this._get('/api/status'); },

  // ── Readers ──────────────────────────────────────────────────────────────
  listReaders()  { return this._get('/api/readers'); },

  // ── Sensors ──────────────────────────────────────────────────────────────
  listSensors(reader)   { return this._get(`/api/readers/${reader}/sensors`); },
  addSensor(reader, s)  { return this._post(`/api/readers/${reader}/sensors`, s); },

  // ── InfluxDB ─────────────────────────────────────────────────────────────
  influxHealth()         { return this._get('/api/influx/health'); },
  listMeasurements()     { return this._get('/api/influx/measurements'); },
  getLatest(measurement) {
    const q = measurement ? `?measurement=${encodeURIComponent(measurement)}` : '';
    return this._get(`/api/influx/latest${q}`);
  },
  query(q) {
    return this._get(`/api/influx/query?q=${encodeURIComponent(q)}`);
  },
  tagValues(measurement, tag) {
    return this._get(`/api/influx/tags?measurement=${encodeURIComponent(measurement)}&tag=${encodeURIComponent(tag)}`);
  },

  // ── Grafana ──────────────────────────────────────────────────────────────
  grafanaHealth()         { return this._get('/api/grafana/health'); },
  listDashboards()        { return this._get('/api/grafana/dashboards'); },
  createDashboard(title, measurements, fields, tags) {
    return this._post('/api/grafana/dashboards', { title, measurements, fields, tags });
  },
  deleteDashboard(uid)    { return this._delete(`/api/grafana/dashboards?uid=${encodeURIComponent(uid)}`); },
  ensureDatasource()      { return this._get('/api/grafana/datasource'); },
};

export default API;
