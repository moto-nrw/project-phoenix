# Frontend Integration - Real-Time Updates

## ADDED Requirements

### Requirement: useSSE React Hook

The frontend SHALL provide a reusable React hook for SSE connections with auto-reconnection.

#### Scenario: Hook initialization
- **GIVEN** a React component needs real-time updates
- **WHEN** the component calls `useSSE('/api/sse/events', options)`
- **THEN** the hook SHALL establish EventSource connection
- **AND** SHALL return connection state (isConnected, error, reconnectAttempts)
- **AND** SHALL invoke onMessage callback for each event

#### Scenario: Automatic reconnection
- **GIVEN** an active SSE connection via useSSE hook
- **WHEN** connection drops (network error or server restart)
- **THEN** the hook SHALL detect onerror event
- **AND** SHALL attempt reconnection after exponential backoff (1s, 2s, 4s, 8s, 16s)
- **AND** SHALL stop after maxReconnectAttempts (default: 5)
- **AND** SHALL update reconnectAttempts state

#### Scenario: Hook cleanup on unmount
- **GIVEN** a component using useSSE hook
- **WHEN** the component unmounts
- **THEN** the hook SHALL close EventSource connection
- **AND** SHALL clear any pending reconnection timers
- **AND** SHALL prevent memory leaks

### Requirement: SSE Proxy API Route

The frontend SHALL provide Next.js API route that proxies SSE connections to the Go backend.

#### Scenario: SSE connection proxying
- **GIVEN** a browser connects to `/api/sse/events`
- **WHEN** the Next.js API route handler receives the request
- **THEN** it SHALL validate user session using getServerSession
- **AND** SHALL extract JWT token from session
- **AND** SHALL proxy connection to Go backend at `${NEXT_PUBLIC_API_URL}/api/sse/events`
- **AND** SHALL include `Authorization: Bearer ${token}` header
- **AND** SHALL stream events back to browser with correct headers

#### Scenario: Unauthorized proxy request
- **GIVEN** a request to `/api/sse/events` without valid session
- **WHEN** the Next.js API route validates session
- **THEN** it SHALL return HTTP 401 Unauthorized
- **AND** SHALL NOT establish connection to backend

#### Scenario: SSE headers preservation
- **GIVEN** an SSE proxy connection
- **THEN** the response SHALL include:
  - `Content-Type: text/event-stream`
  - `Cache-Control: no-cache`
  - `Connection: keep-alive`
- **AND** SHALL use streaming response (ReadableStream or Response body pipe)

### Requirement: MyRoom Page Real-Time Updates

The myroom page SHALL display real-time student check-ins and check-outs without manual refresh.

#### Scenario: Real-time student check-in display
- **GIVEN** a supervisor viewing myroom page for active group "Basketball"
- **WHEN** a student checks into Basketball group
- **THEN** the page SHALL receive SSE event within 1 second
- **AND** SHALL refetch student list via existing API
- **AND** SHALL update UI to show new student
- **AND** SHALL NOT reload entire page

#### Scenario: Real-time student check-out display
- **GIVEN** a supervisor viewing myroom page with checked-in students
- **WHEN** a student checks out
- **THEN** the page SHALL receive SSE event
- **AND** SHALL remove student from display
- **AND** SHALL update student count badge

#### Scenario: Multiple group updates
- **GIVEN** a supervisor with 3 active groups (Basketball, Chess, Football)
- **WHEN** students check in/out across different groups
- **THEN** the page SHALL receive events for all 3 groups
- **AND** SHALL update room/group displays accordingly
- **AND** SHALL handle rapid sequential events without UI glitches

### Requirement: Connection Status Indicator

The frontend SHALL display SSE connection status to users.

#### Scenario: Connected state display
- **GIVEN** an active SSE connection
- **THEN** the UI SHALL show green indicator with text "Live updates active"
- **AND** SHALL be visible but unobtrusive

#### Scenario: Reconnecting state display
- **GIVEN** SSE connection dropped and attempting reconnection
- **THEN** the UI SHALL show yellow/orange indicator with text "Reconnecting..."
- **AND** SHALL include attempt counter (e.g., "Attempt 2/5")

#### Scenario: Disconnected state display
- **GIVEN** SSE connection failed after max reconnection attempts
- **THEN** the UI SHALL show red indicator with text "Live updates unavailable"
- **AND** SHALL suggest manual refresh
- **AND** page SHALL remain functional (polling/manual refresh still works)

### Requirement: Event Type Handling

The frontend SHALL correctly parse and handle different SSE event types.

#### Scenario: student_checkin event handling
- **GIVEN** SSE event with type "student_checkin"
- **THEN** the handler SHALL extract active_group_id from event data
- **AND** SHALL check if current page displays that group
- **AND** SHALL trigger refetch of student list for that group
- **AND** SHALL show toast notification (optional, configurable)

#### Scenario: student_checkout event handling
- **GIVEN** SSE event with type "student_checkout"
- **THEN** the handler SHALL trigger refetch
- **AND** SHALL update UI to reflect student removal

#### Scenario: activity_update event handling
- **GIVEN** SSE event with type "activity_update"
- **THEN** the handler SHALL refetch group/room data
- **AND** SHALL update activity name, room assignment, or supervisor list

#### Scenario: Unknown event type handling
- **GIVEN** SSE event with unrecognized type
- **THEN** the handler SHALL log warning to console
- **AND** SHALL NOT crash or break page
- **AND** SHALL continue processing subsequent events

### Requirement: Graceful Degradation

The frontend SHALL remain fully functional even if SSE is unavailable.

#### Scenario: SSE connection failure
- **GIVEN** SSE endpoint unreachable or backend SSE disabled
- **WHEN** page loads
- **THEN** the page SHALL display data fetched via normal API calls
- **AND** SHALL show "Live updates unavailable" message
- **AND** SHALL allow manual refresh
- **AND** SHALL NOT show error overlays or prevent normal usage

#### Scenario: Browser without EventSource support
- **GIVEN** a browser without EventSource API (very old browsers)
- **THEN** the useSSE hook SHALL detect missing support
- **AND** SHALL log warning
- **AND** SHALL NOT establish SSE connection
- **AND** page SHALL work with manual refresh only
