# Frontend — Agent Context

Next.js frontend for Project Phoenix. Read the root `CLAUDE.md` first — it owns the multi-tenancy model, the three-portals table, and cross-stack rules. This file covers frontend-specific knowledge only.

**Stack:** Next.js 16+ (App Router), React 19+, TypeScript (strict), Tailwind CSS 4+, NextAuth v5 (beta), Zod env validation, Axios + SWR, Vitest + Testing Library, Playwright (E2E + guide PDFs), pnpm.

## Development Commands

```bash
pnpm run dev             # Dev server (http://localhost:3000) — prefer docker compose for full-stack work
pnpm run check           # verify-locales + oxlint + tsc — MUST pass before committing (zero warnings)
pnpm run lint:fix        # Auto-fix lint issues
pnpm run test            # Vitest (test:run for CI mode)
pnpm run format:write    # Prettier
pnpm run generate:guides # Render /help guides to PDF (Playwright)
pnpm run knip            # Detect unused dependencies/exports
```

## Environment & API URLs — Fail Fast, No Defaults

Core env vars are **required with no fallbacks** (root `CLAUDE.md` "No Fallbacks" rule; schemas in `src/env.js` + `src/lib/env-validation.js` crash the build when one is missing — only `NODE_ENV`/`NEXT_PUBLIC_LOG_LEVEL` have defaults, and PostHog/Sentry vars are optional). Copy `.env.example` to `.env.local`.

| Variable | Scope | Purpose |
|----------|-------|---------|
| `NEXT_PUBLIC_API_URL` | Client + Server | Browser-accessible backend URL (axios `baseURL`) |
| `API_URL` | Server only | Backend URL for route handlers (Docker: `http://server:8080`) |
| `NEXTAUTH_URL` / `NEXTAUTH_SECRET` | Server | NextAuth base URL + JWT secret |
| `SKIP_ENV_VALIDATION` | Build | `true` skips env validation (Docker builds) |

**`getServerApiUrl()`** (`lib/server-api-url.ts`) returns `env.API_URL` — no fallback chain. Route handlers must use it; never `NEXT_PUBLIC_API_URL` on the server.

### Server-Only Import Isolation (`.server.ts`)

Server-only modules carry a `.server.ts` suffix (`route-wrapper.server.ts`, `api-helpers.server.ts`, `operator/route-wrapper.server.ts`, `parent/route-wrapper.server.ts`). Code that could land in a client bundle must import server-only helpers **dynamically**:

```typescript
// CORRECT — keeps server env out of the client bundle
const { getServerApiUrl } = await import("~/lib/server-api-url");

// WRONG — static import pulls server env into the client bundle
import { getServerApiUrl } from "~/lib/server-api-url";
```

The same applies to `auth` and any other server-only import in mixed files.

## Multi-Tenancy & Routing

The proxy (`src/proxy.ts`) routes by hostname: operator host → `/operator/*`, parents host → `/parents/*`, `{slug}.TENANT_DOMAIN` → `/[tenant]/*`; reserved slugs (`src/lib/reserved-slugs.ts`) are blocked. Cross-host paths redirect (307) back to their canonical subdomain (defense-in-depth on top of cookie isolation). Portal/session/cookie details: root `CLAUDE.md` "Three Portals".

### App Directory Structure

```
src/app/
├── [tenant]/              # Tenant app (slug resolved by proxy)
│   ├── layout.tsx         # Validates slug via /auth/tenant/resolve, wraps in TenantProvider
│   ├── (protected)/       # Auth-required (dashboard, students, rooms, invitations, settings, …)
│   └── (public)/          # Pre-auth (invite, enroll, reset-password)
├── operator/              # Operator portal: provisioning, schools, organizations, accounts,
│                          #   devices, announcements, suggestions, settings, invite, login, …
├── parents/               # Parents portal: children/[id], enroll/[tenantSlug]/[phaseId],
│                          #   accept-guardian-invite/[token], login, …
├── help/                  # Public help guides (see .claude/rules/help-guide-sync.md)
├── invite/, reset-password/  # Root-level public token flows
└── api/                   # Route handlers (proxy to Go backend)
```

### Tenant Context & Navigation

- **`TenantProvider`** (`lib/tenant-context.tsx`): context holding `tenantSlug` + resolved tenant metadata
- **`useTenant()`** throws outside provider; **`useTenantSlugSafe()`** returns `null` — use for SWR cache key prefixing
- **`useTenantRouter()`** (`lib/tenant-router.ts`): subdomain-vs-path-aware navigation
- **`TenantGuard`** (`components/tenant/tenant-guard.tsx`): detects session/URL tenant mismatch (multi-tab) and auto-switches
- **`BinaryModeGuard` / `NfcModeGuard`** (`components/tenant/`): gate routes on the `presence_mode` / NFC tenant settings
- **Error contract**: backend string `"account does not have access to this tenant"` is hardcoded in `lib/tenant-api.ts` — changing the backend string breaks tenant switching silently

## UI: Reuse the Kit (MANDATORY)

Build all new UI from `src/components/ui/` (and `ui/page-header/`); never hand-roll buttons/cards/modals/tables. Brand colors come only from `LOCATION_COLORS` (`src/lib/location-helper.ts`) via arbitrary-value hex (`bg-[#83CD2D]`) — never generic Tailwind hues (`bg-green-500`), which are different colors. Component map, hex table, radius/spacing tokens, gotchas, and the design checklist: **`.claude/rules/frontend-ui-kit.md`**. Search `src/components/` and `src/lib/` for existing code before writing anything new.

