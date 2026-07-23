import { defineStore } from 'pinia'
import { api } from '../api/client'

// Defaults mirror the backend's own fallback defaults (internal/config),
// used until the GET /api/config fetch resolves (or if it fails entirely,
// e.g. offline) so the app still behaves sensibly without them.
export const useConfigStore = defineStore('config', {
  state: () => ({
    defaultPageSize: 48,
    playerSeekSeconds: 15,
    resumeThresholdSeconds: 5,
    progressReportIntervalSeconds: 10,
    // Defaults open (matches SEAHORSE_DISABLE_REGISTRATION defaulting to
    // false) — if the config fetch fails, err on the side of still showing
    // the option rather than silently hiding a working feature.
    registrationEnabled: true,
  }),
  actions: {
    async fetch() {
      const config = await api.getConfig()
      this.defaultPageSize = config.default_page_size
      this.playerSeekSeconds = config.player_seek_seconds
      this.resumeThresholdSeconds = config.resume_threshold_seconds
      this.progressReportIntervalSeconds = config.progress_report_interval_seconds
      this.registrationEnabled = config.registration_enabled
    },
  },
})
