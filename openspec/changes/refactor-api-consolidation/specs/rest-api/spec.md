# REST API Design Capability

## MODIFIED Requirements

### Requirement: RESTful Query Parameter Filtering
API endpoints SHALL use query parameters for filtering and searching, not path-based filter segments.

**Context**: Currently 26 endpoints violate REST principles by encoding filters in URL paths (e.g., `/by-category/{cat}`, `/status/{status}`), making the API inconsistent and harder to discover.

#### Scenario: Filter by field value
- **GIVEN** client wants to filter resources by a field
- **WHEN** making GET request to resource collection
- **THEN** SHALL use query parameter: `GET /api/resources?field=value`
- **AND** SHALL NOT use path segment: `/api/resources/by-field/{value}`

#### Scenario: Filter by status
- **GIVEN** client wants devices with specific status
- **WHEN** querying devices
- **THEN** SHALL use `GET /api/v2/iot?status=active`
- **AND** endpoints `/api/iot/active`, `/api/iot/offline`, `/api/iot/maintenance` SHALL be removed
- **AND** SHALL support multiple status values: `?status=active,offline`

#### Scenario: Filter by type
- **GIVEN** client wants to filter by resource type
- **WHEN** querying collection
- **THEN** SHALL use `GET /api/v2/iot?type=rfid`
- **AND** endpoint `/api/iot/type/{type}` SHALL be removed

#### Scenario: Boolean availability filter
- **GIVEN** client wants available resources only
- **WHEN** querying staff, rooms, or activities
- **THEN** SHALL use `?available=true`
- **AND** endpoints `/staff/available`, `/rooms/available`, `/activities/available` SHALL be removed

### Requirement: REST State Changes via PATCH
API endpoints SHALL use PATCH method for resource state updates, not custom action endpoints.

#### Scenario: End active session
- **GIVEN** client wants to end an active group session
- **WHEN** updating session state
- **THEN** SHALL use `PATCH /api/v2/active/groups/{id}` with body `{"status": "ended", "ended_at": "timestamp"}`
- **AND** endpoint `POST /api/active/groups/{id}/end` SHALL be removed

#### Scenario: Claim unclaimed group
- **GIVEN** supervisor wants to claim an active group
- **WHEN** updating group ownership
- **THEN** SHALL use `PATCH /api/v2/active/groups/{id}` with body `{"claimed_by": staffId}`
- **AND** endpoint `POST /api/active/groups/{id}/claim` SHALL be removed

#### Scenario: Activate/deactivate account
- **GIVEN** admin wants to change account active status
- **WHEN** updating account
- **THEN** SHALL use `PATCH /auth/v2/accounts/{id}` with body `{"active": true}` or `{"active": false}`
- **AND** endpoints `POST /accounts/{id}/activate` and `/accounts/{id}/deactivate` SHALL be removed

#### Scenario: Idempotent state updates
- **GIVEN** client sends PATCH with state update
- **WHEN** state is already in desired state
- **THEN** SHALL return 200 OK with unchanged resource
- **AND** SHALL NOT return error for idempotent operations

### Requirement: Canonical Resource Paths
Each resource SHALL have one canonical access path, eliminating duplicate routes.

#### Scenario: Student visit access
- **GIVEN** client needs student's visits
- **WHEN** querying visit data
- **THEN** canonical path SHALL be `GET /api/v2/students/{id}/visits`
- **AND** endpoint `/api/active/visits/student/{studentId}` SHALL be removed (duplicate)

#### Scenario: Current visit access
- **GIVEN** client needs student's current active visit
- **WHEN** querying current visit
- **THEN** canonical path SHALL be `GET /api/v2/students/{id}/current-visit`
- **AND** endpoint `/api/active/visits/student/{studentId}/current` SHALL be removed

#### Scenario: Staff supervised groups
- **GIVEN** client needs staff member's supervised groups
- **WHEN** querying groups
- **THEN** canonical path SHALL be `GET /api/v2/staff/{id}/groups`
- **AND** endpoint `/api/active/supervisors/staff/{staffId}` SHALL be removed

### Requirement: Maximum URL Nesting Depth
API endpoints SHALL NOT exceed 3 levels of nesting (excluding API version prefix).

