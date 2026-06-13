/**
 * IIoT Observability Stack — Dashboard Page
 * System overview: status of all components, latest readings, quick links.
 */

import API from '../api.js';
import Components from '../components.js';

const DashboardPage = {
  async render(container) {
    container.innerHTML = `
      <div class="page-header">
        <h1>📊 Dashboard</h1>
        <button class="btn" onclick="window.location.reload()">Refresh</button>
      </div>
      <div id="status-grid" class="status-grid"></div>
      <div class="card">
        <div class="card-header">📡 Latest Readings</div>
        <div id="latest-readings"><div class="loading">Loading readings...</div></div>
      </div>
      <div class="card" style="margin-top: 16px;">
        <div class="card-header">⚡ Quick Actions</div>
        <div style="display: flex; gap: 8px;">
          <button class="btn btn-primary" onclick="window.location.hash='#readers'">📡 Manage Readers</button>
          <button class="btn btn-primary" onclick="window.location.hash='#sensors'">🔍 View Sensors</button>
          <button class="btn" onclick="window.location.hash='#grafana'">📈 Open Grafana</button>
          <button class="btn" onclick="DashboardPage.autoCreateDashboards()">✨ Auto-Create Dashboards</button>
        </div>
      </div>
    `;

    await this.loadStatus();
    await this.loadLatestReadings();
  },

  async loadStatus() {
    const grid = document.getElementById('status-grid');
    if (!grid) return;

    try {
      const status = await API.status();
      grid.innerHTML = this.renderStatusCards(status);
    } catch (err) {
      grid.innerHTML = Components.alert('error', `Failed to load status: ${err.message}`);
    }
  },

  renderStatusCards(status) {
    const services = [
      { key: 'web_ui',   label: 'Web UI',    icon: '🌐' },
      { key: 'influxdb', label: 'InfluxDB',  icon: '💾' },
      { key: 'grafana',  label: 'Grafana',   icon: '📈' },
    ];

    return services.map(svc => {
      let info = status[svc.key];
      if (typeof info === 'string') info = { status: info };
      if (!info) info = { status: 'unknown' };

      const statusText = info.status || 'unknown';
      const statusCls = statusText === 'ok' ? 'ok' : 'error';

      let detail = '';
      if (info.url) detail += `<div style="font-size: 11px; color: var(--text-secondary); margin-top: 4px;">${Components.escape(info.url)}</div>`;
      if (info.db) detail += `<div style="font-size: 11px; color: var(--text-secondary);">DB: ${Components.escape(info.db)}</div>`;

      return `
        <div class="status-card">
          <div class="label">${svc.icon} ${svc.label}</div>
          <div class="value">
            <span class="status-dot ${statusCls}"></span>
            ${statusText}
          </div>
          ${detail}
        </div>
      `;
    }).join('');
  },

  async loadLatestReadings() {
    const el = document.getElementById('latest-readings');
    if (!el) return;

    try {
      const data = await API.getLatest();

      if (!data || data.length === 0) {
        el.innerHTML = '<p style="color: var(--text-secondary); padding: 24px; text-align: center;">No data yet. Publish some MQTT messages to see readings here.</p>';
        return;
      }

      // Build a flat table of latest readings
      let rows = [];
      for (const group of data) {
        const measurement = group.measurement || 'unknown';
        const points = group.data ? [group.data] : [];
        // If group itself contains time/value fields
        if (typeof group === 'object' && group.time) {
          rows.push(group);
        }
        // Handle array-form results
        if (Array.isArray(group)) {
          rows = rows.concat(group);
        }
      }

      // Try to extract readings from the influx response format
      const readings = [];
      for (const item of data) {
        if (item.data && item.data.time) {
          const d = item.data;
          const time = d.time;
          for (const [key, val] of Object.entries(d)) {
            if (key !== 'time' && key !== 'device' && key !== 'reading' && key !== 'reader' &&
                key !== 'site' && typeof val === 'number') {
              readings.push({
                measurement: item.measurement,
                device: d.device || '—',
                reading: key,
                value: typeof val === 'number' ? val.toFixed(2) : val,
                time: time,
              });
            }
          }
        } else if (item.measurement && typeof item === 'object') {
          // Direct from measurement list
          readings.push({
            measurement: item.measurement,
            device: item.device || '—',
            reading: item.reading || 'value',
            value: item.value || '—',
            time: item.time || item.logged_time,
          });
        }
      }

      if (readings.length === 0 && data.length > 0) {
        el.innerHTML = `<p style="color: var(--text-secondary); padding: 24px; text-align: center;">
          Data exists in InfluxDB but couldn't be parsed for display.
          <br>Check <a href="#grafana" style="color: var(--accent);">Grafana</a> for full visualization.
        </p>`;
        return;
      }

      if (readings.length === 0) {
        el.innerHTML = '<p style="color: var(--text-secondary); padding: 24px; text-align: center;">No readings found.</p>';
        return;
      }

      el.innerHTML = Components.table(readings.slice(0, 50), [
        { key: 'measurement', label: 'Measurement' },
        { key: 'device', label: 'Device' },
        { key: 'reading', label: 'Reading' },
        { key: 'value', label: 'Value' },
        { key: 'time', label: 'Time', render: v => Components.formatEpoch(v) },
      ]);
    } catch (err) {
      el.innerHTML = Components.alert('error', `Failed to load readings: ${err.message}`);
    }
  },

  async autoCreateDashboards() {
    try {
      // Get all measurements from InfluxDB
      const measurements = await API.listMeasurements();
      const names = (measurements || [])
        .map(m => m.name)
        .filter(Boolean);

      if (names.length === 0) {
        alert('No measurements found in InfluxDB yet. Publish some data first.');
        return;
      }

      for (const name of names) {
        try {
          await API.createDashboard(`IIoT: ${name}`, [name], [], []);
          console.log(`Dashboard created for: ${name}`);
        } catch (err) {
          console.warn(`Dashboard for ${name}: ${err.message}`);
        }
      }

      alert(`Created dashboards for ${names.length} measurements. Open Grafana to view them.`);
      window.location.hash = '#grafana';
    } catch (err) {
      alert(`Failed: ${err.message}`);
    }
  },
};

export default DashboardPage;
