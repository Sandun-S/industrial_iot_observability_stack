/**
 * IIoT Observability Stack — Sensors Page
 * Browse sensors within MQTT reader configs.
 */

import API from '../api.js';
import Components from '../components.js';

const SensorsPage = {
  selectedReader: null,

  async render(container) {
    // Get reader name from hash: #sensors/reader-name
    const hash = window.location.hash.replace('#', '').split('/');
    this.selectedReader = hash[1] || null;

    container.innerHTML = `
      <div class="page-header">
        <h1>🔍 Sensors ${this.selectedReader ? `— ${Components.escape(this.selectedReader)}` : ''}</h1>
        <div>
          ${this.selectedReader ? `<button class="btn btn-primary" onclick="SensorsPage.showAddSensorModal()">+ Add Sensor</button>` : ''}
          <button class="btn" onclick="window.location.reload()">Refresh</button>
        </div>
      </div>
      ${this.selectedReader ? '' : '<p style="color: var(--text-secondary); margin-bottom: 16px;">Select a reader to view its sensors:</p>'}
      <div id="sensors-content"><div class="loading">Loading...</div></div>
    `;

    if (this.selectedReader) {
      await this.loadSensors();
    } else {
      await this.loadReaderList();
    }
  },

  async loadReaderList() {
    const el = document.getElementById('sensors-content');
    if (!el) return;

    try {
      const readers = await API.listReaders();

      if (!readers || readers.length === 0) {
        el.innerHTML = '<div class="card"><p style="color: var(--text-secondary); text-align: center; padding: 24px;">No readers configured. <a href="#readers" style="color: var(--accent);">Add one first</a>.</p></div>';
        return;
      }

      el.innerHTML = Components.table(readers, [
        { key: 'name', label: 'Reader' },
        { key: 'broker', label: 'Broker' },
        { key: 'sensors', label: 'Sensors' },
        {
          key: 'config_file', label: 'Action',
          render: (_, row) => `<a href="#sensors/${encodeURIComponent(row.name || row.config_file)}" class="btn btn-sm">View Sensors →</a>`,
        },
      ]);
    } catch (err) {
      el.innerHTML = Components.alert('error', `Failed to load readers: ${err.message}`);
    }
  },

  async loadSensors() {
    const el = document.getElementById('sensors-content');
    if (!el || !this.selectedReader) return;

    try {
      // Load reader info from readers list
      const readers = await API.listReaders();
      const reader = (readers || []).find(r =>
        r.name === this.selectedReader || r.config_file === this.selectedReader ||
        r.config_file === this.selectedReader + '.yaml' || r.config_file === this.selectedReader + '.yml'
      );

      // Load sensors
      const sensors = await API.listSensors(this.selectedReader);

      let html = '';
      if (reader) {
        html += `
          <div class="card">
            <div class="card-header">📡 Reader Info</div>
            <table>
              <tr><td style="width: 120px; color: var(--text-secondary);">Name</td><td>${Components.escape(reader.name)}</td></tr>
              <tr><td style="color: var(--text-secondary);">Broker</td><td>${Components.escape(reader.broker)}</td></tr>
              <tr><td style="color: var(--text-secondary);">Description</td><td>${Components.escape(reader.description || '—')}</td></tr>
              <tr><td style="color: var(--text-secondary);">Config File</td><td><code>${Components.escape(reader.config_file)}</code></td></tr>
            </table>
          </div>
        `;
      }

      if (!sensors || sensors.length === 0) {
        html += '<div class="card"><p style="color: var(--text-secondary); text-align: center; padding: 24px;">No sensors configured for this reader. Add one to start collecting data.</p></div>';
      } else {
        html += Components.table(sensors, [
          { key: 'name', label: 'Sensor Name' },
          { key: 'topic', label: 'MQTT Topic' },
          { key: 'measurement', label: 'Measurement' },
          { key: 'fields', label: 'Fields' },
          {
            key: 'csv', label: 'Type',
            render: v => v ? Components.badge('CSV', 'yellow') : Components.badge('JSON', 'blue'),
          },
          {
            key: 'tags', label: 'Tags',
            render: v => {
              if (!v || Object.keys(v).length === 0) return '—';
              return Object.entries(v).map(([k, val]) =>
                `<span style="font-size: 11px; color: var(--text-secondary);">${Components.escape(k)}=${Components.escape(val)}</span>`
              ).join(', ');
            },
          },
        ]);
      }

      el.innerHTML = html;
    } catch (err) {
      el.innerHTML = Components.alert('error', `Failed to load sensors: ${err.message}`);
    }
  },

  showAddSensorModal() {
    if (!this.selectedReader) return;

    const content = `
      <p style="color: var(--text-secondary); margin-bottom: 16px;">
        Add a new sensor to reader <strong>${Components.escape(this.selectedReader)}</strong>.
      </p>
      <form id="add-sensor-form">
        ${Components.field('Sensor Name', 'name', { required: true, placeholder: 'e.g. Server Room Temperature' })}
        ${Components.field('MQTT Topic', 'topic', { required: true, placeholder: 'e.g. sensors/server-room/temperature', help: 'Supports MQTT wildcards + and #' })}
        ${Components.field('Measurement', 'measurement', { required: true, placeholder: 'e.g. environment' })}

        <div style="border: 1px solid var(--border); border-radius: 6px; padding: 12px; margin-bottom: 12px;">
          <h4 style="margin-bottom: 8px; font-size: 13px;">Field Mappings</h4>
          <div id="field-mappings">
            <div class="field-row" style="display: flex; gap: 8px; margin-bottom: 8px;">
              <input name="field_key_0" placeholder="Field key (e.g. temperature_c)" style="flex: 1;" />
              <input name="field_path_0" placeholder="JSON path (e.g. temperature)" style="flex: 1;" />
              <select name="field_type_0" style="width: 100px;">
                <option value="float">float</option>
                <option value="int">int</option>
                <option value="string">string</option>
              </select>
              <input name="field_unit_0" placeholder="Unit" style="width: 80px;" />
            </div>
          </div>
          <button type="button" class="btn btn-sm" onclick="SensorsPage.addFieldRow()">+ Add Field</button>
        </div>

        ${Components.field('Tags (key=value, comma separated)', 'tags_str', { placeholder: 'location=server-room,building=main', help: 'Optional static tags for all readings from this sensor' })}
      </form>
    `;

    const actions = `
      <button class="btn" onclick="Components.closeModal()">Cancel</button>
      <button class="btn btn-primary" onclick="SensorsPage.submitAddSensor()">Add Sensor</button>
    `;

    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.innerHTML = `<div class="modal"><h3>Add Sensor</h3>${content}<div class="form-actions">${actions}</div></div>`;
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
    document.body.appendChild(overlay);
  },

  fieldCount: 1,

  addFieldRow() {
    const container = document.getElementById('field-mappings');
    if (!container) return;

    const idx = this.fieldCount++;
    const row = document.createElement('div');
    row.className = 'field-row';
    row.style.cssText = 'display: flex; gap: 8px; margin-bottom: 8px;';
    row.innerHTML = `
      <input name="field_key_${idx}" placeholder="Field key" style="flex: 1;" />
      <input name="field_path_${idx}" placeholder="JSON path" style="flex: 1;" />
      <select name="field_type_${idx}" style="width: 100px;">
        <option value="float">float</option>
        <option value="int">int</option>
        <option value="string">string</option>
      </select>
      <input name="field_unit_${idx}" placeholder="Unit" style="width: 80px;" />
    `;
    container.appendChild(row);
  },

  async submitAddSensor() {
    const form = document.getElementById('add-sensor-form');
    if (!form) return;

    const data = Components.getFormData(form);

    // Collect fields
    const fields = [];
    for (let i = 0; i < 10; i++) {
      const key = data[`field_key_${i}`];
      if (key) {
        fields.push({
          key,
          json_path: data[`field_path_${i}`] || '.',
          type: data[`field_type_${i}`] || 'float',
          unit: data[`field_unit_${i}`] || '',
        });
      }
    }

    // Parse tags
    const tags = {};
    if (data.tags_str) {
      data.tags_str.split(',').forEach(pair => {
        const [k, v] = pair.split('=').map(s => s.trim());
        if (k && v) tags[k] = v;
      });
    }

    try {
      await API.addSensor(this.selectedReader, {
        name: data.name,
        topic: data.topic,
        measurement: data.measurement,
        fields,
        tags,
      });

      Components.closeModal();
      await this.loadSensors();
    } catch (err) {
      alert(`Failed to add sensor: ${err.message}`);
    }
  },
};

export default SensorsPage;
