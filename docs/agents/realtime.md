# Real-time updates (SSE)

Read before changing event producers, event types, SSE connections, or client
refetch behavior. Backend-relative paths below start at `backend/`;
frontend-relative paths start at `frontend/src/`.

## Backend

- **Hub**: `backend/realtime/` (dependency-neutral package, `*slog.Logger` with nil-safe `getLogger()`). Single instance built by the process root (`composeProcess` in `backend/api/server.go`) through Delivery's `NewRealtimeHub` (`backend/modules/delivery/compose/realtime.go`), then handed to the broadcasting owners and the SSE HTTP resource (connections).
- **Endpoint**: `/api/sse/events` — JWT-authenticated, auto-subscribes the client to the active groups they supervise, 30s heartbeat.
- **Broadcasting**: services fire events after data changes via `realtime.NewEvent(...)` + `BroadcastToGroup` — fire-and-forget, broadcast errors are logged and never block the operation. Per-client buffers are small and lossy (events drop when a client's channel is full), which is why clients refetch instead of trusting delivery. Broadcast points live in `modules/studentpresence/internal/application/presence/` (visits, sessions, attendance).
- **Event types**: authoritative list in `realtime/events.go` (student check-in/out, activity lifecycle, instance lifecycle, dashboard counts, supervision/arrival-schedule/settings changes). Frontend types mirror it in `frontend/src/lib/sse-types.ts` — keep both in sync.
- Events are notification triggers, not payloads — clients refetch via bulk endpoints.
- **School portal** (moto schule, `/api/school/sse/events`): clients are indexed by account and by school, never by group or tenant fan-out. They get `BroadcastToSchoolAccounts` events, plus `class_day_changed`, which `Hub.BroadcastToTenant` derives from every `student_updated` at the same school (#4792). It carries only the source, never a child; open class views refetch. Every successful morning-attendance save also queues a school-scoped `student_updated` through Delivery's existing after-commit queue; the hub derives `class_day_changed` without child data. This includes corrections to present and works independently of notification settings. The notice `school_unreported_absence` (#4791, „Fehlt ohne Meldung“) separately reaches the Schulverwaltung as a `notification` event; `SchoolRealtimeBridge` also dispatches `class-day-refresh` on it.

## Frontend

Read the [SSE hook](../../frontend/src/lib/hooks/use-sse.ts) for current
options and retry defaults, and its [tests](../../frontend/src/lib/hooks/use-sse.test.ts)
for reconnect/error behavior. Events are triggers: refetch rather than using
them as data payloads.

- Hook: `~/lib/hooks/use-sse` (`status`: `idle | connected | reconnecting | failed`); event types in `lib/sse-types.ts` mirror `backend/realtime/events.go` — keep in sync
- Proxy route `app/api/sse/events/route.ts` bypasses the route wrapper (streaming) with `export const runtime = "nodejs"`, injecting the JWT server-side (EventSource can't set headers)
- After an event for the current group, refetch in bulk: `GET /api/active/groups/{id}/visits/display` (O(1), not per-student)
- Connection drops usually mean an expired JWT (lifetime configured by `AUTH_JWT_EXPIRY`) or no supervised active groups
