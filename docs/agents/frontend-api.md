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

| Variable                           | Scope           | Purpose                                                       |
| ---------------------------------- | --------------- | ------------------------------------------------------------- |
| `NEXT_PUBLIC_API_URL`              | Client + Server | Browser-accessible backend URL (axios `baseURL`)              |
| `API_URL`                          | Server only     | Backend URL for route handlers (Docker: `http://server:8080`) |
| `NEXTAUTH_URL` / `NEXTAUTH_SECRET` | Server          | NextAuth base URL + JWT secret                                |
| `SKIP_ENV_VALIDATION`              | Build           | `true` skips env validation (Docker builds)                   |

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
and error handling (operator/parent/school wrappers live under `lib/operator/`,
`lib/parent/`, and `lib/school/`). Transparent raw-response routes use
`lib/backend-proxy-route.server.ts` for tenant, public, or operator auth plus
fetch/error handling; helper-backed success adapters use its tenant adapter.
Do not move a route across portal auth boundaries to share code. Streaming SSE
uses its documented exception in [the SSE contract](realtime.md).

Backend error responses have one wire contract: the shared route wrappers
(`route-wrapper.server.ts` and the operator, parent, and school wrappers) propagate
`ApiResponseError` through `handleApiError`; raw-response factories and the
audited direct-fetch exceptions use `forwardBackendResponse` or
`backendResponseError`. These paths retain the
backend status, body bytes, `Content-Type`, and `Retry-After`. Do not parse and
rebuild an error body, infer status from an error string, or substitute a
message when the backend returned an empty body. Locally generated errors
(validation, missing session, network failure) are separate from backend
responses.

The following response paths intentionally do not use the ordinary JSON
success wrapper. Their **backend errors still use the shared error path**:

| Route(s)                                                                                                                                                                                                                                                   | Success-path reason                                                                                                         |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `api/sse/events`, `api/parent/sse/events`, `api/school/sse/events`                                                                                                                                                                                         | `proxySSEStream` preserves a live event stream and abort propagation; a buffered JSON wrapper would prevent streaming.      |
| `api/meal-plan/participants/export`, `api/students/export`, `api/statistics/export`, `api/students/day-log/export`, `api/students/[id]/attendance-history/export`                                                                                          | CSV/PDF export bytes and download headers must remain intact.                                                               |
| `api/time-tracking/export`, `api/staff/time-tracking/export`, `api/staff/[id]/time-tracking/export`, `api/birthdays/staff-export`, `api/emergency/snapshot/export`, `api/guardians/payment-overview/export`, `api/operator/billing/key-date-counts/export` | CSV/PDF export bytes and download headers must remain intact; portal auth stays distinct.                                   |
| `api/enrollment/phases/[id]/export`, `api/enrollment/admin/reports/care-usage/export`, `api/enrollment/admin/reports/class-roster/export`, `api/enrollment/admin/students/[studentId]/requests/export`                                                     | Enrollment export formats and filenames come from backend headers.                                                          |
| `api/import/{students,teachers,class-list-entries,opening-balances}/template`                                                                                                                                                                              | Template file bytes and backend filenames cannot be JSON-wrapped.                                                           |
| `api/announcement-attachments/[announcementId]/[attachmentId]/download`, `api/files/folders/[folderId]/files/[fileId]/download`, `api/staff/[id]/documents/[documentId]/download`, `api/students/[id]/documents/[documentId]/download`                     | Attachment bytes, MIME types, and dispositions are passed through.                                                          |
| `api/parent/me/news/[announcementId]/attachments/[attachmentId]/download`                                                                                                                                                                                  | Parent attachment bytes retain disposition and `nosniff` headers; parent relationship authorization remains in the backend. |
| `api/public/enrollment-form-legal-documents/[filename]`, `api/public/enrollment-legal-documents/[filename]`, `api/public/login-image/[filename]`, `api/me/profile/avatar/[filename]`, `api/staff/[id]/avatar`, `api/students/[id]/photo/[filename]`        | Public or user image/PDF responses retain content-type and caching behavior.                                                |
| `api/calendar-feed/[token]`, `api/request-feed/[token]`                                                                                                                                                                                                    | iCalendar/RSS subscriptions retain their wire format and cache policy; the path token is the subscription credential.       |

