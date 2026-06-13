/**
 * IIoT Observability Stack — Application Router & Shell
 */

import API from './api.js';
import Components from './components.js';

// Simple HTML escape (used in template literals)
function escapeHTML(str) {
  if (!str) return '';
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}
import DashboardPage from './pages/dashboard.js';
import ReadersPage from './pages/readers.js';
import SensorsPage from './pages/sensors.js';
import SettingsPage from './pages/settings.js';

const App = {
  currentPage: null,
  pageModules: {
    dashboard: DashboardPage,
    readers: ReadersPage,
    sensors: SensorsPage,
    grafana: null, // loaded inline
    settings: SettingsPage,
  },

  async init() {
    console.log('IIoT Observability Stack — Initializing...');

    // Try to load settings from localStorage
    const savedGrafanaUrl = localStorage.getItem('iiot_grafana_url');
    if (savedGrafanaUrl) {
      // Settings will be loaded by the settings page
    }

    window.addEventListener('hashchange', () => this.route());
    this.route();
  },

  async route() {
    const hash = window.location.hash.replace('#', '') || 'dashboard';
    const page = hash.split('/')[0];

    // Update sidebar active link
    document.querySelectorAll('.nav-link').forEach(link => {
      link.classList.toggle('active', link.dataset.page === page);
    });

    const container = document.getElementById('page-container');
    container.innerHTML = '<div class="loading">Loading...</div>';

    try {
      if (page === 'grafana') {
        this.renderGrafanaPage(container);
      } else if (this.pageModules[page]) {
        await this.pageModules[page].render(container);
      } else {
        container.innerHTML = `<div class="alert alert-error">Page not found: ${page}</div>`;
      }
      this.currentPage = page;
    } catch (err) {
      container.innerHTML = Components.alert('error', `Failed to load page: ${err.message}`);
      console.error(err);
    }
  },

  renderGrafanaPage(container) {
    const grafanaUrl = localStorage.getItem('iiot_grafana_url') || `${window.location.protocol}//${window.location.hostname}:3000`;
    container.innerHTML = `
      <div class="page-header">
        <h1>📈 Grafana Dashboards</h1>
        <div>
          <a href="${grafanaUrl}" target="_blank" class="btn btn-primary">Open Grafana ↗</a>
          <button class="btn" onclick="window.location.reload()">Refresh</button>
        </div>
      </div>
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px;" id="dashboard-list">
        <div class="loading">Loading dashboards...</div>
      </div>
    `;
    this.loadDashboardList(grafanaUrl);
  },

  async loadDashboardList(grafanaUrl) {
    const el = document.getElementById('dashboard-list');
    if (!el) return;
    try {
      const dashboards = await API.listDashboards();
      if (!dashboards || dashboards.length === 0) {
        el.innerHTML = `
          <div class="card" style="text-align: center; grid-column: 1/-1;">
            <p style="font-size: 36px;">📈</p>
            <h3>No Dashboards Yet</h3>
            <p style="color: var(--text-secondary); margin: 8px 0;">
              Open Grafana directly to create dashboards, or use the auto-create feature on the Dashboard page.
            </p>
            <a href="${grafanaUrl}" target="_blank" class="btn btn-primary">Open Grafana ↗</a>
          </div>
        `;
        return;
      }
      el.innerHTML = dashboards.map(d => `
        <div class="card">
          <div class="card-header">${escapeHTML(d.title || 'Untitled')}</div>
          <p style="font-size: 12px; color: var(--text-secondary);">UID: ${escapeHTML(d.uid || '—')}</p>
          <p style="font-size: 12px; color: var(--text-secondary);">Tags: ${(d.tags || []).join(', ') || '—'}</p>
          <a href="${grafanaUrl}/d/${d.uid}" target="_blank" class="btn btn-sm" style="margin-top: 8px;">Open ↗</a>
        </div>
      `).join('');
    } catch (err) {
      el.innerHTML = `
        <div class="card" style="text-align: center; grid-column: 1/-1;">
          <h3>Grafana Connection</h3>
          <p style="color: var(--text-secondary); margin: 8px 0;">
            ${err.message || 'Could not connect to Grafana API'}
          </p>
          <a href="${grafanaUrl}" target="_blank" class="btn btn-primary">Open Grafana Directly ↗</a>
        </div>
      `;
    }
  },

  navigate(page) {
    window.location.hash = `#${page}`;
  },
};

// Boot
document.addEventListener('DOMContentLoaded', () => App.init());
