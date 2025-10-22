# Add Server-Sent Events for Real-Time Updates

## Why

Supervisors currently need to manually refresh pages to see student check-ins/check-outs and activity changes. This creates a poor user experience and delays in responding to student movements. Real-time updates via Server-Sent Events (SSE) will provide instant visibility into student location changes, creating a more responsive and modern interface similar to chat applications.

## What Changes

- **Backend**: Add SSE infrastructure with group-based subscription system
  - **New package `backend/realtime/`** (neutral layer, no circular dependencies)
  - Event hub for managing client connections and broadcasting to groups
  - New `/api/sse/events` HTTP endpoint with JWT authentication
  - **Event broadcasting in `services/active/`** layer (covers ALL entry points: IoT, manual, automated)
  - Auto-discovery: Uses existing `GetStaffActiveSupervisions()` for permission-consistent subscriptions
  - **Performance**: New bulk fetch endpoint `GET /api/active/groups/{id}/visits/display` (O(1) vs O(N))

- **Frontend**: Add SSE client integration
  - **Custom streaming proxy** in `app/api/sse/events/route.ts` (bypasses route-wrapper.ts, uses `runtime='nodejs'`)
  - New `useSSE` React hook with exponential backoff reconnection and cleanup
  - Optimized refetch using bulk endpoint (replaces N+1 query pattern)
  - Update myroom and ogs_groups pages to handle real-time events
  - Connection status indicator showing live update state (green/yellow/red)

- **Data Flow Architecture**: SSE events are notification triggers, not data payloads
  - **SSE Event**: Minimal trigger data (student_id, student_name, active_group_id)
  - **Bulk Refetch**: Full data fetched via `GET /api/active/groups/{id}/visits/display` after event
  - **Rationale**: School class and group name unnecessary in events (implicit context + fetched in bulk)
  - This pattern minimizes SSE bandwidth while maintaining data freshness

- **Security**: GDPR-compliant minimal data transmission
  - Events contain only notification triggers (student ID, name)
  - No sensitive data (birthday, address, guardians, school_class) in event stream
  - Server-side permission checks on connection (GetStaffActiveSupervisions)
  - Automatic disconnect when token expires (15min)

## Impact

- **Affected specs**:
  - New: `real-time-notifications` (SSE infrastructure)
  - Modified: `active-sessions` (event broadcasting)
  - New: `frontend-integration` (React hooks and pages)

- **Affected code**:
  - Backend (new): `realtime/` (events, hub, broadcaster interface)
  - Backend (new): `api/sse/resource.go` (Resource struct with hub + activeSvc dependencies), `api/sse/api.go` (HTTP endpoint)
  - Backend (modified):
    - `services/factory.go` (hub instantiation in Factory.RealtimeHub field)
    - `services/active/active_service.go` (broadcasts in CreateVisit, EndVisit, StartActivitySession, StartActivitySessionWithSupervisors, EndActivitySession, ProcessDueScheduledCheckouts)
    - `api/base.go` (API.SSE field, wiring `sse.NewResource(services.RealtimeHub, services.Active)`)
  - Frontend (new): `lib/hooks/use-sse.ts`, `app/api/sse/events/route.ts`, `lib/sse-types.ts`
  - Frontend (modified): `app/myroom/page.tsx`, `app/ogs_groups/page.tsx` (SSE integration, bulk fetch), `lib/active-api.ts` (bulk endpoint)

- **Breaking changes**: None - pure addition, existing polling/refresh still works

- **Performance**: Minimal overhead (~10KB per connection, <1ms per event broadcast)

- **GDPR compliance**: Events contain only data already visible to supervisors, audit logging for connections
