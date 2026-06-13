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

      // Each row has the value stored under its field key (row[row.reading])
      const readings = [];
      for (const row of data) {
        if (!row.time) continue;
        const fieldKey = row.reading || 'value';
        // Look up the actual value using the field key name
        let val = row[fieldKey];
        if (val === undefined || val === null) {
          // Fallback: try common keys
          val = row.value ?? row._value ?? null;
        }
        if (val !== undefined && val !== null) {
          readings.push({
            measurement: row.measurement || '—',
            device: row.device || '—',
            reading: fieldKey,
            value: typeof val === 'number' ? val.toFixed(2) : String(val),
            time: row.time,
          });
        }
      }

      if (readings.length === 0) {
        el.innerHTML = '<p style="color: var(--text-secondary); padding: 24px; text-align: center;">No readings found. Check Grafana for data.</p>';
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
      // Discover all measurements from InfluxDB
      const measurements = await API.listMeasurements();
      const names = (measurements || []).map(m => m.name).filter(Boolean);

      if (names.length === 0) {
        alert('No measurements found in InfluxDB yet. Publish some data first.');
        return;
      }

      let created = 0;
      for (const name of names) {
        try {
          // Discover device/reading pairs for this measurement
          const devices = await API.tagValues(name, 'device');
          const deviceList = (devices || []).map(d => d.value).filter(Boolean);

          // For each device, discover which readings actually exist
          const deviceGroups = [];
          for (const device of deviceList) {
            const deviceReadings = await API.tagValuesFiltered(name, 'reading', 'device', device);
            const readingList = (deviceReadings || []).map(r => r.value).filter(Boolean);
            if (readingList.length > 0) {
              deviceGroups.push({
                device,
                readings: readingList.map(key => ({ key, unit: '' })),
              });
            }
          }

          if (deviceGroups.length === 0) {
            // Fallback: no devices found, create generic dashboard
            deviceGroups.push({ device: name, readings: readingList.map(k => ({ key: k, unit: '' })) });
          }

          await API.createDashboardForMeasurement(`IIoT: ${name}`, name, deviceGroups);
          console.log(`Dashboard created for: ${name}`);
          created++;
        } catch (err) {
          console.warn(`Dashboard for ${name}: ${err.message}`);
        }
      }

      alert(`Created/updated ${created} dashboard(s). Open Grafana to view them.`);
      window.location.hash = '#grafana';
    } catch (err) {
      alert(`Failed: ${err.message}`);
    }
  },
};

window.DashboardPage = DashboardPage;
export default DashboardPage;
