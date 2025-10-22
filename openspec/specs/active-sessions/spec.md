# active-sessions Specification

## Purpose
TBD - created by archiving change add-sse-real-time-updates. Update Purpose after archive.
## Requirements
### Requirement: Event Broadcasting Integration

Active session services SHALL integrate with SSE hub for event distribution.

#### Scenario: Hub dependency injection
- **GIVEN** active session services (check-in, check-out, session management)
- **THEN** services SHALL receive SSE hub instance via constructor
- **AND** SHALL call hub.BroadcastToGroup() after successful database writes
- **AND** SHALL continue operation even if broadcast fails (fire-and-forget)

#### Scenario: Broadcast failure handling
- **GIVEN** a student check-in with SSE hub unavailable
- **WHEN** hub.BroadcastToGroup() fails
- **THEN** the system SHALL log error
- **AND** SHALL continue processing (check-in succeeds)
- **AND** SHALL NOT fail the HTTP request
- **AND** clients will sync via manual refresh

### Requirement: Audit Logging for Real-Time Events

The system SHALL log SSE event broadcasts for GDPR compliance and debugging using backend/logging.Logger.

#### Scenario: Event broadcast logging with structured logger
- **GIVEN** an SSE event is broadcast
- **THEN** the system SHALL use backend/logging.Logger (NOT fmt.Printf or log.Printf)
- **AND** SHALL log:
  - Event type
  - Active group ID
  - Number of recipients
  - Timestamp
- **AND** log level SHALL be DEBUG for normal events (student_checkin, student_checkout)
- **AND** log level SHALL be INFO for session start/end events
- **AND** log level SHALL be ERROR for broadcast failures

#### Scenario: SSE connection audit logging
- **GIVEN** a supervisor establishes or closes SSE connection
- **THEN** the system SHALL log at INFO level with:
  - User ID
  - Subscribed group IDs
  - Connection/disconnection timestamp
- **AND** SHALL use structured logging for GDPR audit trail

