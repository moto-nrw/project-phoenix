# Technical Design: SSE Real-Time Updates

## Context

The system currently uses request-response HTTP for all data fetching. Supervisors must manually refresh or poll to see changes. Modern web applications (Slack, WhatsApp Web, Twitter) provide real-time updates creating better user experience. This design adds Server-Sent Events (SSE) for unidirectional server→client notifications.

**Constraints:**
- Must work with existing JWT authentication (15min expiry)
- Must respect GDPR data minimization
- Must scale to 10-15 concurrent supervisors initially
- Must coexist with existing REST APIs (not replace)

**Stakeholders:**
- Supervisors: Need instant notification of student movements
- Students: Indirectly benefit from faster supervisor response
- System: Needs maintainable, secure solution

## Goals / Non-Goals

**Goals:**
- Real-time notifications for student check-ins/check-outs
- Automatic updates for myroom and ogs_groups pages
- Sub-second latency for event delivery
- Secure, permission-checked event distribution
- Graceful degradation (works if SSE fails)

**Non-Goals:**
- Bidirectional communication (chat) - defer to future WebSocket implementation
- Historical event replay - database is source of truth
- Perfect reliability - occasional missed events acceptable (refetch handles)
- Real-time presence ("who's viewing this page") - not needed yet

## Decisions

### Decision 1: SSE vs WebSockets

**Chosen:** Server-Sent Events (SSE)

**Rationale:**
- One-way communication sufficient (server pushes, client reacts)
- Simpler than WebSockets (no connection management, auto-reconnect)
- Standard HTTP, easier authentication with JWT
- Built into browsers (EventSource API)
- Can add WebSockets later for chat without replacing SSE

**Alternatives considered:**
- **Polling:** Simple but wasteful, delays up to poll interval
- **WebSockets:** More complex, bidirectional features not needed now
- **Long polling:** Awkward connection management, no better than SSE

### Decision 2: Package Structure - Neutral Realtime Package

**Chosen:** Create `backend/realtime/` package for events/hub, NOT in `backend/api/sse/`

**Rationale:**
- Services layer must emit events WITHOUT importing API package (dependency inversion)
- Current architecture: `api` → `services` → `models` → `repositories`
- Placing events in `api` would force `services` → `api` circular dependency
- Neutral package allows: `services` → `realtime` ← `api`

**Package structure:**
```
backend/realtime/
├── events.go       # Event types and structures
├── hub.go          # Connection hub and broadcasting
└── broadcaster.go  # Interface for services to use

backend/api/sse/
└── api.go          # HTTP endpoint only (uses realtime.Hub)

backend/services/active/
└── active_service.go  # Emits events via realtime.Broadcaster
```

### Decision 3: Activity-Centric Subscriptions

**Chosen:** Subscribe by `active_group_id` (activity sessions), not `room_id`

**Rationale:**
- Activities are the core domain entity (rooms are just locations)
- One supervisor may oversee multiple groups in different rooms
- Supports "My Group" page (group-centric view)
- Aligns with existing permission model (supervisors assigned to groups)

**Implementation:**
```
User connects → Extract user_id from JWT
             → Use existing GetStaffActiveSupervisions() service method
             → Subscribe to returned active_group_ids
             → Send events only for subscribed groups
```

**CRITICAL:** Reuse `active.GetStaffActiveSupervisions()` (line 547-566) instead of hand-writing SQL in SSE package to maintain permission consistency.

### Decision 4: SSE as Notification Trigger (Not Data Payload)

**Chosen:** Events contain minimal trigger data only; full data fetched via bulk endpoint

**Rationale:**
- **GDPR compliance:** Minimize sensitive data in transit (event size ~100 bytes)
- **Security:** If SSE stream compromised, attacker sees only trigger notifications
- **Consistency:** Bulk refetch ensures data freshness (database = source of truth)
- **Simplicity:** No state synchronization between event payload and database state
- **Efficiency:** School class and group name implicit in subscription context (already known to supervisor)

**Event structure (minimal trigger):**
```json
{
  "type": "student_checkin",
  "active_group_id": "42",
  "data": {
    "student_id": "123",
    "student_name": "Max Müller"    // Optional display hint
  },
  "timestamp": "2025-01-12T14:30:00Z"
}
```

**NOT included in events:** school_class (fetched in bulk), group_name (implicit in active_group_id), birthday, address, guardians, medical info

