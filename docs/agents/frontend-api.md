# Frontend API and server boundaries

Read before changing API clients, route handlers, server-only imports, env URL
resolution, or response mapping. Paths beginning with `src/` are relative to
`frontend/`; `lib/`, `app/`, and wrapper filenames start at `frontend/src/`.
`.claude/`, `frontend/`, and `docs/` paths start at the repository root.
Env files are relative to `frontend/`.

## Environment & API URLs — Fail Fast, No Defaults

Required values fail fast through `src/env.js` and `src/lib/env-validation.js`.
Read `.claude/rules/env-docker-sync.md` for the canonical exceptions and change
checklist. Local values belong in `.env.local` using `.env.example` as the template.

| Variable | Scope | Purpose |
|----------|-------|---------|
| `NEXT_PUBLIC_API_URL` | Client + Server | Browser-accessible backend URL (axios `baseURL`) |
| `API_URL` | Server only | Backend URL for route handlers (Docker: `http://server:8080`) |
| `NEXTAUTH_URL` / `NEXTAUTH_SECRET` | Server | NextAuth base URL + JWT secret |
| `SKIP_ENV_VALIDATION` | Build | `true` skips env validation (Docker builds) |

**`getServerApiUrl()`** (`lib/server-api-url.ts`) returns the startup-validated `process.env.API_URL` — no fallback chain. Route handlers must use it; never `NEXT_PUBLIC_API_URL` on the server.

### Server-Only Import Isolation (`.server.ts`)

Server-only wrappers carry a `.server.ts` suffix (`lib/route-wrapper.server.ts`, `lib/api-helpers.server.ts`, `lib/operator/route-wrapper.server.ts`, `lib/parent/route-wrapper.server.ts`).

Read the existing server-only wrappers and keep server auth/env imports out
of client components. A dynamic import is not itself an authorization or
server-only boundary; inspect the caller and execution path.

The same applies to `auth` and any other server-only import in mixed files.

## Architecture Patterns

### Route Handlers (Next.js 16)

Backend-proxy handlers use the affected portal's wrapper for consistent auth
and error handling (operator/parent wrappers live under `lib/operator/` and
`lib/parent/`). Streaming SSE uses its documented exception in
[the SSE contract](realtime.md).

Read [the tenant route wrapper](../../frontend/src/lib/route-wrapper.server.ts)
and the affected portal's wrapper before changing a handler. Existing consumers
such as [the rooms route](../../frontend/src/app/api/rooms/route.ts) show the
request/response wiring; retain the affected endpoint's error contract.

Context params are async in Next.js 16: `params: Promise<...>` — always `await`.

### API Clients & Data Mapping

- `lib/{domain}-api.ts` — backend calls; `lib/{domain}-helpers.ts` — type mapping
- Backend `int64` IDs → frontend `string` (`data.id.toString()`); `snake_case` → `camelCase`
- Paginated lists arrive as `{ status, data, pagination: { current_page, page_size, total_pages, total_records } }` (`PaginatedResponse<T>` in `lib/api.ts`)

Inspect the domain's existing mapper, such as
[room response mapping](../../frontend/src/lib/room-helpers.ts), rather than
copying a generic mapper. Preserve nullability and date-vs-instant semantics.

### Auth Token Flow

Login (per portal — see `docs/agents/contracts.md`) returns access + refresh tokens with configured lifetimes; NextAuth stores them in the session; route handlers extract the token and forward it as `Authorization: Bearer`; refresh happens automatically on expiry. MFA can insert a challenge step between credentials and session. Invitation/password-reset token flows live at `app/invite/`, `app/[tenant]/(public)/invite/`, `app/reset-password/`, with API clients in `lib/invitation-api.ts` / `lib/invitation-helpers.ts` — use these instead of hitting backend routes directly.
