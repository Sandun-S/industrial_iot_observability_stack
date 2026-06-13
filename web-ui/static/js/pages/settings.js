/**
 * IIoT Observability Stack — Settings Page
 * System configuration: InfluxDB, Grafana, and MQTT broker defaults.
 */

import API from '../api.js';
import Components from '../components.js';

const SettingsPage = {
  async render(container) {
    const currentGrafanaUrl = localStorage.getItem('iiot_grafana_url') || `${window.location.protocol}//${window.location.hostname}:3000`;
    const currentGrafanaToken = localStorage.getItem('iiot_grafana_token') || '';

    container.innerHTML = `
      <div class="page-header">
        <h1>⚡ Settings</h1>
        <button class="btn" onclick="SettingsPage.saveSettings()">Save Settings</button>
      </div>

      <div class="card">
        <div class="card-header">📈 Grafana Connection</div>
        <form id="settings-form">
          ${Components.field('Grafana URL', 'grafana_url', {
            value: currentGrafanaUrl,
            placeholder: 'http://grafana:3000',
            help: 'Used for iframe embedding and dashboard links. Default: http://HOSTNAME:3000'
          })}
          ${Components.field('Grafana Service Account Token', 'grafana_token', {
            type: 'password',
            value: currentGrafanaToken,
            placeholder: 'Optional — for API dashboard creation',
            help: 'Create in Grafana: Administration → Service accounts → Add service account → Admin role → Add token'
          })}
        </form>
      </div>

      <div class="card" style="margin-top: 16px;">
        <div class="card-header">💾 InfluxDB Connection</div>
        <p style="color: var(--text-secondary); font-size: 14px; margin-bottom: 8px;">
          InfluxDB connection is configured via environment variables in the Docker stack.
        </p>
        <table>
          <tr><td style="width: 200px; color: var(--text-secondary);">URL</td><td><code id="status-influx-url">Loading...</code></td></tr>
          <tr><td style="color: var(--text-secondary);">Database</td><td><code id="status-influx-db">Loading...</code></td></tr>
          <tr><td style="color: var(--text-secondary);">Status</td><td><span id="status-influx-status">Checking...</span></td></tr>
        </table>
      </div>

      <div class="card" style="margin-top: 16px;">
        <div class="card-header">📡 Default MQTT Configuration</div>
        <p style="color: var(--text-secondary); font-size: 14px; margin-bottom: 8px;">
          These are the default values used when creating new MQTT readers.
        </p>
        <form id="mqtt-defaults-form">
          ${Components.field('Default MQTT Broker', 'default_broker', {
            value: localStorage.getItem('iiot_default_broker') || 'tcp://mosquitto:1883',
            placeholder: 'tcp://mosquitto:1883'
          })}
          ${Components.field('Default QoS', 'default_qos', {
            type: 'select',
            value: localStorage.getItem('iiot_default_qos') || '1',
            options: [
              {value:'0', label:'0 - At most once'},
              {value:'1', label:'1 - At least once'},
              {value:'2', label:'2 - Exactly once'}
            ]
          })}
        </form>
      </div>

      <div class="card" style="margin-top: 16px;">
        <div class="card-header">🗄️ PostgreSQL Exporter (to-postgres)</div>
        <p style="color: var(--text-secondary); font-size: 13px; margin-bottom: 12px;">
          Sync InfluxDB data to an external PostgreSQL/TimescaleDB. Set the connection URL and interval below.
          Leave URL empty to disable.
        </p>
        <form id="exporter-form">
          ${Components.field('Enable', 'enabled', {
            type: 'select',
            value: localStorage.getItem('iiot_export_enabled') || 'false',
            options: [{value:'false',label:'Disabled'},{value:'true',label:'Enabled'}]
          })}
          ${Components.field('PostgreSQL URL', 'postgres_url', {
            value: localStorage.getItem('iiot_export_postgres_url') || '',
            placeholder: 'postgres://user:pass@host:5432/db?sslmode=disable',
            help: 'Connection string for the target PostgreSQL/TimescaleDB'
          })}
          ${Components.field('Sync Interval (seconds)', 'sync_interval', {
            value: localStorage.getItem('iiot_export_sync_interval') || '60',
            placeholder: '60'
          })}
          ${Components.field('Site Label', 'site', {
            value: localStorage.getItem('iiot_export_site') || 'default',
            placeholder: 'default'
          })}
        </form>
        <div style="margin-top: 8px;">
          <button class="btn btn-primary" onclick="SettingsPage.saveExporter()">Save & Apply</button>
          <span id="export-status" style="margin-left: 12px; font-size: 13px; color: var(--text-secondary);"></span>
        </div>
      </div>

      <div class="card" style="margin-top: 16px;">
        <div class="card-header">🛠️ System Info</div>
        <div id="sys-info"><div class="loading">Loading system info...</div></div>
      </div>
    `;

    await this.loadInfluxStatus();
    await this.loadSystemInfo();
  },

  async loadInfluxStatus() {
    try {
      const status = await API.influxHealth();
      document.getElementById('status-influx-url').textContent = status.url || '—';
      document.getElementById('status-influx-db').textContent = status.database || '—';
      document.getElementById('status-influx-status').innerHTML =
        Components.statusDot(status.status, status.status);
    } catch {
      document.getElementById('status-influx-status').innerHTML =
        Components.statusDot('error', 'unreachable');
    }
  },

  async loadSystemInfo() {
    const el = document.getElementById('sys-info');
    if (!el) return;

    try {
      const status = await API.status();
      el.innerHTML = `<pre style="font-size: 12px; color: var(--text-secondary); overflow-x: auto;">${Components.escape(JSON.stringify(status, null, 2))}</pre>`;
    } catch (err) {
      el.innerHTML = Components.alert('error', err.message);
    }
  },

  async saveExporter() {
    const form = document.getElementById('exporter-form');
    if (!form) return;
    const data = Components.getFormData(form);
    // Save to localStorage
    localStorage.setItem('iiot_export_enabled', data.enabled);
    localStorage.setItem('iiot_export_postgres_url', data.postgres_url);
    localStorage.setItem('iiot_export_sync_interval', data.sync_interval);
    localStorage.setItem('iiot_export_site', data.site);
    // Send to exporter via backend proxy
    const statusEl = document.getElementById('export-status');
    try {
      const resp = await API._post('/api/exporter/config', {
        enabled: data.enabled === 'true',
        postgres_url: data.postgres_url,
        sync_interval: parseInt(data.sync_interval) || 60,
        site: data.site || 'default',
      });
      statusEl.textContent = '✓ Saved & applied';
      statusEl.style.color = 'var(--green)';
    } catch (err) {
      statusEl.textContent = '⚠ ' + err.message + ' (is to-postgres container running?)';
      statusEl.style.color = 'var(--yellow)';
    }
  },

  saveSettings() {
    const form = document.getElementById('settings-form');
    if (form) {
      const data = Components.getFormData(form);
      if (data.grafana_url) localStorage.setItem('iiot_grafana_url', data.grafana_url);
      if (data.grafana_token) localStorage.setItem('iiot_grafana_token', data.grafana_token);
    }

    const mqttForm = document.getElementById('mqtt-defaults-form');
    if (mqttForm) {
      const mqttData = Components.getFormData(mqttForm);
      if (mqttData.default_broker) localStorage.setItem('iiot_default_broker', mqttData.default_broker);
      if (mqttData.default_qos) localStorage.setItem('iiot_default_qos', mqttData.default_qos);
    }

    // Also save exporter config
    this.saveExporter();
  },
};

window.SettingsPage = SettingsPage;
export default SettingsPage;