**Data flow:**
1. SSE event arrives → triggers refetch
2. Client calls `GET /api/active/groups/{id}/visits/display` (bulk endpoint)
3. Bulk response includes: student_id, student_name, school_class, group_name, check_in_time
4. UI updates with complete, fresh data

### Decision 5: Service Layer Broadcasting (Not IoT Handlers)

**Chosen:** Emit events from `services/active/` layer, NOT from IoT API handlers

**Rationale:**
- Multiple entry points create visits: IoT check-in, manual check-in via webapp, scheduled checkouts, daily cleanup
- Broadcasting only in IoT handlers misses manual and automated flows
- Service layer is single source of truth for visit lifecycle
- Ensures SSE events for ALL visit changes regardless of entry point

**Broadcast points in ActiveService:**
```
CreateVisit()                        → student_checkin event
EndVisit()                           → student_checkout event
StartActivitySession()               → activity_start event (line 1251)
StartActivitySessionWithSupervisors() → activity_start event (line 1374)
EndActivitySession()                 → activity_end event (line 1801)
ProcessDueScheduledCheckouts()       → student_checkout event (line 2314, automated)
```

**Service signature update:**
```go
// In services/active/active_service.go
type Service struct {
    // ... existing fields ...
    broadcaster realtime.Broadcaster  // NEW
}

func NewService(...existing params..., broadcaster realtime.Broadcaster) Service {
    return &service{
        // ... existing fields ...
        broadcaster: broadcaster,
    }
}
```

**Factory update:**
```go
// In services/factory.go
type Factory struct {
    // ... existing service fields ...
    RealtimeHub realtime.Hub  // NEW - Shared hub instance
}

func NewFactory(repos *repositories.Factory, db *bun.DB) (*Factory, error) {
    // Create realtime hub (single instance shared by services AND API)
    realtimeHub := realtime.NewHub()

    // Pass to active service
    activeService := active.NewService(
        ...existing params...,
        realtimeHub,  // NEW - Broadcaster interface
    )

    return &Factory{
        // ... existing service assignments ...
        RealtimeHub: realtimeHub,  // NEW - Expose for API layer
    }
}
```

**API wiring:**
```go
// In api/base.go
type API struct {
    // ... existing domain resources ...
    SSE *sse.Resource  // NEW
}

func NewAPI(services *services.Factory, ...) *API {
    return &API{
        // ... existing resources ...
        SSE: sse.NewResource(
            services.RealtimeHub,  // Hub for broadcasting
            services.Active,        // NEW - For GetStaffActiveSupervisions()
        ),
    }
}

func (a *API) Router() chi.Router {
    // ... existing mounts ...
    r.Mount("/api/sse", a.SSE.Router())  // NEW
}
```

**SSE Resource signature:**
```go
// In api/sse/resource.go
type Resource struct {
    hub        realtime.Hub
    activeSvc  active.Service  // NEW - For querying supervised groups
}

func NewResource(hub realtime.Hub, activeSvc active.Service) *Resource {
    return &Resource{
        hub:       hub,
        activeSvc: activeSvc,
    }
}
```

### Decision 6: Next.js Proxy Pattern (NOT route-wrapper.ts)

**Chosen:** Custom streaming proxy in `app/api/sse/events/route.ts`, bypassing route-wrapper.ts

**Rationale:**
- **route-wrapper.ts incompatible:** Always returns `NextResponse.json()` (buffers response)
- **SSE requires streaming:** Must use `Response` with `ReadableStream` body
- **EventSource header limitation:** Cannot set Authorization header, must inject token server-side
- **Token refresh alignment:** Must coordinate with existing Axios interceptor (frontend/src/lib/api.ts:75-160)

