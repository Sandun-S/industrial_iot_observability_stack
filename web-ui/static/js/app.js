/**
 * IIoT Observability Stack — Application Router & Shell
 */

import API from './api.js';
import Components from './components.js';
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
          <a href="${grafanaUrl}" target="_blank" class="btn">Open Grafana ↗</a>
          <button class="btn btn-primary" onclick="window.location.reload()">Refresh</button>
        </div>
      </div>
      <div class="card" style="padding: 8px;">
        <div class="grafana-container">
          <iframe src="${grafanaUrl}?orgId=1&kiosk=tv"
                  frameborder="0"
                  allowfullscreen>
          </iframe>
        </div>
      </div>
      <p style="margin-top: 12px; color: var(--text-secondary); font-size: 12px; text-align: center;">
        If the Grafana iframe is blank, open <a href="${grafanaUrl}" target="_blank" style="color: var(--accent);">${grafanaUrl}</a> directly.
        Default login: admin / admin
      </p>
    `;
  },

  navigate(page) {
    window.location.hash = `#${page}`;
  },
};

// Boot
document.addEventListener('DOMContentLoaded', () => App.init());
