/**
 * Slugs and subdomains reserved for infrastructure — never valid as tenant identifiers.
 *
 * This is the single source of truth for the frontend. Used by:
 * - middleware.ts (blocks reserved subdomains at request level)
 * - [tenant]/layout.tsx (blocks reserved slugs at route level)
 *
 * Backend equivalent: backend/models/platform/organization.go (reservedSlugs map).
 * Both lists MUST stay in sync. If you add a slug here, add it in the backend too.
 */
export const RESERVED_SLUGS = new Set([
  // Active infrastructure (Caddy blocks / DNS records)
  "www", // www.moto-app.de redirect
  "api", // api.moto-app.de, api-staging, api-demo
  "operator", // operator dashboard
  "grafana", // grafana.moto-app.de monitoring
  "pyreportal", // pyreportal.moto-app.de kiosk SPA
  // Defensive reservations (common infrastructure subdomains)
  "admin",
  "app",
  "dashboard",
  "analytics",
  "status",
  "mail",
  "staging",
  "demo",
]);
