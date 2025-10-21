# Active Sessions - SSE Event Broadcasting

## MODIFIED Requirements

### Requirement: Student Check-In Processing

The system SHALL process student check-ins via RFID and broadcast real-time events to supervisors.

#### Scenario: Successful RFID check-in
- **GIVEN** a student with valid RFID card at a device
- **WHEN** the student scans their card
- **THEN** the system SHALL create visit record in `active.visits` table
- **AND** SHALL broadcast SSE event of type "student_checkin" to active_group subscribers
- **AND** event SHALL include student_id, student_name, school_class, and timestamp
- **AND** SHALL return HTTP 200 with visit details

#### Scenario: Manual check-in via webapp
- **GIVEN** a supervisor manually checking in a student
- **WHEN** the check-in is processed
- **THEN** the system SHALL create visit record
- **AND** SHALL broadcast SSE event to all supervisors of that active group
- **AND** SHALL include check-in source (manual vs RFID) in audit log

### Requirement: Student Check-Out Processing

The system SHALL process student check-outs and broadcast real-time events.

#### Scenario: RFID check-out
- **GIVEN** a student with active visit
- **WHEN** the student scans RFID to check out
- **THEN** the system SHALL update visit record with check_out_time
- **AND** SHALL set is_active to false
- **AND** SHALL reload visit with student/person data (for event payload)
- **AND** SHALL broadcast SSE event of type "student_checkout"
- **AND** event SHALL include student_id, student_name, active_group_id, and timestamp

#### Scenario: Manual check-out
- **GIVEN** a supervisor manually checking out a student
- **WHEN** the check-out is processed via EndVisit() (line 429)
- **THEN** the system SHALL update visit record
- **AND** SHALL reload visit data to get student display fields
- **AND** SHALL broadcast SSE event to all group supervisors
- **AND** SHALL return HTTP 200

### Requirement: Activity Session Lifecycle

The system SHALL broadcast events when activity sessions start or end.

#### Scenario: Session start notification (StartActivitySession)
- **GIVEN** a supervisor claims an unclaimed group/room via StartActivitySession() (line 1251)
- **WHEN** the active session is created
- **THEN** the system SHALL query room and group metadata before broadcasting
- **AND** SHALL broadcast "activity_start" event
- **AND** event SHALL include active_group_id, activity_name, room_id, room_name, supervisor_ids
- **AND** newly assigned supervisors SHALL receive the event on their next SSE connection

#### Scenario: Session start with multiple supervisors (StartActivitySessionWithSupervisors)
- **GIVEN** a supervisor starts session with additional supervisors via StartActivitySessionWithSupervisors() (line 1374)
- **WHEN** the active session is created
- **THEN** the system SHALL query room and group metadata before broadcasting
- **AND** SHALL broadcast "activity_start" event
- **AND** event SHALL include all assigned supervisor_ids

#### Scenario: Session end notification (EndActivitySession)
- **GIVEN** an active session
- **WHEN** the supervisor ends the session via EndActivitySession() (line 1801)
- **THEN** the system SHALL update end_time in active.groups
- **AND** SHALL query final session metadata before broadcasting
- **AND** SHALL broadcast "activity_end" event to all subscribers
- **AND** clients SHALL unsubscribe from that active_group_id

#### Scenario: Automated scheduled checkout (ProcessDueScheduledCheckouts)
- **GIVEN** scheduled checkouts are due
- **WHEN** ProcessDueScheduledCheckouts() runs (line 2314)
- **THEN** the system SHALL broadcast "student_checkout" event for each processed checkout using fire-and-forget pattern
- **AND** broadcast failures SHALL NOT block the checkout loop
- **AND** SHALL log broadcast errors at ERROR level
- **AND** event SHALL indicate automated source

## ADDED Requirements

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