#### Scenario: Flat analytics endpoints
- **GIVEN** client requests analytics data
- **WHEN** accessing analytics
- **THEN** SHALL use `GET /api/v2/analytics/dashboard` or `/api/v2/analytics/room-utilization?room_id=X`
- **AND** SHALL NOT use 4+ levels: `/api/active/analytics/room/{roomId}/utilization`

#### Scenario: Scheduled checkout access
- **GIVEN** client queries scheduled checkouts for student
- **WHEN** filtering by student
- **THEN** SHALL use `GET /api/v2/scheduled-checkouts?student_id=X&pending=true`
- **AND** SHALL NOT use `/api/active/scheduled-checkouts/student/{studentId}/pending`

### Requirement: Remove Business Logic from URLs
API endpoint paths SHALL represent resources and relationships, not business rules or specific queries.

#### Scenario: Feedback by category
- **GIVEN** client wants feedback for specific category
- **WHEN** querying feedback
- **THEN** SHALL use `GET /api/v2/feedback?category=mensa`
- **AND** hardcoded endpoint `/api/feedback/mensa` SHALL be removed

#### Scenario: Room status queries
- **GIVEN** client needs student's room status within group
- **WHEN** querying student data
- **THEN** SHALL derive from `GET /api/v2/students/{id}/current-visit` (includes room info)
- **AND** specific endpoint `/api/groups/{id}/students/room-status` SHALL be removed

#### Scenario: In-group-room check
- **GIVEN** client checks if student is in their group's assigned room
- **WHEN** querying student location
- **THEN** SHALL compute from visit data on client side or use general query
- **AND** endpoint `/api/students/{id}/in-group-room` SHALL be removed (too specific)

## ADDED Requirements

### Requirement: API Versioning Infrastructure
The system SHALL support multiple API versions simultaneously during migration periods.

#### Scenario: Version prefix routing
- **GIVEN** API supports v1 and v2
- **WHEN** client makes request to `/api/v1/users`
- **THEN** request SHALL route to v1 handler or adapter
- **AND** WHEN client requests `/api/v2/users`
- **THEN** SHALL route to v2 handler
- **AND** both versions SHALL coexist without conflicts

#### Scenario: Version deprecation headers
- **GIVEN** v1 API is deprecated
- **WHEN** client makes request to `/api/v1/*`
- **THEN** response SHALL include `Deprecation: true` header
- **AND** SHALL include `Sunset: YYYY-MM-DD` header with removal date
- **AND** SHALL include `Link: <https://docs/migration>; rel="deprecation"`

#### Scenario: Default version handling
- **GIVEN** client requests `/api/users` without version prefix
- **WHEN** processing request
- **THEN** SHALL default to latest stable version (v2)
- **OR** SHALL return 400 with clear error requiring version specification
- **Decision**: Return 400 requiring explicit version (prevents accidental usage)

### Requirement: v1-to-v2 Adapter Layer
The system SHALL provide adapters to transform v1 API calls to v2 format during migration.

#### Scenario: Path parameter to query parameter adaptation
- **GIVEN** v1 endpoint `/api/v1/rooms/by-category/{category}`
- **WHEN** adapter processes request
- **THEN** SHALL extract `{category}` from path
- **AND** SHALL transform to v2 call: `GET /api/v2/rooms?category={category}`
- **AND** SHALL forward request to v2 handler
- **AND** SHALL return v2 response unchanged

#### Scenario: Action endpoint to PATCH adaptation
- **GIVEN** v1 endpoint `POST /api/v1/active/groups/{id}/end`
- **WHEN** adapter processes request
- **THEN** SHALL transform to `PATCH /api/v2/active/groups/{id}` with body `{"status": "ended"}`
- **AND** SHALL preserve original request context (auth, headers)

#### Scenario: Adapter error handling
- **GIVEN** adapter transforms v1 call to v2
- **WHEN** v2 handler returns error
- **THEN** adapter SHALL pass error through unchanged
- **AND** SHALL maintain v1-compatible error response format

### Requirement: Query Parameter Validation
All query parameters SHALL be validated to prevent injection and ensure type safety.

