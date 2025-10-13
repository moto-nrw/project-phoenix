# Real-Time Updates via Server-Sent Events (SSE)

This package provides the SSE infrastructure for real-time notifications in Project Phoenix. It enables supervisors to receive instant updates about student check-ins/check-outs and activity changes without manual page refreshing.

## Architecture

### Components

1. **Hub (`hub.go`)**: Manages client connections and broadcasts events to subscribed groups
2. **Event (`events.go`)**: Defines event types and data structures
3. **Broadcaster (`broadcaster.go`)**: Interface for services to emit events without tight coupling

### Key Features

- **Group-based subscriptions**: Clients subscribe to specific active groups based on their supervisor permissions
- **Fire-and-forget broadcasting**: Events are sent asynchronously without blocking service operations
- **Thread-safe**: Uses RWMutex for concurrent client management
- **GDPR-compliant**: Events contain only display-level data (no sensitive information)

## Unit Tests

### Backend Tests (`hub_test.go`)

Comprehensive test coverage for the Hub implementation:

```bash
# Run all tests
go test ./realtime

# Run with verbose output
go test -v ./realtime

# Run with race detection
go test -race ./realtime
```

**Test Coverage:**
- ✅ Client registration with single/multiple group subscriptions
- ✅ Client unregistration and cleanup of empty groups
- ✅ Idempotent unregister (handling non-existent clients)
- ✅ Broadcasting to single/multiple subscribers
- ✅ Group isolation (events only go to subscribed groups)
- ✅ Silent broadcasts when no subscribers (no error)
- ✅ Channel full handling (skip instead of block)
- ✅ Client and group subscriber counting

### Frontend Tests (`frontend/src/lib/hooks/__tests__/use-sse.test.ts`)

Comprehensive test coverage for the `useSSE` React hook:

```bash
# Run all frontend tests
cd frontend && npm test

# Run tests in watch mode
npm test

# Run tests with UI
npm run test:ui

# Run tests once (CI mode)
npm run test:run
```

**Test Coverage:**
- ✅ Initial connection establishment
- ✅ Event message handling and parsing
- ✅ Reconnection attempts with exponential backoff
- ✅ Status transitions (idle → connected → reconnecting → failed)
- ✅ Cleanup on unmount (EventSource closed, timers cleared)
- ✅ Error handling and recovery
- ✅ Parse error resilience
- ✅ EventSource browser support detection

## Integration Testing

### Manual Testing Approach

Since SSE requires persistent HTTP connections that Bruno/curl cannot easily simulate, integration testing is best done manually with browser DevTools:

#### Step 1: Start Services

```bash
# Terminal 1: Backend
docker compose up -d postgres
docker compose up server

# Terminal 2: Frontend
cd frontend && npm run dev
```

#### Step 2: Open Browser DevTools

1. Navigate to http://localhost:3000 and log in
2. Open DevTools (F12)
3. Go to **Network** tab
4. Filter by **EventStream** or **SSE** (depending on browser)
5. Navigate to MyRoom or OGS Groups page

#### Step 3: Observe SSE Connection

You should see a connection to `/api/sse/events` with:
- **Status**: `200 OK`
- **Type**: `eventsource` or `text/event-stream`
- **Transfer**: `(pending)` or streaming indicator

#### Step 4: Trigger Events via Check-In

```bash
# Terminal 3: Run Bruno check-in test
cd bruno
bru run --env Local 06-checkins.bru
```

#### Step 5: Verify Event Flow

**In Browser DevTools Network Tab:**
- Click on the `/api/sse/events` connection
- Go to **EventStream** or **Messages** tab
- You should see events appear in real-time:

```json
{
  "type": "student_checkin",
  "active_group_id": "123",
  "data": {
    "student_id": "456",
    "student_name": "Test Student"
  },
  "timestamp": "2025-10-13T18:30:00Z"
}
```

**In Browser Console:**
- Look for `✅ SSE connected` log message
- Events received should trigger refetch logs

**In UI:**
- Student list should update automatically
- Connection status indicator shows green (connected)

### Expected Behavior

| Action | SSE Event | UI Update |
|--------|-----------|-----------|
| Student check-in (IoT) | `student_checkin` | Student appears in room's visit list |
| Student check-out (IoT) | `student_checkout` | Student disappears from visit list |
| Activity session start | `activity_start` | Group appears in active sessions |
| Activity session end | `activity_end` | Group disappears from active sessions |
| Manual check-in (MyRoom) | `student_checkin` | Other supervisors see update |

### Testing Reconnection

1. Stop the backend server (`docker compose stop server`)
2. Observe in DevTools:
   - Connection status changes to **yellow** (reconnecting)
   - Console shows `SSE reconnecting in Xms...` with exponential backoff