### Audited direct-fetch exceptions

The API tree has 737 route files. The 56 files below still call `fetch` directly
because they stream non-JSON content, use the shared file-upload wrapper, or
perform a route-specific business/session adaptation. Client-visible backend
errors use `forwardBackendResponse` or `backendResponseError`; the demo
readiness probe and optional profile prefetch are intentionally consumed by
their surrounding BFF flow.
A direct fetch outside this inventory needs a reason and a shared proxy path.

| Route under `app/api/` | Reason the transparent JSON factory is not used |
| --- | --- |
| `activities/[id]/supervisors/[supervisorId]` | Performs session refresh and the documented TOKEN_EXPIRED mapping before forwarding. |
| `announcement-attachments/[announcementId]` | Multipart attachment upload uses the shared file-upload validator. |
| `announcement-attachments/[announcementId]/[attachmentId]/download` | Streams attachment bytes and disposition for a browser download. |
| `auth/login` | Relays login response cookies as well as the backend body. |
| `auth/logout` | Uses the tenant refresh token for revocation; the browser still clears its local session. |
| `birthdays/staff-export` | Streams the staff-birthday export file. |
| `calendar-feed/[token]` | Returns iCalendar text; the path token is the subscription credential. |
| `demo/access/handoff` | Checks readiness and returns a portal redirect, not backend JSON. |
| `demo/access/reset` | Validates a demo credential and maps a reset result to the entry URL. |
| `demo/access/sessions` | Accepts a link or cookie token and stores a new httpOnly demo cookie on success. |
| `demo/access/status` | Uses the shared demo-token forwarding helper for credential-scoped status. |
| `emergency/snapshot/export` | Streams emergency-snapshot export bytes and filename. |
| `enrollment/admin/data-correction` | Maps flattened request/child IDs and a corrected payload to the backend child route. |
| `enrollment/admin/decide` | Maps flattened request/child IDs and decision fields to the backend child route. |
| `enrollment/admin/offerings` | Maps flattened request/child IDs and offering edits to the backend child route. |
| `enrollment/admin/reports/care-usage/export` | Streams the care-usage export with backend filename. |
| `enrollment/admin/reports/class-roster/export` | Streams the class-roster export with backend filename. |
| `enrollment/admin/students/[studentId]/requests/export` | Streams one student’s enrollment requests export. |
| `enrollment/form-bootstrap/public/[tenantSlug]/[phaseId]` | Combines public form metadata with optional session-backed profile prefetch. |
| `enrollment/legal-documents` | Multipart legal-document upload uses the shared file-upload validator. |
| `enrollment/phases/[id]/export` | Streams a phase export in its backend-selected format. |
| `files/folders/[folderId]/files` | Multipart folder-file upload uses the shared file-upload validator. |
| `files/folders/[folderId]/files/[fileId]/download` | Streams folder-file bytes and disposition. |
| `guardian-invitations/[token]/accept` | Maps invitation password fields and forwards analytics-session headers. |
| `guardians/payment-overview/export` | Streams payment-overview export bytes with backend disposition. |
| `import/class-list-entries/template` | Returns the class-list CSV template and backend filename. |
| `import/opening-balances/template` | Returns the opening-balance CSV template and backend filename. |
| `import/students/template` | Returns the student CSV template and backend filename. |
| `import/teachers/template` | Returns the teacher CSV template and backend filename. |
| `invitations/accept` | Maps signup fields and chooses the signed invitation-owner session when an existing account accepts. |
| `me/profile/avatar` | Validates image magic bytes and MIME before upload through the shared upload wrapper. |
| `me/profile/avatar/[filename]` | Serves avatar image bytes with MIME and cache headers. |
| `meal-plan/participants/export` | Streams meal-plan CSV bytes with the backend filename. |
| `operator/billing/key-date-counts/export` | Streams operator billing export bytes with no-store caching and operator refresh. |
| `parent/me/news/[announcementId]/attachments/[attachmentId]/download` | Streams a parent-authorized attachment with disposition and nosniff headers. |
| `public/enrollment-form-legal-documents/[filename]` | Serves a public form PDF as a body stream. |
| `public/enrollment-legal-documents/[filename]` | Serves a public legal PDF with MIME validation and cache policy. |
| `public/login-image/[filename]` | Serves a public login image with MIME validation and cache policy. |
| `request-feed/[token]` | Returns RSS/XML with subscription-token scope and private no-store cache policy. |
| `school/auth/logout` | Uses the school refresh token, not the access token, to revoke the scoped session. |
| `settings/enrollment/legal-agb-document` | Multipart legal PDF upload uses the shared file-upload validator. |
| `settings/login-image` | Uploads validated tenant image data and invalidates the tenant cache. |
| `staff/[id]/avatar` | Returns image bytes, MIME type, and cache headers. |
| `staff/[id]/documents` | Multipart staff-document upload uses the shared file-upload validator. |
| `staff/[id]/documents/[documentId]/download` | Streams one staff document with its MIME type and disposition. |
| `staff/[id]/time-tracking/export` | Streams one staff member’s time-tracking export. |
| `staff/time-tracking/export` | Streams staff time-tracking export bytes. |
| `statistics/export` | Streams statistics export bytes with no-store caching. |
| `students/[id]/attendance-history/export` | Streams attendance-history export bytes with no-store caching. |
| `students/[id]/documents` | Multipart student-document upload uses the shared file-upload validator. |
| `students/[id]/documents/[documentId]/download` | Streams a student document with MIME and disposition. |
| `students/[id]/photo` | Validates photo bytes and consent; DELETE also refreshes an expired session. |
| `students/[id]/photo/[filename]` | Serves student photo bytes with MIME and cache headers. |
| `students/day-log/export` | Streams day-log export bytes with no-store caching. |
| `students/export` | Streams the student export file and backend disposition. |
| `time-tracking/export` | Streams time-tracking export bytes without JSON buffering. |