#### Scenario: Validate allowed query parameters
- **GIVEN** endpoint accepts specific query parameters
- **WHEN** client provides unknown parameter
- **THEN** SHALL ignore unknown parameters (lenient)
- **OR** SHALL return 400 Bad Request listing allowed parameters (strict mode)
- **Decision**: Lenient by default, strict mode via config

#### Scenario: Type validation for query values
- **GIVEN** query parameter expects boolean
- **WHEN** client provides `?active=yes` instead of `?active=true`
- **THEN** SHALL return 400 Bad Request with clear error message
- **AND** error SHALL specify expected format: "Parameter 'active' must be 'true' or 'false'"

#### Scenario: Array parameter parsing
- **GIVEN** query parameter accepts multiple values
- **WHEN** client provides `?status=active,offline,maintenance`
- **THEN** SHALL parse as array `["active", "offline", "maintenance"]`
- **AND** SHALL support both comma-separated and repeated params: `?status=active&status=offline`

## REMOVED Requirements

### Requirement: Path-Based Filtering Endpoints
**Reason**: Violates REST principles, creates API bloat
**Migration**: All `/by-{field}/{value}` patterns replaced with query parameters in v2

Removed endpoints:
- `/api/rooms/by-category/{category}`
- `/api/users/by-account/{accountId}`
- `/api/users/by-tag/{tagId}`
- `/api/config/key/{key}`
- `/api/config/category/{category}`
- `/api/iot/type/{type}`
- `/api/iot/status/{status}`
- `/api/iot/registered-by/{personId}`
- `/api/schedules/dateframes/by-date`
- `/api/schedules/recurrence-rules/by-frequency`
- `/api/schedules/recurrence-rules/by-weekday`
- `/api/schedules/timeframes/by-range`

### Requirement: Status-Specific Filter Endpoints
**Reason**: Each status value creates separate endpoint, causing explosion
**Migration**: Consolidate to base endpoint with `?status=X` query parameter

Removed endpoints:
- `/api/iot/active`, `/api/iot/offline`, `/api/iot/maintenance`
- `/api/rooms/available`, `/api/staff/available`, `/api/staff/available-for-substitution`
- `/api/activities/schedules/available`, `/api/activities/supervisors/available`
- `/api/active/combined/active`, `/api/active/groups/unclaimed`
- `/api/substitutions/active`
- `/api/schedules/timeframes/active`
- `/api/me/groups/active`, `/api/me/groups/activity`
- `/api/users/rfid-cards/available`

### Requirement: Custom Action Endpoints
**Reason**: Non-RESTful, state changes should use PATCH
**Migration**: Replace with PATCH requests updating resource state

Removed endpoints:
- `/api/active/groups/{id}/claim` → PATCH with `{claimed_by}`
- `/api/active/groups/{id}/end` → PATCH with `{status: "ended"}`
- `/api/active/combined/{id}/end` → PATCH with `{status: "ended"}`
- `/api/active/supervisors/{id}/end` → PATCH with `{ended_at}`
- `/api/active/visits/{id}/end` → PATCH with `{ended_at}`
- `/api/active/visits/student/{studentId}/checkout` → PATCH current visit
- `/auth/accounts/{id}/activate` → PATCH with `{active: true}`
- `/auth/accounts/{id}/deactivate` → PATCH with `{active: false}`
- `/auth/parent-accounts/{id}/activate` → PATCH with `{active: true}`
- `/auth/parent-accounts/{id}/deactivate` → PATCH with `{active: false}`
- `/api/activities/quick-create` → POST /api/activities with template
- `/api/config/initialize-defaults` → POST /api/config with defaults body
- `/api/iot/checkin` → POST /api/active/visits
- `/api/active/mappings/add` → POST /api/active/mappings
- `/api/active/mappings/remove` → DELETE /api/active/mappings/{id}

## Migration Notes

**Breaking Changes**: This proposal removes 110 endpoints and changes URL patterns for remaining endpoints.

**v1 Compatibility**: Old endpoints continue working via adapter layer during 3-month transition.

**Frontend Migration Required**: ~110 API call sites need updating to v2 URLs and patterns.

**Timeline**: 5-month migration (1 month backend, 2 months frontend, 1 month deprecation, 1 month cleanup).

**Rollback**: Can revert to v1-only by removing v2 router (but loses consolidation benefits).