**Implementation:**
```typescript
// frontend/src/app/api/sse/events/route.ts
export const runtime = 'nodejs';  // REQUIRED for streaming

export async function GET(request: NextRequest) {
  const session = await auth();

  if (!session?.user?.token) {
    return new Response('Unauthorized', { status: 401 });
  }

  // Fetch from Go backend with token header
  const backendResponse = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/sse/events`,
    {
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Accept': 'text/event-stream',
      },
    }
  );

  if (!backendResponse.ok) {
    return new Response('SSE connection failed', { status: 502 });
  }

  // Stream backend response to client
  return new Response(backendResponse.body, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive',
    },
  });
}
```

**Token expiry handling:**
- Next.js proxy doesn't retry on 401 (SSE connection closes)
- Frontend `useSSE` hook detects close, triggers reconnection
- New connection gets fresh token from updated session
- Aligns with existing token refresh in Axios interceptor

### Decision 7: Hub-Based Broadcasting with Structured Logging

**Chosen:** Central hub manages all SSE connections and group subscriptions

**Rationale:**
- **Efficiency:** O(1) broadcast to group subscribers
- **Isolation:** Services don't manage connections directly
- **Testability:** Hub can be unit tested independently

**Architecture:**
```go
type Hub struct {
    clients      map[*Client]bool
    groupClients map[string][]*Client  // group_id -> subscribers
    mu           sync.RWMutex           // Guard concurrent access
}

// Services call hub (fire-and-forget)
if err := s.broadcaster.BroadcastToGroup(groupID, event); err != nil {
    logging.Logger.Error().Err(err).Msg("Broadcast failed")
    // Continue execution - SSE failure doesn't break business logic
}
```

**SSE handler pattern:**
```go
func (rs *Resource) eventsHandler(w http.ResponseWriter, r *http.Request) {
    // Check flusher support
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", 500)
        return
    }

    // Set SSE headers...

    // Register client
    client := &Client{Channel: make(chan Event, 10)}
    rs.hub.Register(client, groupIDs)
    defer rs.hub.Unregister(client)

    // Stream events
    for {
        select {
        case <-r.Context().Done():  // Client disconnected (NOT CloseNotifier - deprecated)
            return
        case event := <-client.Channel:
            // Send event and flush
            fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, jsonData)
            flusher.Flush()
        }
    }
}
```

**Logging:**
- **Use `backend/logging.Logger`** for structured logs (NOT ad-hoc `log.Printf` or `fmt.Printf`)
- Connection events: INFO level with user_id, subscribed_groups
- Event broadcasts: DEBUG level with event_type, group_id, recipient_count
- Broadcast errors: ERROR level with error context (fire-and-forget, don't fail service calls)
- Align with existing logging pattern in other services

**Example:**
```go
import "github.com/moto-nrw/project-phoenix/logging"

// Connection established
logging.Logger.Info().
    Int64("user_id", userID).
    Strs("subscribed_groups", groupIDs).
    Msg("SSE connection established")

// Event broadcast
logging.Logger.Debug().
    Str("event_type", event.Type).
    Str("active_group_id", groupID).
    Int("recipient_count", len(clients)).
    Msg("SSE event broadcast")

