---
paths:
  - "frontend/src/app/**/page.tsx"
  - "frontend/src/lib/analytics*"
  - "frontend/src/lib/posthog-client*"
  - "frontend/src/components/analytics/**"
  - "backend/analytics/**"
  - "backend/api/base.go"
  - "backend/api/testdata/route_table.golden"
  - "scripts/posthog-dashboards*"
  - ".github/workflows/posthog-dashboards.yml"
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
| School collection switch (`analytics.enabled`, shared, default on) and recording Freigabe (operator-only) | `backend/modules/settings/internal/defaults/analytics.go`; reaches the OGS portal through tenant resolve |
| Person's consent (§ 25 TDDDG): store, prompt after login, revocation card | `frontend/src/lib/analytics-consent.ts`, `components/analytics/analytics-consent-prompt.tsx`, `components/settings/analytics-consent-section.tsx` |
| Consent per account (`auth.accounts.analytics_consent`), `GET`/`PUT /analytics-consent/` | `identityaccess.AccountAnalyticsConsent`, `modules/identityaccess/inbound/me/analytics_consent.go` |
| Pseudonymous user ID (same hash on both sides) | `frontend/src/lib/analytics-pseudonym.ts`, `backend/analytics/pseudonym.go` |
| PostHog project settings, privacy text draft | `docs/operations/nutzungsanalyse.md` |
| The PostHog dashboards, their insights, and the feature areas (`FEATURES`) | `scripts/posthog-dashboards.mjs` |
| Deploying the dashboards to PostHog (preview on PRs, sync on merge) | `.github/workflows/posthog-dashboards.yml` |

The floor for real schools: route templates instead of URLs, the deployment
instead of the real host (the OGS portal runs on `{slug}.TENANT_DOMAIN`), no
element text, no person profile, no IP. A privacy rule belongs in
`analytics-policy.ts` and its table test, never in a component.

Collection needs two gates: the school's switch and the signed-in person's
**consent** (#4901). The SDK touches the device even without cookies, so no
school decision replaces the consent; `analyticsCollectionAllowed` requires
both outside the demo. The consent belongs to the account and holds in every
portal and school. Undecided, refused, failed reads, previews and anonymous
pages (login, public enrollment, status pages) load no SDK and send no
`X-POSTHOG-SESSION-ID`; a revocation stops capture, recording and the header
before it is saved, a consent applies only once stored. The prompt asks only
undecided accounts of a school that allows collection, on deployments with a
PostHog key; the demo is exempt. Backend core actions of an account without
consent keep counting for the school but carry no `$session_id` and no
pseudonymous person (`CoreActionConfig.Consent`). Every gate below comes on
top of the consent.

`analytics.enabled` is the school-controlled collection gate (default `true`,
`config:manage`; operators may also change it). Resolve errors deny collection,
never fall back to an environment value. The OGS tenant resolve carries the
switch, including on public enrollment pages. School sessions resolve their
school; parents collect only if **every** active guardian school permits it.
Until that decision arrives, neither portal loads the SDK. Anonymous parents
and school login pages stay off because their schools are not yet known.
The host owns the decision: `posthog-client.ts` accepts session updates only
from that host's surface. On OGS, only the `TenantProvider` matching the host
slug may permit collection. Every mounted school provider is also a separate
restrictive gate: embedded enrollment/edit/adjust targets must allow collection
in addition to the host or portal decision, never instead of it. Unknown or
refreshing target policies deny collection; unmount removes only that gate.
Keep enrollment target gates mounted during loading and errors, including
Suspense fallback on the parent enrollment form, not just around the loaded form.
Parents and school sessions refresh on `visibilitychange` and every five
minutes while visible, disabling collection before reading. Errors deny; stale
responses cannot restore permission. Anonymous parents enrollment pages stay
off. Outside the demo, token status pages on parents and OGS hosts never
collect, including for
authenticated parents, allowed schools and the redirect after submission:
their contract cannot establish the target school policy. Mount the unresolved
school gate outside Suspense until the status route unmounts.
The `public` surface (hosts outside operator/parents/school/tenant)
never collects outside the demo, an intentional behavior change since #4757.
Parent `enrollment_submitted` additionally resolves and checks its target school,
even if every existing membership school permits analytics.
The demo is exempt. Disabling collection also suppresses recording Freigabe,
backend core actions, kiosk presence events and school `$groupidentify`.
`analytics.CollectionPolicy` owns actor/public-school decisions;
`analytics.GateSchoolTracker` gates school-scoped backend captures in both
API and worker roots. Page and writing-route guards still classify code,
independently of the runtime switch. The switch establishes no legal basis;
that review remains open in #3843, legal slice 3843-e.

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
4. Read the account's consent at login and stop it at logout
   (`startAnalyticsConsent` / `stopAnalyticsConsent`), render
   `AnalyticsConsentPrompt`, and give the portal's personal settings an
   `AnalyticsConsentSection`.
5. Add the context to the table in `analytics-policy.test.ts`.

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

## New dashboard or insight

The PostHog dashboards exist only as `scripts/posthog-dashboards.mjs`; an
insight built by hand in the PostHog UI is not reproducible. The workflow
`posthog-dashboards.yml` deploys them like migrations: a pull request runs
`--check` and `--dry-run` and shows both in its job summary; a merge into
development syncs and fails unless a second run reports `Everything is
current.` Do not sync from a local checkout; that puts unmerged state live.

1. Add the insight to its dashboard in `dashboards()`, or add a dashboard
   with its insights. Build the query with `hogql()`, `trends()`, or
   `funnel()` and pass `production` or `demo`: they add the deployment
   filter. A HogQL query uses the given condition in every `from events`.
2. Aggregate by role, surface, or school at the finest. Never break down by
   `distinct_id` or a person, and never count people: the usage analytics
   does not rate staff members.
3. Run `node --test scripts/posthog-dashboards.test.mjs`. It fails for an
   insight without a deployment filter, with staging, or with a person.
4. Open the pull request. Its `PostHog dashboards / preview` check fails when PostHog
   rejects a query; the job summary lists what the merge will create or
   update. Objects are matched by exact name: a renamed definition creates a
   new one, so delete the old one in PostHog by hand.

The script reads the route templates from `analytics-routes.ts` and the
browser events from `analytics-policy.ts`; a change to either file runs the
workflow too. Locally, `--check` and `--dry-run` only read and need a key in
`POSTHOG_PERSONAL_API_KEY` (never in the repository).