### Before/After Screenshots for UI Changes (MANDATORY)

When a change alters what a school user sees (new screen, migrated component, layout/styling change), produce paired before/after screenshots via the `ui-before-after` skill (`.claude/skills/ui-before-after/`): capture the identical interactions against the base ref and your branch, composite them into `pair-*.png` images, and at the end tell the user the local file paths so they can attach the images to the PR manually (GitHub has no API for native attachment uploads; never host screenshots via releases, tags, or Gists — see the PR-screenshots rule in the root `CLAUDE.md`). Backend-only changes and pure refactors with zero visual delta are exempt, but a consolidation refactor that claims "no visual change" should prove it with a pair.

## Architecture Patterns

### Route Handlers (Next.js 16)

All `app/api/` handlers proxy to the Go backend through `route-wrapper.server.ts` for consistent auth + error handling (operator/parent portals have their own wrappers under `lib/operator/` and `lib/parent/`):

```typescript
// app/api/{resource}/route.ts
export const GET = createGetHandler(async (request, token, params) => {
  const response = await apiGet(`/api/resources`, token);
  return response.data;
});
```

Context params are async in Next.js 16: `params: Promise<...>` — always `await`.

### API Clients & Data Mapping

- `lib/{domain}-api.ts` — backend calls; `lib/{domain}-helpers.ts` — type mapping
- Backend `int64` IDs → frontend `string` (`data.id.toString()`); `snake_case` → `camelCase`
- Paginated lists arrive as `{ status, data, pagination: { current_page, page_size, total_pages, total_records } }` (`PaginatedResponse<T>` in `lib/api.ts`)

```typescript
export function mapResourceResponse(data: BackendResource): Resource {
  return {
    id: data.id.toString(),
    createdAt: new Date(data.created_at),
    teacher: data.teacher ? mapTeacherResponse(data.teacher) : undefined,
  };
}
```

### Auth Token Flow

Login (per portal — see root `CLAUDE.md`) returns access (15min) + refresh tokens; NextAuth stores them in the session; route handlers extract the token and forward it as `Authorization: Bearer`; refresh happens automatically on expiry. MFA can insert a challenge step between credentials and session. Invitation/password-reset token flows live at `app/invite/`, `app/[tenant]/(public)/invite/`, `app/reset-password/`, with API clients in `lib/invitation-api.ts` / `lib/invitation-helpers.ts` — use these instead of hitting backend routes directly.

## Date Handling (MANDATORY)

Calendar dates travel as `"YYYY-MM-DD"` strings; never derive one via `.toISOString()` (UTC shifts a day around Berlin midnight — the custom oxlint rule `date-safety/no-utc-date-extraction` rejects it). Use `toISODate` / `todayISO` / `parseISODate` / `formatDate` from `~/lib/date-helpers`. Full table: `.claude/rules/calendar-dates.md`.

## Logging (MANDATORY)

Use `createLogger` from `~/lib/logger` — never bare `console.*`. Snake_case event names, extract `error.message` (never pass raw Error objects). Detailed patterns: the `frontend-structured-logging` skill.

```typescript
const logger = createLogger({ component: "MyComponentName" });
logger.error("profile_save_failed", { error: err instanceof Error ? err.message : String(err) });
```

Only three files may use raw `console.*`: `src/lib/logger.ts`, `src/test/setup.ts`, `src/app/api/logs/route.ts`. The logger is globally mocked in tests (passes through to `console.*`, so tests spy on `console.error`).

## Real-Time Updates (SSE)

```typescript
const { status, isConnected, error, reconnectAttempts } = useSSE("/api/sse/events", {
  onMessage: handleSSEEvent,          // events are TRIGGERS, not payloads — refetch on receipt
  reconnectInterval: 1000,            // exponential backoff 1s→16s, max 5 attempts (defaults)
  maxReconnectAttempts: 5,
});
```

- Hook: `~/lib/hooks/use-sse` (`status`: `idle | connected | reconnecting | failed`); event types in `lib/sse-types.ts` mirror `backend/realtime/events.go` — keep in sync
- Proxy route `app/api/sse/events/route.ts` bypasses the route wrapper (streaming) with `export const runtime = "nodejs"`, injecting the JWT server-side (EventSource can't set headers)
- After an event for the current group, refetch in bulk: `GET /api/active/groups/{id}/visits/display` (O(1), not per-student)
- Connection drops usually mean an expired JWT (15min) or no supervised active groups

## TypeScript & Linting

- `tsconfig`: `strict`, `noUncheckedIndexedAccess`, paths `~/*` and `@/*` → `./src/*`, target ES2022
- Linting: **oxlint** (`.oxlintrc.json` — plugins react/nextjs/jsx-a11y/import/promise; correctness+perf = error). Disabled rules and their rationale live in `.oxlintrc.json`; a custom `date-safety` plugin lives in `scripts/oxlint-plugin-date-safety.mjs`
- Conventions: `??` over `||` for defaults, `import type` for types, `_` prefix for unused vars, `useSearchParams` needs a Suspense boundary, only server components may be async

@AGENTS.md
