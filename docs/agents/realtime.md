# Real-time updates (SSE)

Read before changing event producers, event types, SSE connections, or client
refetch behavior. Backend-relative paths below start at `backend/`;
frontend-relative paths start at `frontend/src/`.

## Backend

- **Hub**: `backend/realtime/` (dependency-neutral package, `*slog.Logger` with nil-safe `getLogger()`). Single instance wired in `services.Factory`, injected into the active service (broadcasting) and the SSE API resource (connections).
- **Endpoint**: `/api/sse/events` — JWT-authenticated, auto-subscribes the client to the active groups they supervise, 30s heartbeat.
- **Broadcasting**: services fire events after data changes via `realtime.NewEvent(...)` + `BroadcastToGroup` — fire-and-forget, broadcast errors are logged and never block the operation. Per-client buffers are small and lossy (events drop when a client's channel is full), which is why clients refetch instead of trusting delivery. Broadcast points live in `services/active/` (visits, sessions, attendance).
- **Event types**: authoritative list in `realtime/events.go` (student check-in/out, activity lifecycle, instance lifecycle, dashboard counts, supervision/arrival-schedule/settings changes). Frontend types mirror it in `frontend/src/lib/sse-types.ts` — keep both in sync.
- Events are notification triggers, not payloads — clients refetch via bulk endpoints.

## Frontend

Read the [SSE hook](../../frontend/src/lib/hooks/use-sse.ts) for current
options and retry defaults, and its [tests](../../frontend/src/lib/hooks/use-sse.test.ts)
for reconnect/error behavior. Events are triggers: refetch rather than using
them as data payloads.

- Hook: `~/lib/hooks/use-sse` (`status`: `idle | connected | reconnecting | failed`); event types in `lib/sse-types.ts` mirror `backend/realtime/events.go` — keep in sync
- Proxy route `app/api/sse/events/route.ts` bypasses the route wrapper (streaming) with `export const runtime = "nodejs"`, injecting the JWT server-side (EventSource can't set headers)
- After an event for the current group, refetch in bulk: `GET /api/active/groups/{id}/visits/display` (O(1), not per-student)
- Connection drops usually mean an expired JWT (lifetime configured by `AUTH_JWT_EXPIRY`) or no supervised active groups
