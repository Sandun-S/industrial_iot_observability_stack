/**
 * IIoT Observability Stack — Readers Page
 * List and manage MQTT reader instances.
 */

import API from '../api.js';
import Components from '../components.js';

const ReadersPage = {
  async render(container) {
    container.innerHTML = `
      <div class="page-header">
        <h1>📡 MQTT Readers</h1>
        <div>
          <button class="btn btn-primary" onclick="ReadersPage.showAddModal()">+ Add Reader</button>
          <button class="btn" onclick="window.location.reload()">Refresh</button>
        </div>
      </div>
      <div id="readers-content"><div class="loading">Loading readers...</div></div>
    `;

    await this.loadReaders();
  },

  async loadReaders() {
    const el = document.getElementById('readers-content');
    if (!el) return;

    try {
      const readers = await API.listReaders();

      if (!readers || readers.length === 0) {
        el.innerHTML = `
          <div class="card" style="text-align: center; padding: 40px;">
            <p style="font-size: 48px; margin-bottom: 16px;">📡</p>
            <h3>No MQTT Readers Configured</h3>
            <p style="color: var(--text-secondary); margin: 8px 0 16px;">
              Add an MQTT reader to start collecting data from your MQTT brokers.
            </p>
            <button class="btn btn-primary" onclick="ReadersPage.showAddModal()">+ Add Your First Reader</button>
          </div>
        `;
        return;
      }

      el.innerHTML = Components.table(readers, [
        { key: 'name', label: 'Name' },
        { key: 'description', label: 'Description', render: v => v || '—' },
        { key: 'broker', label: 'MQTT Broker' },
        { key: 'sensors', label: 'Sensors' },
        { key: 'status', label: 'Status', render: v => Components.statusDot(v, v) },
        {
          key: 'config_file', label: 'Actions',
          render: (_, row) => {
            const name = encodeURIComponent(row.name || row.config_file);
            return `
              <button class="btn btn-sm" onclick="window.location.hash='#sensors/${name}'">🔍 Sensors</button>
              <button class="btn btn-sm" onclick="ReadersPage.deleteReader('${name}')" style="margin-left:4px;">🗑️</button>
            `;
          },
        },
      ]);
    } catch (err) {
      el.innerHTML = Components.alert('error', `Failed to load readers: ${err.message}`);
    }
  },

  showAddModal() {
    const content = `
      <p style="color: var(--text-secondary); margin-bottom: 16px;">
        Create a new MQTT reader configuration. A new Docker container will be deployed
        for this reader to subscribe to the specified MQTT broker and topics.
      </p>
      <form id="add-reader-form">
        ${Components.field('Reader Name', 'name', { required: true, placeholder: 'e.g. temperature-reader' })}
        ${Components.field('Description', 'description', { placeholder: 'e.g. Temperature sensors across the facility' })}
        ${Components.field('MQTT Broker URL', 'broker', { required: true, placeholder: 'tcp://192.168.1.100:1883', help: 'Format: tcp://host:port' })}
        ${Components.field('Client ID', 'client_id', { placeholder: 'iiot-reader-01 (auto-generated if empty)' })}
        ${Components.field('QoS', 'qos', { type: 'select', options: [{value:'0',label:'0 - At most once'},{value:'1',label:'1 - At least once'},{value:'2',label:'2 - Exactly once'}] })}
        ${Components.field('Username', 'username', { placeholder: 'MQTT username (optional)' })}
        ${Components.field('Password', 'password', { type: 'password', placeholder: 'MQTT password (optional)' })}
        ${Components.field('InfluxDB URL', 'influx_url', { placeholder: 'http://influxdb:8086', value: 'http://influxdb:8086' })}
        ${Components.field('InfluxDB Database', 'influx_db', { placeholder: 'iiot', value: 'iiot' })}
        ${Components.field('Sensor Name (initial)', 'sensor_name', { required: true, placeholder: 'e.g. Server Room Temperature' })}
        ${Components.field('MQTT Topic', 'sensor_topic', { required: true, placeholder: 'e.g. sensors/+/temperature', help: 'Supports MQTT wildcards + and #' })}
        ${Components.field('Measurement', 'sensor_measurement', { required: true, placeholder: 'e.g. environment' })}
        ${Components.field('Field Key', 'field_key', { placeholder: 'e.g. temperature_c', value: 'value' })}
        ${Components.field('JSON Path', 'json_path', { placeholder: 'e.g. temperature (or . for full payload)', value: '.' })}
      </form>
    `;

    const actions = `
      <button class="btn" onclick="Components.closeModal()">Cancel</button>
      <button class="btn btn-primary" onclick="ReadersPage.submitAddReader()">Create Reader</button>
    `;

    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.innerHTML = `<div class="modal"><h3>Add MQTT Reader</h3>${content}<div class="form-actions">${actions}</div></div>`;
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
    document.body.appendChild(overlay);
  },

  async deleteReader(name) {
    if (!confirm(`Delete reader "${decodeURIComponent(name)}"? This removes the config file.`)) return;
    try {
      await API._delete(`/api/readers/${name}`);
      await ReadersPage.loadReaders();
    } catch (err) {
      alert(`Failed to delete: ${err.message}`);
    }
  },

  async submitAddReader() {
    const form = document.getElementById('add-reader-form');
    if (!form) return;

    const data = Components.getFormData(form);

    // Build YAML config via API
    // For now, we add the reader with one initial sensor
    // The actual reader config creation would be handled by the backend saving a YAML file
    try {
      const readerName = data.name || data.sensor_name.toLowerCase().replace(/\s+/g, '-');
      const payload = {
        name: data.sensor_name,
        topic: data.sensor_topic,
        measurement: data.sensor_measurement,
        fields: [{
          key: data.field_key || 'value',
          json_path: data.json_path || '.',
          type: 'float',
        }],
        tags: {},
      };

      await API.addSensor(readerName, payload);
      Components.closeModal();
      await ReadersPage.loadReaders();
    } catch (err) {
      alert(`Failed to create reader: ${err.message}`);
    }
  },
};

window.ReadersPage = ReadersPage;
export default ReadersPage;
