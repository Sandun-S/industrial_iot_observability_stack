/**
 * IIoT Observability Stack — Reusable UI Components
 */

const Components = {
  /**
   * Render an alert banner.
   * @param {'error'|'success'|'info'} type
   * @param {string} message
   * @returns {string} HTML
   */
  alert(type, message) {
    return `<div class="alert alert-${type}">${this.escape(message)}</div>`;
  },

  /**
   * Render a status dot with label.
   */
  statusDot(status, label) {
    const cls = status === 'ok' || status === 'running' ? 'ok' :
                status === 'error' || status === 'unreachable' ? 'error' : 'warn';
    return `<span class="status-dot ${cls}"></span>${label || status}`;
  },

  /**
   * Render a badge.
   */
  badge(text, color) {
    return `<span class="badge badge-${color || 'blue'}">${this.escape(text)}</span>`;
  },

  /**
   * Render a data table from an array of objects.
   * @param {Array<Object>} rows
   * @param {Array<{key: string, label: string, render?: Function}>} columns
   * @param {Object} options - { emptyMessage: string, className: string }
   */
  table(rows, columns, options = {}) {
    if (!rows || rows.length === 0) {
      return `<div class="card"><p style="color: var(--text-secondary); text-align: center; padding: 24px;">
        ${options.emptyMessage || 'No data found.'}</p></div>`;
    }

    let html = '<div class="card" style="padding: 0; overflow-x: auto;"><table>';
    html += '<thead><tr>';
    for (const col of columns) {
      html += `<th>${this.escape(col.label)}</th>`;
    }
    html += '</tr></thead><tbody>';

    for (const row of rows) {
      html += '<tr>';
      for (const col of columns) {
        const value = row[col.key];
        html += '<td>';
        if (col.render) {
          html += col.render(value, row);
        } else if (value === null || value === undefined) {
          html += '<span style="color: var(--text-secondary);">—</span>';
        } else if (typeof value === 'boolean') {
          html += value ? '✅' : '❌';
        } else {
          html += this.escape(String(value));
        }
        html += '</td>';
      }
      html += '</tr>';
    }

    html += '</tbody></table></div>';
    return html;
  },

  /**
   * Render a modal dialog.
   */
  modal(title, content, actions = '') {
    return `
      <div class="modal-overlay" id="modal-overlay" onclick="if(event.target===this)this.remove()">
        <div class="modal">
          <h3>${this.escape(title)}</h3>
          <div class="modal-body">${content}</div>
          ${actions ? `<div class="form-actions" style="margin-top: 16px;">${actions}</div>` : ''}
        </div>
      </div>
    `;
  },

  closeModal() {
    const overlay = document.querySelector('.modal-overlay');
    if (overlay) overlay.remove();
  },

  /**
   * Render a form field.
   */
  field(label, name, opts = {}) {
    const { type = 'text', value = '', placeholder = '', required = false, rows } = opts;
    const id = `field-${name}`;
    let input;

    if (type === 'textarea') {
      input = `<textarea id="${id}" name="${name}" placeholder="${placeholder}" rows="${rows || 4}"
                ${required ? 'required' : ''}>${this.escape(value)}</textarea>`;
    } else if (type === 'select') {
      input = `<select id="${id}" name="${name}" ${required ? 'required' : ''}>
        ${(opts.options || []).map(o => `<option value="${o.value}" ${o.value === value ? 'selected' : ''}>${o.label}</option>`).join('')}
      </select>`;
    } else {
      input = `<input id="${id}" type="${type}" name="${name}" value="${this.escape(value)}"
                placeholder="${placeholder}" ${required ? 'required' : ''} />`;
    }

    return `
      <div class="form-group">
        <label for="${id}">${label} ${required ? '*' : ''}</label>
        ${input}
        ${opts.help ? `<small style="color: var(--text-secondary); font-size: 11px;">${opts.help}</small>` : ''}
      </div>
    `;
  },

  /**
   * Get form data as an object from a form element.
   */
  getFormData(formEl) {
    const data = {};
    const formData = new FormData(formEl);
    for (const [key, value] of formData.entries()) {
      data[key] = value;
    }
    return data;
  },

  /**
   * Escape HTML entities.
   */
  escape(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  },

  /**
   * Format a date string for display.
   */
  formatDate(dateStr) {
    if (!dateStr) return '—';
    try {
      return new Date(dateStr).toLocaleString();
    } catch {
      return dateStr;
    }
  },

  /**
   * Format an epoch timestamp (ms) for display.
   */
  formatEpoch(ms) {
    if (!ms) return '—';
    try {
      return new Date(ms).toLocaleString();
    } catch {
      return String(ms);
    }
  },

  /**
   * Truncate a string with ellipsis.
   */
  truncate(str, n) {
    if (!str) return '';
    return str.length > n ? str.substring(0, n) + '...' : str;
  },
};

window.Components = Components;
export default Components;
