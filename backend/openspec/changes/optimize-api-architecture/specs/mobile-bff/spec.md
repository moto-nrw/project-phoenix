# Mobile Backend-For-Frontend Capability - Delta Specifications

## ADDED Requirements

### Requirement: Mobile Dashboard Aggregation
The system SHALL provide a mobile-optimized dashboard endpoint that aggregates data from multiple services in a single request.

#### Scenario: Dashboard data aggregation
- **WHEN** a mobile client requests `GET /mobile/v1/dashboard`
- **THEN** the system SHALL fetch analytics, active groups, and student data in parallel
- **AND** the response SHALL combine all data into a single JSON object
- **AND** the response time SHALL be <200ms p95

#### Scenario: Parallel service execution
- **WHEN** the mobile dashboard endpoint is called
- **THEN** all service calls SHALL execute concurrently using goroutines
- **AND** the endpoint SHALL wait for all calls to complete before responding
- **AND** if any service call fails, the response SHALL include partial data with error indicators

#### Scenario: Mobile authentication
- **WHEN** a mobile client requests a BFF endpoint
- **THEN** the system SHALL use the same JWT authentication as web clients
- **AND** the mobile client SHALL include JWT in Authorization header
- **AND** expired tokens SHALL be rejected with HTTP 401

### Requirement: Batch Query Endpoint
The system SHALL support batch queries to reduce round trips for mobile clients fetching multiple resources.

#### Scenario: Batch query request
- **WHEN** a mobile client requests `POST /mobile/v1/batch-query`
- **AND** the request body contains `{"queries": [{"endpoint": "/students", "params": {...}}, {"endpoint": "/groups", "params": {...}}]}`
- **THEN** the system SHALL execute all queries in parallel
- **AND** the response SHALL include results for each query in the same order
- **AND** individual query failures SHALL NOT fail the entire batch

#### Scenario: Batch query size limit
- **WHEN** a mobile client submits a batch query with >10 queries
- **THEN** the system SHALL return HTTP 400 Bad Request
- **AND** the error message SHALL indicate "Maximum 10 queries per batch request"

### Requirement: Sparse Fieldsets Support
The system SHALL support field selection to reduce payload size for mobile clients on limited bandwidth.

#### Scenario: Minimal fieldset request
- **WHEN** a mobile client requests `/mobile/v1/students?fields[students]=id,name,location`
- **THEN** the response SHALL include only id, name, and location fields
- **AND** all other student fields SHALL be omitted
- **AND** the payload size SHALL be ~90% smaller than full response

#### Scenario: Nested resource fieldsets
- **WHEN** a mobile client requests `/mobile/v1/visits?fields[visits]=id,checkInTime&fields[students]=id,name`
- **THEN** visit objects SHALL include only id and checkInTime
- **AND** nested student objects SHALL include only id and name
- **AND** the system SHALL use database-level column selection (not application filtering)

#### Scenario: Invalid field selection
- **WHEN** a mobile client requests `/mobile/v1/students?fields[students]=id,invalidField`
- **THEN** the system SHALL return HTTP 400 Bad Request
- **AND** the error message SHALL list valid field names

### Requirement: Cursor Pagination for Infinite Scroll
The system SHALL support cursor-based pagination for mobile list endpoints to enable infinite scroll UX.

#### Scenario: Cursor-based pagination
- **WHEN** a mobile client requests `/mobile/v1/students?cursor=eyJpZCI6MTIzfQ&limit=50`
- **THEN** the system SHALL return 50 students starting after the cursor position
- **AND** the response SHALL include `meta.next_cursor` for fetching the next page
- **AND** the response SHALL include `meta.has_more` boolean indicator

#### Scenario: First page without cursor
- **WHEN** a mobile client requests `/mobile/v1/students?limit=50` without cursor parameter
- **THEN** the system SHALL return the first 50 students ordered by ID descending
- **AND** the response SHALL include cursor for fetching page 2

#### Scenario: No more results
- **WHEN** a mobile client requests a cursor pagination query
- **AND** no more results exist after the cursor
- **THEN** the response SHALL include `meta.has_more: false`
- **AND** the response SHALL NOT include `meta.next_cursor`

### Requirement: Mobile-Optimized Response Format
The system SHALL provide mobile-optimized response formats with reduced nesting and consistent structure.

#### Scenario: Flattened response structure
- **WHEN** a mobile client requests `/mobile/v1/active-groups/:id`
- **THEN** nested resources SHALL be flattened into top-level fields where appropriate
- **AND** the response SHALL use camelCase (not snake_case) for JavaScript compatibility
- **AND** timestamps SHALL be ISO 8601 formatted strings

#### Scenario: Response compression
- **WHEN** a mobile client includes `Accept-Encoding: gzip` header
- **THEN** the response SHALL be gzip-compressed
- **AND** the `Content-Encoding: gzip` header SHALL be present
- **AND** compressed payload SHALL be ~70% smaller than uncompressed

## MODIFIED Requirements

_None - Mobile BFF is a new capability, no existing requirements modified_

## REMOVED Requirements

_None - Mobile BFF does not remove any existing capabilities_
