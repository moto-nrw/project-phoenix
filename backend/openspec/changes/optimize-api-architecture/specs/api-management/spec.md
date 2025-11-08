# API Management Capability - Delta Specifications

## ADDED Requirements

### Requirement: API Versioning

The system SHALL support multiple API versions simultaneously to enable
backward-compatible evolution of the REST API.

#### Scenario: V1 and V2 coexistence

- **WHEN** a client requests `/api/v1/active/groups`
- **AND** another client requests `/api/v2/active/groups`
- **THEN** both requests SHALL succeed with appropriate response formats
- **AND** both versions SHALL use the same underlying service layer

#### Scenario: Version deprecation headers

- **WHEN** a client requests a deprecated V1 endpoint
- **THEN** the response SHALL include `X-API-Deprecated: true` header
- **AND** the response SHALL include `X-API-Sunset-Date` header with sunset
  timeline
- **AND** the response body SHALL remain unchanged (backward compatibility)

### Requirement: Endpoint Usage Tracking

The system SHALL track usage metrics for all API endpoints to enable data-driven
optimization decisions.

#### Scenario: Usage metrics collection

- **WHEN** any API endpoint is called
- **THEN** the system SHALL asynchronously record endpoint path, HTTP method,
  and timestamp
- **AND** the system SHALL increment the call count for that endpoint
- **AND** the tracking SHALL NOT impact response time (async operation)

#### Scenario: Unused endpoint identification

- **WHEN** an administrator queries endpoint usage statistics
- **AND** an endpoint has received <10 calls in 14 days
- **THEN** the endpoint SHALL be flagged as "unused"
- **AND** a deletion candidate report SHALL be generated

### Requirement: Query Parameter Filtering

The system SHALL support flexible filtering of list endpoints via query
parameters while maintaining permission boundaries.

#### Scenario: Simple list filtering

- **WHEN** a client requests `/api/v2/active/groups?status=unclaimed`
- **THEN** the system SHALL filter results to unclaimed groups only
- **AND** the system SHALL validate the user has permission to view groups
- **AND** the response SHALL match the existing group list format

#### Scenario: Multi-parameter filtering

- **WHEN** a client requests `/api/v2/visits?group_id=123&student_id=456`
- **THEN** the system SHALL filter by both group ID and student ID
- **AND** the system SHALL validate permission for both the group and student
- **AND** the query SHALL execute at database level (not application filtering)

#### Scenario: Unauthorized filter parameter

- **WHEN** a client requests `/api/v2/students?teacher_id=999`
- **AND** the authenticated user is not authorized to access teacher 999's data
- **THEN** the system SHALL return HTTP 403 Forbidden
- **AND** the response SHALL include error message "Unauthorized access to
  teacher data"

### Requirement: IoT API Stability

The system SHALL maintain absolute backward compatibility for IoT device
endpoints to prevent firmware incompatibility.

#### Scenario: IoT endpoint frozen

- **WHEN** a backend developer attempts to modify `/iot/checkin` endpoint
- **AND** the change would alter request or response format
- **THEN** the code review process SHALL reject the change
- **AND** the developer SHALL be directed to create a new versioned endpoint
  instead

#### Scenario: IoT device authentication unchanged

- **WHEN** an RFID device authenticates using Device API Key + Staff PIN
- **THEN** the authentication mechanism SHALL remain unchanged from current
  implementation
- **AND** any new auth methods SHALL be additive (not replacing existing)

## MODIFIED Requirements

_None - This change adds new capabilities without modifying existing
requirements_

## REMOVED Requirements

### Requirement: Specialized List Endpoints

**Reason:** Replaced by query parameter filtering in V2 API for better
maintainability

**Endpoints Removed:**

- `GET /api/active/groups/unclaimed` → Use
  `/api/v2/active/groups?status=unclaimed`
- `GET /api/active/groups/room/:roomId` → Use
  `/api/v2/active/groups?room_id=:roomId`
- `GET /api/active/groups/group/:groupId` → Use
  `/api/v2/active/groups?group_id=:groupId`

**Migration:** V1 endpoints remain available for 6 months; frontend should
migrate to V2 query params

#### Scenario: V1 specialized endpoint still functional

- **WHEN** a client requests `/api/active/groups/unclaimed` (V1)
- **THEN** the endpoint SHALL continue to function as before
- **AND** the response SHALL include deprecation header
- **BUT** no new V1 specialized endpoints SHALL be created