The three SSE routes (`sse/events`, `parent/sse/events`, and
`school/sse/events`) use `proxySSEStream` separately: each must keep its own
portal credential and abortable event stream, which a JSON factory would buffer.

Helper-backed custom handlers without a direct `fetch` have narrower reasons:
`rooms/[id]/history` applies the documented empty/disabled BFF mapping;
`students/day-log`, `students/[id]/attendance-history`, and
`statistics/report` whitelist audit-sensitive queries, retry refreshed tenant
sessions, and keep their established no-store success envelope;
`auth/password` validates and renames password fields before `apiPost`;
`auth/token` reads the local session; `logs` and `parent/logs` ingest client
telemetry. `enrollment/legal-documents/[filename]` retries deletion with a
refreshed tenant token. `timetable/betreuungsplan/export`,
`timetable/lists/export`, `rooms/export`, `staff-shifts/export`,
`calendar/appointments/[appointmentId]/ics`, and
`parent/calendar/appointments/[appointmentId]/ics` retain downloadable bytes
and headers. The remaining helper-backed routes use their portal's
`create*Handler` or `proxy*` factories, including the operator and parent
wrappers; they are not handwritten auth/fetch/error loops.

Business adapters are not transparent proxies: room history maps the
documented 404 and `feature_disabled` 403 to an empty/disabled view;
student privacy-consent GET maps an absent consent to its default model, and
student PUT marks a consent write that succeeded before a later student write
failed. Demo handoff produces a redirect after checking readiness. Auth
refresh/retry and `TOKEN_EXPIRED` follow the session contract. All other
backend errors in those routes retain the shared error contract. Logout
forwards backend errors unchanged even though the browser always clears its
local session.

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
