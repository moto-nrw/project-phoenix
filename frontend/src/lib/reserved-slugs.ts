/**
 * Slugs and subdomains reserved for infrastructure — never valid as tenant identifiers.
 *
 * This is the single source of truth for the frontend. Used by:
 * - proxy.ts (blocks reserved subdomains at request level)
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
  "parents", // parents.moto-app.de — legacy guardian portal redirect
  "eltern", // eltern.moto-app.de — guardian portal (cross-tenant)
  "schule", // schule.moto-app.de — school portal "moto schule" (#2207)
  "school", // /school path namespace inside the App Router
  "grafana", // grafana.moto-app.de monitoring
  "pyreportal", // pyreportal.moto-app.de kiosk SPA
  "help", // public /help docs — top-level app route shadows [tenant]
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
