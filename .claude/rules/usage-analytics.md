---
paths:
  - "frontend/src/app/**/page.tsx"
  - "frontend/src/lib/analytics*"
  - "frontend/src/lib/posthog-client*"
  - "frontend/src/components/analytics/**"
  - "backend/analytics/**"
  - "backend/api/base.go"
  - "backend/api/testdata/route_table.golden"
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
| Core actions: writing route → backend event | `backend/analytics/core_actions.go` |
| Backend tracker (batching, `deployment`, `$session_id`) | `backend/analytics/analytics.go` |
| Browser session to the backend (`X-POSTHOG-SESSION-ID`) | `frontend/src/lib/analytics-session-header.server.ts` |
| Analyse-Freigabe settings (`analytics.*`, operator-only) | `backend/services/config/defaults/analytics.go`; reaches the OGS portal through tenant resolve |
| Pseudonymous user ID (same hash on both sides) | `frontend/src/lib/analytics-pseudonym.ts`, `backend/analytics/pseudonym.go` |
| PostHog project settings, privacy text draft | `docs/operations/nutzungsanalyse.md` |

The floor for real schools: route templates instead of URLs, the deployment
instead of the real host (the OGS portal runs on `{slug}.TENANT_DOMAIN`), no
element text, no person profile, no IP. A privacy rule belongs in
`analytics-policy.ts` and its table test, never in a component.

Session recording runs only in the public demo and in the OGS portal of a
school with Analyse-Freigabe; a person (pseudonymous ID) exists only in the
latter. Both are guarded twice: the client starts nothing elsewhere, and the
filter drops `$snapshot` and person events outside those contexts. Never
widen a tier without the table test covering it. An element that must never
appear in any recording carries `data-analytics-block`.

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

## New writing route

Every POST, PUT, PATCH, or DELETE route of the portal routers (`/api`,
`/auth`, `/parent`, `/school`, `/demo`) is classified in `coreActions` in
`backend/analytics/core_actions.go`. The operator dashboard, the kiosk
(`/api/iot`), and CalDAV are not portals and stay out.

1. Add the route with its chi pattern, exactly as `route_table.golden` lists
   it: `event("…")` when a successful write is a core action worth counting,
   otherwise `notCaptured`. A login-like route whose response mints the
   session takes `session("…")`; a public route without a session names its
   surface (`public("…")`).
2. Name a new event in snake_case after what succeeded (`group_created`).
   Properties are fixed: `school_id`, `surface`, `role`, `deployment`,
   `$session_id`, and `export_type` for exports. Never a path ID or a body
   value.
3. Run `go test ./api/ -run TestFullProductionRouterGolden` from `backend/`.
   The route guard (`core action classification`) goes red for an
   unclassified writing route and for a table entry whose route is gone.
4. The Next.js route handler forwards `X-POSTHOG-SESSION-ID`. The shared
   helpers do (`api-helpers.server`, `getClientForwardHeaders`, the portal
   route wrappers, `createFileExportRoute`); a handler with its own `fetch`
   spreads `incomingAnalyticsSessionHeaders()` into its headers.

The event is sent only for a 2xx response, after the tenant transaction
committed. Do not capture the same action in the browser as well.

## New custom event

Name it in `CUSTOM_EVENTS` and give each property an allowlisted value in
`analytics-policy.ts`; the filter drops everything else. A core action that
ends in a successful write is no custom event: it comes from the backend (see
above), and the browser filter drops the backend's event names.