// Broadcast error (fire-and-forget)
if err := s.broadcaster.BroadcastToGroup(groupID, event); err != nil {
    logging.Logger.Error().
        Err(err).
        Str("event_type", event.Type).
        Msg("SSE broadcast failed, continuing")
    // Don't return error - fire-and-forget!
}
```

### Decision 8: Performance Optimization - Bulk Fetch Endpoint

**Chosen:** Add `GET /api/active/groups/{id}/visits/display` endpoint for batch student data

**Rationale:**
- **Current problem:** myroom page fetches students one-by-one (N+1 query pattern)
  ```typescript
  // Current: O(N) API calls
  const studentPromises = visits.map(visit => fetchStudent(visit.studentId));
  const students = await Promise.all(studentPromises);
  ```
- **SSE events will trigger this repeatedly** (every check-in/out)
- **Heavy load:** 20 students = 20 API calls per event

**New endpoint:**
```go
// GET /api/active/groups/{id}/visits/display
// Returns all active visits with student display data in ONE call
{
  "visits": [
    {
      "visit_id": "123",
      "student_id": "456",
      "student_name": "Max Müller",
      "school_class": "4a",
      "check_in_time": "...",
      "group_name": "Basketball"  // OGS group, not active group
    }
  ]
}
```

**Frontend optimization:**
```typescript
// New: O(1) API call
useSSE('/api/sse/events', {
  onMessage: () => {
    // Single fetch for all student data
    const visits = await activeService.getGroupVisitsWithDisplay(groupId);
    setStudents(visits);
  }
});
```

**Avoids:** Heavy refetch pattern on every SSE event

## Risks / Trade-offs

### Risk: Token Expiration During Active Session

**Impact:** After 15 minutes, SSE connection drops when JWT expires

**Mitigation:**
- Frontend detects disconnect and automatically reconnects
- New connection gets fresh JWT from session
- Exponential backoff prevents thundering herd
- Show "reconnecting..." status to user

### Risk: Event Loss on Network Hiccup

**Impact:** Brief network issues could cause missed events

**Mitigation:**
- Not critical - supervisor can manually refresh
- Database is source of truth, refetch syncs state
- For critical events (emergency alerts), add explicit refetch trigger

### Risk: Connection Limit (HTTP/1.1)

**Impact:** Browsers limit to 6 SSE connections per domain on HTTP/1.1

**Mitigation:**
- Deploy with HTTP/2 (no practical connection limit)
- Current scale (10-15 supervisors) far below limits
- Multiple browser tabs share single connection possible later

### Risk: Data Reload Overhead for Events

**Impact:** Generating rich event payloads requires extra queries:
- `EndVisit()` just updates end_time → must reload visit+student for broadcast
- `StartActivitySession()` creates active.groups → must query room/group metadata
- `EndActivitySession()` updates end_time → must query final session state

**Mitigation:**
- Reload queries are targeted (single visit or group by ID)
- Adds ~5-10ms per event broadcast (acceptable)
- Consider caching student/room data in service layer if becomes bottleneck
- Fire-and-forget pattern ensures reloads don't block critical paths

### Risk: ProcessDueScheduledCheckouts Blocking

**Impact:** Scheduled checkout loop processes many checkouts, broadcast failures could stall automation

**Mitigation:**
- **Fire-and-forget pattern:** Broadcast errors logged but don't halt loop
- Continue processing remaining checkouts even if SSE hub unavailable
- Log errors at ERROR level for monitoring
- Database writes succeed regardless of broadcast outcome

### Risk: Stale Permissions After JWT Issue

**Impact:** Supervisor removed from group still gets events for up to 15min

**Mitigation:**
- Acceptable risk - existing session management has same window
- If critical, add active invalidation (disconnect user's SSE on permission change)
- Short token expiry (15min) limits exposure

## Migration Plan

**Phase 1: Backend Infrastructure (Week 1)**
- Create SSE hub with group subscriptions
- Add `/api/sse/events` endpoint
- No frontend integration yet, test with curl/EventSource console

**Phase 2: Event Broadcasting (Week 1-2)**
- Add broadcasts to check-in service
- Add broadcasts to check-out service
- Add broadcasts to session start/end
- Test with Bruno API + SSE monitoring

**Phase 3: Frontend Integration (Week 2)**
- Create useSSE hook
- Add Next.js API proxy
- Integrate into myroom page
- Add connection status UI

**Phase 4: Testing & Rollout (Week 3)**
- Multi-supervisor testing
- Network failure recovery testing
- Load testing (simulate 20+ connections)
- Gradual rollout to production

**Rollback:**
- Remove SSE integration from frontend (pages work without it)
- Disable SSE endpoint in backend
- No database changes to rollback

## Testing Strategy

**Unit Tests (Required):**
- `backend/realtime/hub_test.go`: Register/unregister clients, broadcast to groups, heartbeat
- `backend/realtime/events_test.go`: Event marshaling, payload structure
- `frontend/lib/hooks/use-sse.test.ts`: Auto-reconnection logic, state transitions, cleanup

**Integration Tests (Required):**
- End-to-end flow: IoT check-in → service broadcast → SSE delivery → frontend receives
- Permission isolation: Supervisor A only receives events for their groups
- Token expiry: Connection closes, reconnection with fresh token succeeds
- Multiple clients: Broadcast reaches all subscribers simultaneously

**Load Testing:**
- 20+ concurrent SSE connections
- 10 events/second broadcast rate
- Memory leak detection (1 hour runtime)

## Open Questions

**Q1:** Should we broadcast ALL events to all supervisors, letting clients filter?
**Decision:** No, use group-based subscriptions for efficiency and security

**Q2:** Should events include full student records (school_class, group_name, etc.)?
**Decision:** No, events are minimal triggers only. Full data fetched via bulk endpoint after event (GDPR minimization + ensures data freshness)

**Q3:** Do we need guaranteed delivery?
**Decision:** No, database is source of truth, refetch handles gaps

**Q4:** Should we log every event delivery for GDPR audit?
**Decision:** Log connections (INFO), event broadcasts (DEBUG) using backend/logging package

**Q5:** How to handle useSSE hook cleanup?
**Decision:** Clear timers on unmount, close EventSource, expose reconnectAttempts state for UI