3. Restart backend (`docker compose up server`)
4. Observe automatic reconnection:
   - Connection status returns to **green** (connected)
   - Console shows `✅ SSE connected`

### Testing Max Reconnection Attempts

1. Stop backend permanently
2. Wait for ~30 seconds
3. Observe status changes to **red** (failed) after 5 attempts
4. Console shows `SSE: Max reconnection attempts reached`

## Event Broadcasting Integration

### Service Layer Integration

Services broadcast events after data changes:

```go
// In services/active/active_service.go
func (s *service) CreateVisit(ctx context.Context, studentID, roomID int64) (*Visit, error) {
    // ... create visit logic ...

    // Broadcast event (fire-and-forget)
    if s.broadcaster != nil {
        event := realtime.NewEvent(
            realtime.EventStudentCheckIn,
            activeGroupID,
            realtime.EventData{
                StudentID:   &studentIDStr,
                StudentName: &studentName,
            },
        )
        _ = s.broadcaster.BroadcastToGroup(activeGroupID, event)
    }

    return visit, nil
}
```

### Testing Broadcasting

**Check backend logs for broadcast confirmation:**

```bash
docker compose logs -f server | grep SSE
```

Expected log output:
```
INFO SSE client connected user_id=1 subscribed_groups=[123,456] total_clients=1
DEBUG SSE event broadcast active_group_id=123 event_type=student_checkin recipient_count=1 successful=1
INFO SSE client disconnected user_id=1 total_clients=0
```

### Testing No Subscribers (Silent Broadcast)

1. Ensure no active SSE connections (no browser tabs open)
2. Trigger check-in via Bruno
3. Check backend logs:

```
DEBUG No SSE subscribers for group active_group_id=123 event_type=student_checkin
```

Service should complete successfully without errors.

## Performance Monitoring

### Memory Usage

Each SSE connection uses approximately **10KB** of memory:
- Client struct: ~200 bytes
- Channel buffer (10 events): ~2KB
- Event data: ~1KB per event

With 100 concurrent connections: ~1MB total memory overhead

### Latency

Event broadcast latency: **<1ms**
- Hub uses non-blocking channel sends
- Events logged but don't block service execution

### Connection Limits

Default channel buffer: **10 events**
- If client lags and buffer fills, new events are skipped
- Logged as warning: `SSE client channel full, skipping event`
- Client automatically refetches on next event to catch up

## Troubleshooting

### Connection Immediately Closes

**Symptom**: SSE connection opens then immediately closes in DevTools

**Possible Causes:**
1. JWT token expired (15min default)
   - Solution: Refresh page to get new token
2. User not supervisor of any active groups
   - Solution: Verify user has active sessions
3. Backend not running
   - Solution: Check `docker compose ps` and logs

### Events Not Received

**Symptom**: Connection open but no events appearing

**Possible Causes:**
1. User not subscribed to the group where event occurred
   - Solution: Verify user is supervisor of the active group
2. Broadcasting disabled in service
   - Solution: Check `services.RealtimeHub != nil`
3. Event data parse error
   - Solution: Check browser console for parse errors

### Reconnection Loop

**Symptom**: Connection keeps reconnecting every few seconds

**Possible Causes:**
1. Backend rejecting connection (auth issue)
   - Solution: Check backend logs for auth errors
2. Network proxy/firewall blocking EventSource
   - Solution: Test with direct connection (no VPN)

## Security Considerations

### Data Minimization (GDPR)

Events contain only display-level data already visible to supervisors:
- ✅ Student ID and name
- ✅ Activity name
- ✅ Room assignment
- ❌ Birthday, address, guardians (never included)

### Authentication

- JWT token validated on connection
- Token passed via query parameter (no way to set headers in EventSource)
- Automatic disconnect when token expires (15min)

### Authorization

- Permissions checked via `GetStaffActiveSupervisions()` on connection
- Users only receive events for groups they supervise
- Group-based isolation prevents cross-contamination

## Related Files

### Backend
- `backend/realtime/hub.go` - Hub implementation
- `backend/realtime/hub_test.go` - Hub tests
- `backend/realtime/events.go` - Event types
- `backend/api/sse/api.go` - HTTP endpoint
- `backend/services/active/active_service.go` - Event broadcasting

### Frontend
- `frontend/src/lib/hooks/use-sse.ts` - React hook
- `frontend/src/lib/hooks/__tests__/use-sse.test.ts` - Hook tests
- `frontend/src/lib/sse-types.ts` - TypeScript types
- `frontend/src/app/api/sse/events/route.ts` - Streaming proxy
- `frontend/src/app/(auth)/myroom/page.tsx` - MyRoom integration
- `frontend/src/app/(auth)/ogs_groups/page.tsx` - OGS Groups integration
