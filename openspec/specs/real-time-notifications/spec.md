# real-time-notifications Specification

## Purpose
TBD - created by archiving change add-sse-real-time-updates. Update Purpose after archive.
## Requirements
### Requirement: SSE Connection Endpoint

The system SHALL provide a `/api/sse/events` HTTP endpoint that establishes Server-Sent Events (SSE) connections for real-time notifications.

#### Scenario: Supervisor establishes SSE connection
- **GIVEN** a supervisor with valid JWT token
- **WHEN** they connect to `/api/sse/events`
- **THEN** the system SHALL establish an SSE stream with `Content-Type: text/event-stream`
- **AND** SHALL query active.GetStaffActiveSupervisions(userID) to determine supervised groups
- **AND** SHALL subscribe them to all returned active_group_ids

#### Scenario: Unauthorized access rejected
- **GIVEN** a request without valid JWT token
- **WHEN** attempting to connect to `/api/sse/events`
- **THEN** the system SHALL return HTTP 401 Unauthorized
- **AND** SHALL NOT establish SSE connection

#### Scenario: Connection terminated on token expiry
- **GIVEN** an active SSE connection
- **WHEN** the JWT token expires (15 minutes)
- **THEN** the system SHALL close the connection
- **AND** the client SHALL automatically attempt to reconnect

### Requirement: Group-Based Event Subscription

The system SHALL automatically subscribe SSE clients to active groups based on their supervisor permissions.

#### Scenario: Auto-discovery of supervised groups
- **GIVEN** a supervisor connects to SSE endpoint
- **WHEN** the connection is established
- **THEN** the system SHALL query `active.groups` for groups where supervisor_id matches AND end_time IS NULL
- **AND** SHALL subscribe the client to all returned active_group_ids

#### Scenario: Only receive events for subscribed groups
- **GIVEN** a supervisor subscribed to groups "Basketball" and "Chess"
- **WHEN** a student checks into "Football" group
- **THEN** the supervisor SHALL NOT receive that event
- **WHEN** a student checks into "Basketball" group
- **THEN** the supervisor SHALL receive the event within 1 second

### Requirement: Event Broadcasting

The system SHALL broadcast events to all SSE clients subscribed to the affected active group.

#### Scenario: Student check-in event broadcast
- **GIVEN** an SSE hub with clients subscribed to active_group "42"
- **WHEN** a student checks into that group
- **THEN** the system SHALL broadcast event with type "student_checkin"
- **AND** SHALL include student_id and student_name in event data (minimal trigger payload)
- **AND** SHALL deliver event to all subscribers within 1 second
- **AND** client SHALL refetch full data via bulk endpoint after receiving event

#### Scenario: Student check-out event broadcast
- **GIVEN** an SSE hub with clients subscribed to active_group "42"
- **WHEN** a student checks out of that group
- **THEN** the system SHALL broadcast event with type "student_checkout"
- **AND** SHALL include student_id and timestamp

#### Scenario: Activity lifecycle events
- **GIVEN** supervisors subscribed to an active group
- **WHEN** the group session starts
- **THEN** the system SHALL broadcast "activity_start" event
- **WHEN** the group session ends
- **THEN** the system SHALL broadcast "activity_end" event

### Requirement: Event Payload Structure

Event payloads SHALL contain minimal trigger data only. Full data fetched via bulk refetch endpoint.

**Architecture Decision:** SSE events are notification triggers, not data payloads. This pattern:
- Minimizes bandwidth (event size ~100 bytes vs ~2KB for full student data)
- Ensures data freshness (bulk endpoint always returns current state)
- Simplifies event schema (no need for school_class, group_name - implicit in subscription context)

#### Scenario: Check-in event payload format
- **GIVEN** a student check-in event
- **THEN** the event SHALL include:
  - type: "student_checkin"
  - active_group_id: string (trigger scope)
  - data.student_id: string (notification identifier)
  - data.student_name: string (optional display hint)
  - timestamp: ISO 8601 string
- **AND** SHALL NOT include school_class (fetched in bulk response)
- **AND** SHALL NOT include group_name (implicit in active_group_id subscription)
- **AND** SHALL NOT include sensitive data (birthday, address, guardian info)

#### Scenario: Event format compliance
- **GIVEN** any SSE event
- **THEN** the event SHALL follow format: `event: <type>\ndata: <json>\n\n`
- **AND** data SHALL be valid JSON

### Requirement: Connection Management

The system SHALL manage SSE client lifecycles including registration, heartbeat, and cleanup.

#### Scenario: Client registration
- **GIVEN** a new SSE connection
- **WHEN** registration occurs
- **THEN** the system SHALL add client to hub
- **AND** SHALL log connection with user_id and subscribed group_ids (GDPR audit)

#### Scenario: Client disconnection cleanup
- **GIVEN** an active SSE connection
- **WHEN** the client disconnects or connection fails
- **THEN** the system SHALL remove client from all group subscriptions
- **AND** SHALL close the channel
- **AND** SHALL log disconnection event

#### Scenario: Heartbeat to prevent timeout
- **GIVEN** an active SSE connection
- **WHEN** no events sent for 30 seconds
- **THEN** the system SHALL send comment line `: heartbeat\n\n`
- **AND** client connection SHALL remain open

#### Scenario: Client disconnect detection
- **GIVEN** an active SSE connection
- **WHEN** detecting client disconnect
- **THEN** the system SHALL use `r.Context().Done()` channel (NOT http.CloseNotifier - deprecated in Go 1.20+)
- **AND** SHALL unregister client immediately
- **AND** SHALL close event channel

### Requirement: Auto-Reconnection

The frontend SSE client SHALL implement automatic reconnection with exponential backoff.

#### Scenario: Network failure recovery
- **GIVEN** an active SSE connection
- **WHEN** network connection drops
- **THEN** the client SHALL detect error event
- **AND** SHALL attempt reconnection after 1 second
- **AND** SHALL use exponential backoff (2^n seconds) for subsequent failures
- **AND** SHALL stop after 5 failed attempts

#### Scenario: Reconnection with fresh token
- **GIVEN** SSE connection dropped due to token expiry
- **WHEN** client attempts reconnection
- **THEN** the client SHALL fetch fresh session token
- **AND** SHALL establish new SSE connection with updated token

### Requirement: Security and Authorization

SSE connections SHALL enforce same security policies as REST API endpoints.

#### Scenario: JWT authentication required
- **GIVEN** an SSE connection request
- **THEN** the system SHALL validate JWT token using existing auth middleware
- **AND** SHALL extract user_id from token claims
- **AND** SHALL reject connections with invalid/expired tokens

#### Scenario: Permission-based subscription filtering
- **GIVEN** a supervisor with permission to view only specific groups
- **WHEN** determining group subscriptions
- **THEN** the system SHALL only subscribe to groups they have permission to supervise
- **AND** SHALL NOT subscribe to groups outside their authorization scope

#### Scenario: HTTPS required for production
- **GIVEN** production environment
- **THEN** SSE connections SHALL only be allowed over HTTPS
- **AND** SHALL reject plain HTTP connections

