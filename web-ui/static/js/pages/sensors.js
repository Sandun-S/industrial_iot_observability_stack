/**
 * IIoT Observability Stack — Sensors Page
 * Browse, add, edit, and delete sensors within MQTT reader configs.
 */

import API from '../api.js';
import Components from '../components.js';

const SensorsPage = {
  selectedReader: null,
  editingSensorName: null, // tracks if we're editing an existing sensor

  async render(container) {
    const hash = window.location.hash.replace('#', '').split('/');
    this.selectedReader = hash[1] || null;

    container.innerHTML = `
      <div class="page-header">
        <h1>🔍 Sensors ${this.selectedReader ? `— ${Components.escape(this.selectedReader)}` : ''}</h1>
        <div>
          ${this.selectedReader ? `<button class="btn btn-primary" onclick="SensorsPage.showSensorModal()">+ Add Sensor</button>` : ''}
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
        { key: 'config_file', label: 'Action', render: (_, row) => `<a href="#sensors/${encodeURIComponent(row.name || row.config_file)}" class="btn btn-sm">View Sensors →</a>` },
      ]);
    } catch (err) {
      el.innerHTML = Components.alert('error', `Failed: ${err.message}`);
    }
  },

  async loadSensors() {
    const el = document.getElementById('sensors-content');
    if (!el || !this.selectedReader) return;
    try {
      const readers = await API.listReaders();
      const reader = (readers || []).find(r => r.name === this.selectedReader || r.config_file === this.selectedReader || r.config_file === this.selectedReader + '.yaml' || r.config_file === this.selectedReader + '.yml');
      const sensors = await API.listSensors(this.selectedReader);

      let html = '';
      if (reader) {
        html += `<div class="card"><div class="card-header">📡 Reader Info</div>
          <table>
            <tr><td style="width:120px;color:var(--text-secondary);">Name</td><td>${Components.escape(reader.name)}</td></tr>
            <tr><td style="color:var(--text-secondary);">Broker</td><td>${Components.escape(reader.broker)}</td></tr>
            <tr><td style="color:var(--text-secondary);">Description</td><td>${Components.escape(reader.description || '—')}</td></tr>
          </table></div>`;
      }
      if (!sensors || sensors.length === 0) {
        html += '<div class="card"><p style="color:var(--text-secondary);text-align:center;padding:24px;">No sensors yet. Click + Add Sensor.</p></div>';
      } else {
        html += Components.table(sensors, [
          { key: 'name', label: 'Sensor Name' },
          { key: 'topic', label: 'MQTT Topic' },
          { key: 'measurement', label: 'Measurement' },
          { key: 'fields', label: 'Fields', render: v => Array.isArray(v) ? v.length : (v ? 1 : 0) },
          { key: 'csv', label: 'Type', render: v => v ? Components.badge('CSV','yellow') : Components.badge('JSON','blue') },
          { key: 'tags', label: 'Tags', render: v => {
            if (!v || Object.keys(v).length === 0) return '—';
            return Object.entries(v).map(([k,val]) => `<span style="font-size:11px;color:var(--text-secondary);">${Components.escape(k)}=${Components.escape(val)}</span>`).join(', ');
          }},
          { key: 'name', label: 'Actions', render: (_, row) => `
            <button class="btn btn-sm" onclick="SensorsPage.showSensorModal('${Components.escape(row.name || '')}')">✏️</button>
            <button class="btn btn-sm" onclick="SensorsPage.deleteSensor('${Components.escape(row.name || '')}')" style="margin-left:4px;">🗑️</button>
          `},
        ]);
      }
      el.innerHTML = html;
    } catch (err) {
      el.innerHTML = Components.alert('error', `Failed: ${err.message}`);
    }
  },

  async deleteSensor(sensorName) {
    if (!this.selectedReader) return;
    if (!confirm(`Delete sensor "${sensorName}"?`)) return;
    try {
      await API._delete(`/api/readers/${encodeURIComponent(this.selectedReader)}/sensors/${encodeURIComponent(sensorName)}`);
      await this.loadSensors();
    } catch (err) {
      alert(`Failed to delete: ${err.message}`);
    }
  },

  // ── Sensor Modal (dual-purpose: add + edit) ────────────────────────────

  showSensorModal(sensorName) {
    if (!this.selectedReader) return;
    this.editingSensorName = sensorName || null;
    const isEdit = !!sensorName;

    // Default values
    let sName = '', sTopic = '', sMeasurement = '', sTags = '';
    let sFields = [{ key: '', path: '', type: 'float', unit: '' }];

    // If editing, show modal with loading state then fill real data
    if (isEdit) {
      this._renderModal('Loading...', '', '', '', [], true);
      API.listSensors(this.selectedReader).then(sensors => {
        const sensor = (sensors || []).find(s => s.name === sensorName);
        if (sensor) {
          this._renderModal(sensor.name, sensor.topic, sensor.measurement,
            (sensor.tags && typeof sensor.tags === 'object') ? Object.entries(sensor.tags).map(([k,v]) => `${k}=${v}`).join(', ') : '',
            (sensor.fields && Array.isArray(sensor.fields)) ? sensor.fields : [{ key: '', path: '', type: 'float', unit: '' }],
            true);
        }
      });
    } else {
      this._renderModal(sName, sTopic, sMeasurement, sTags, sFields, false);
    }
  },

  _renderModal(sName, sTopic, sMeasurement, sTags, sFields, isEdit) {
    // Remove existing modal if any
    Components.closeModal();

    let fieldsHTML = '';
    (sFields || [{ key: '', path: '', type: 'float', unit: '' }]).forEach((f, i) => {
      fieldsHTML += `
        <div class="field-row" style="display:flex;gap:8px;margin-bottom:8px;">
          <input name="field_key_${i}" placeholder="Field key" value="${Components.escape(f.key || '')}" style="flex:1;" />
          <input name="field_path_${i}" placeholder="JSON path" value="${Components.escape(f.json_path || f.path || '')}" style="flex:1;" />
          <select name="field_type_${i}" style="width:100px;">
            <option value="float" ${(f.type || 'float') === 'float' ? 'selected' : ''}>float</option>
            <option value="int" ${f.type === 'int' ? 'selected' : ''}>int</option>
            <option value="string" ${f.type === 'string' ? 'selected' : ''}>string</option>
          </select>
          <input name="field_unit_${i}" placeholder="Unit" value="${Components.escape(f.unit || '')}" style="width:80px;" />
        </div>`;
    });

    const content = `
      <p style="color:var(--text-secondary);margin-bottom:16px;">
        ${isEdit ? `Edit sensor in reader <strong>${Components.escape(this.selectedReader)}</strong>.` : `Add a new sensor to reader <strong>${Components.escape(this.selectedReader)}</strong>.`}
      </p>
      <form id="sensor-form" onsubmit="return false;">
        ${Components.field('Sensor Name', 'name', { required: true, value: sName, placeholder: 'e.g. Server Room Temperature' })}
        ${Components.field('MQTT Topic', 'topic', { required: true, value: sTopic, placeholder: 'e.g. sensors/server-room/temperature', help: 'Supports MQTT wildcards + and #' })}
        ${Components.field('Measurement', 'measurement', { required: true, value: sMeasurement, placeholder: 'e.g. environment' })}
        <div style="border:1px solid var(--border);border-radius:6px;padding:12px;margin-bottom:12px;">
          <h4 style="margin-bottom:8px;font-size:13px;">Field Mappings</h4>
          <div id="field-mappings">${fieldsHTML}</div>
          <button type="button" class="btn btn-sm" onclick="SensorsPage.addFieldRow()">+ Add Field</button>
        </div>
        ${Components.field('Tags (key=value, comma separated)', 'tags_str', { value: sTags, placeholder: 'location=server-room,building=main', help: 'Optional static tags' })}
      </form>
    `;

    const actions = `
      <button class="btn" onclick="Components.closeModal()">Cancel</button>
      <button class="btn btn-primary" onclick="SensorsPage.submitSensor()">${isEdit ? 'Update Sensor' : 'Add Sensor'}</button>
    `;

    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.innerHTML = `<div class="modal"><h3>${isEdit ? 'Edit' : 'Add'} Sensor</h3>${content}<div class="form-actions" style="margin-top:16px;">${actions}</div></div>`;
    overlay.addEventListener('click', (e) => { if (e.target === overlay) Components.closeModal(); });
    document.body.appendChild(overlay);

    this.fieldCount = (sFields || []).length || 1;
  },

  fieldCount: 1,

  addFieldRow() {
    const container = document.getElementById('field-mappings');
    if (!container) return;
    const idx = this.fieldCount++;
    const row = document.createElement('div');
    row.className = 'field-row';
    row.style.cssText = 'display:flex;gap:8px;margin-bottom:8px;';
    row.innerHTML = `
      <input name="field_key_${idx}" placeholder="Field key" style="flex:1;" />
      <input name="field_path_${idx}" placeholder="JSON path" style="flex:1;" />
      <select name="field_type_${idx}" style="width:100px;">
        <option value="float">float</option><option value="int">int</option><option value="string">string</option>
      </select>
      <input name="field_unit_${idx}" placeholder="Unit" style="width:80px;" />
    `;
    container.appendChild(row);
  },

  async submitSensor() {
    const form = document.getElementById('sensor-form');
    if (!form) return;

    const data = Components.getFormData(form);
    const fields = [];
    for (let i = 0; i < 20; i++) {
      const key = (data[`field_key_${i}`] || '').trim();
      if (key) {
        fields.push({
          key,
          json_path: (data[`field_path_${i}`] || '.').trim() || '.',
          type: data[`field_type_${i}`] || 'float',
          unit: (data[`field_unit_${i}`] || '').trim(),
        });
      }
    }
    const tags = {};
    if (data.tags_str) {
      data.tags_str.split(',').forEach(pair => {
        const [k, v] = pair.split('=').map(s => s.trim());
        if (k && v) tags[k] = v;
      });
    }

    try {
      // If editing, delete old sensor first
      if (this.editingSensorName) {
        await API._delete(`/api/readers/${encodeURIComponent(this.selectedReader)}/sensors/${encodeURIComponent(this.editingSensorName)}`);
      }
      await API.addSensor(this.selectedReader, {
        name: data.name,
        topic: data.topic,
        measurement: data.measurement,
        fields,
        tags,
      });
      Components.closeModal();
      this.editingSensorName = null;
      await this.loadSensors();
    } catch (err) {
      alert(`Failed: ${err.message}`);
    }
  },
};

window.SensorsPage = SensorsPage;
export default SensorsPage;
