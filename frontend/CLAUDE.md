# Frontend — Agent Context

Next.js frontend for Project Phoenix. Before changing tenant scoping, portals,
login/MFA, or enrollment, read `docs/agents/contracts.md`. This file covers
frontend-specific knowledge only.

## Commands and verification

Read versions and scripts from `frontend/package.json`.
Run `cd frontend && pnpm run check` with zero warnings and relevant behavior
tests after code changes. Before push, run `scripts/test-changed.sh origin/development`
without `--fast`. Service and screenshot workflows: `docs/agents/operations.md`.

## Server and API boundaries

Server requests use `getServerApiUrl()` / `API_URL`, not the browser URL.
Keep auth and server env imports out of client bundles; required values fail fast.
Before changing clients, handlers, imports, URL resolution, or data mapping,
read [frontend API and server boundaries](../docs/agents/frontend-api.md).

## Multi-Tenancy & Routing

The proxy (`src/proxy.ts`) routes by hostname: operator host → `/operator/*`, parents host → `/parents/*`, school host → `/school/*`, `{slug}.TENANT_DOMAIN` → `/[tenant]/*`; reserved slugs (`src/lib/reserved-slugs.ts`) are blocked. Cross-host paths redirect (307) back to their canonical subdomain (defense-in-depth on top of cookie isolation). Portal/session/cookie details: `docs/agents/contracts.md`.

Inspect `src/app/` for the current page tree instead of copying a cached layout.

### Tenant Context & Navigation

- **`TenantProvider`** (`lib/tenant-context.tsx`): context holding `tenantSlug` + resolved tenant metadata
- **`useTenant()`** throws outside provider; **`useTenantSlugSafe()`** returns `null` — use for SWR cache key prefixing
- **`useTenantRouter()`** (`lib/tenant-router.ts`): subdomain-vs-path-aware navigation
- **`TenantGuard`** (`components/tenant/tenant-guard.tsx`): detects session/URL tenant mismatch (multi-tab) and auto-switches
- **`BinaryModeGuard` / `NfcModeGuard`** (`components/tenant/`): gate routes on the `presence_mode` / NFC tenant settings

## UI: Reuse the Kit (MANDATORY)

Build all new UI from `src/components/ui/` (and `ui/page-header/`); never hand-roll buttons/cards/modals/tables. Brand semantics come from `LOCATION_COLORS` (`src/lib/location-helper.ts`); use kit components or `moto-*` tokens, not generic Tailwind hues or copied hex literals. Component map, hex table, radius/spacing tokens, gotchas, and the design checklist: **`.claude/rules/frontend-ui-kit.md`**. Search `src/components/` and `src/lib/` for existing code before writing anything new.

### Verständlichkeit and help

Before user-visible changes, read `.claude/rules/verstaendlichkeit.md` and run
its checklist against the real screen; record the result in the PR. Load
`moto-einfache-sprache` before writing German copy. For changed tenant flows,
read `.claude/rules/help-guide-sync.md` and update the guide/screenshots in the
same PR when required.

### Before/After Screenshots for UI Changes (MANDATORY)

When a change alters what a school user sees (new screen, migrated component, layout/styling change), produce paired before/after screenshots via the `ui-before-after` skill (`frontend/.claude/skills/ui-before-after/SKILL.md`): capture the identical interactions against the base ref and your branch, composite them into `pair-*.png` images, and at the end tell the user the local file paths so they can attach the images to the PR manually (GitHub has no API for native attachment uploads; never host screenshots via releases, tags, or Gists — see `docs/agents/operations.md` PR screenshots and QA evidence). Backend-only changes and pure refactors with zero visual delta are exempt, but a consolidation refactor that claims "no visual change" should prove it with a pair.

## Data contracts

Backend `int64` IDs become frontend strings; preserve nullable fields and
map backend `snake_case` to frontend `camelCase` in existing domain helpers.
Use the affected portal's route wrapper for authentication and error handling;
SSE streaming is the explicit exception in the SSE contract below.

## Date Handling (MANDATORY)

Calendar dates travel as `"YYYY-MM-DD"` strings; never derive one via `.toISOString()` (UTC shifts a day around Berlin midnight — the custom oxlint rule `date-safety/no-utc-date-extraction` rejects it). Use `toISODate` / `todayISO` / `parseISODate` / `formatDate` from `~/lib/date-helpers`. Full table: `.claude/rules/calendar-dates.md`.

## Logging (MANDATORY)

Use `createLogger` from `~/lib/logger` — never bare `console.*`. Snake_case event names, extract `error.message` (never pass raw Error objects). Detailed patterns: the `frontend-structured-logging` skill.

Only three files may use raw `console.*`: `src/lib/logger.ts`, `src/test/setup.ts`, `src/app/api/logs/route.ts`. The logger is globally mocked in tests (passes through to `console.*`, so tests spy on `console.error`).

## Real-time updates

Events trigger bulk refetches; they are not data payloads. Read
[the SSE contract](../docs/agents/realtime.md) for producer/consumer paths,
retry behavior, and authentication when changing real-time updates.

## TypeScript & Linting

- `tsconfig`: `strict`, `noUncheckedIndexedAccess`, paths `~/*` and `@/*` → `./src/*`, target ES2022
- Linting: **oxlint** (`.oxlintrc.json` — plugins react/nextjs/jsx-a11y/import/promise; correctness+perf = error). Disabled rules and their rationale live in `.oxlintrc.json`; custom plugins live in `scripts/oxlint-plugin-date-safety.mjs` and `scripts/oxlint-plugin-ui-kit.mjs` (UI-kit drift ratchet — five hard-zero rules, see `.claude/rules/frontend-ui-kit.md`)
- Conventions: `??` over `||` for ordinary data defaults (not required infrastructure configuration), `import type` for types, `_` prefix for unused vars, `useSearchParams` needs a Suspense boundary, only server components may be async

## Performance

Keep bundle and render budgets intact. Before changing baselines, always-mounted
components, or performance tooling, read
[frontend performance guardrails](../docs/agents/frontend-performance.md).

@AGENTS.md
