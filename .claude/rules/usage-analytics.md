---
paths:
  - "frontend/src/app/**/page.tsx"
  - "frontend/src/lib/analytics*"
  - "frontend/src/lib/posthog-client*"
  - "frontend/src/components/analytics/**"
---

# Usage analytics (Nutzungsanalyse)

Every page and every portal belongs to the usage analytics from its first
commit. Terms: `CONTEXT.md`, section „Nutzungsanalyse"; spec #3598.

## Where the rules live

| Concern | Single source of truth |
|---|---|
| Init options and `before_send` filter for every context | `frontend/src/lib/analytics-policy.ts` |
| Route templates per portal | `frontend/src/lib/analytics-routes.ts` |
| SDK loading, buffering, current context | `frontend/src/lib/posthog-client.ts` |
| Portal login/logout registration | `TenantAuthWrapper` (OGS), `PortalAnalyticsSession` (parents, school) |
| Same-origin `/ingest` proxy to PostHog EU | `frontend/src/proxy.ts` |

The floor for real schools: route templates instead of URLs, the deployment
instead of the real host (the OGS portal runs on `{slug}.TENANT_DOMAIN`), no
element text, no person profile, no IP. A privacy rule belongs in
`analytics-policy.ts` and its table test, never in a component.

## New page

1. Add the page's route template to the portal's list in
   `analytics-routes.ts`, dynamic segments as `:param`.
2. Run `pnpm exec vitest run src/lib/analytics-routes.test.ts`. The page guard
   goes red for any `page.tsx` without a template; green means the page is
   sent as its template, never as its raw URL.

## New surface (portal)

1. Add the surface to `ANALYTICS_SURFACES` and decide its tier in
   `analytics-policy.ts`. Until then it runs on the strictest tier, the rules
   of the parents portal.
2. Give it a route list and a page guard entry in `analytics-routes.ts` and
   `analytics-routes.test.ts`.
3. Register its context at login and clear it at logout
   (`registerPortalSession` / `clearPortalSession` in `lib/analytics.ts`).
4. Add the context to the table in `analytics-policy.test.ts`.

## New custom event

Name it in `CUSTOM_EVENTS` and give each property an allowlisted value in
`analytics-policy.ts`; the filter drops everything else. Core actions that end
in a successful write come from the backend (#3602).
