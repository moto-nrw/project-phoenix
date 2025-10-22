# Service Base Patterns Capability

## ADDED Requirements

### Requirement: Base Service Abstraction
The system SHALL provide a base service struct in `services/base/service.go` to eliminate 1,200 lines of duplicated service patterns across 11 services.

#### Scenario: Common error wrapping
- **GIVEN** a service operation encounters an error
- **WHEN** service calls `s.wrapError(operation, err)`
- **THEN** helper SHALL create ServiceError with operation context
- **AND** SHALL preserve original error for unwrapping
- **AND** SHALL format as "service: operation: original error message"

#### Scenario: Database connection access
- **GIVEN** service extends BaseService
- **WHEN** service needs database access
- **THEN** SHALL access via `s.db` field from base
- **AND** SHALL not redefine database connection in concrete service

#### Scenario: Transaction handler access
- **GIVEN** service needs to run transactional operations
- **WHEN** accessing transaction handler
- **THEN** SHALL use `s.txHandler` from base service
- **AND** SHALL not create separate transaction handler instance

### Requirement: Standardized Error Types
The system SHALL provide shared service error types, eliminating 11 duplicate ServiceError type definitions.

#### Scenario: Single ServiceError definition
- **GIVEN** all services need error wrapping
- **WHEN** defining service errors
- **THEN** SHALL use `base.ServiceError` type
- **AND** SHALL NOT define custom ServiceError per service
- **AND** error SHALL implement Error() and Unwrap() methods

#### Scenario: Error context preservation
- **GIVEN** ServiceError wraps underlying error
- **WHEN** error is unwrapped using errors.Unwrap or errors.As
- **THEN** original error SHALL be accessible
- **AND** error chain SHALL be preserved for debugging

### Requirement: Transaction Helper Methods
The system SHALL provide transaction helpers to eliminate 57 duplicate transaction patterns.

#### Scenario: Simple transaction wrapper
- **GIVEN** service operation requires transaction
- **WHEN** calling `s.WithTransaction(ctx, func(ctx context.Context) error { ... })`
- **THEN** helper SHALL begin transaction
- **AND** SHALL execute provided function
- **AND** SHALL commit if function returns nil
- **AND** SHALL rollback if function returns error
- **AND** SHALL return any error from function or transaction

#### Scenario: Generic transaction with return value
- **GIVEN** transaction needs to return a value
- **WHEN** calling `result, err := s.RunInTx(ctx, func(ctx context.Context) (*Entity, error) { ... })`
- **THEN** helper SHALL execute function in transaction
- **AND** SHALL return both the result and error
- **AND** SHALL commit on success (nil error)
- **AND** SHALL rollback on error

#### Scenario: Nested transaction handling
- **GIVEN** transaction is already active in context
- **WHEN** calling transaction helper
- **THEN** SHALL reuse existing transaction (no nested transaction)
- **AND** SHALL not commit/rollback inner transaction
- **AND** outer transaction SHALL control commit/rollback

### Requirement: Validation Helper Methods
The system SHALL provide validation helpers to standardize entity validation across services.

#### Scenario: Validate before create
- **GIVEN** service creates new entity
- **WHEN** calling `s.ValidateAndCreate(ctx, entity)`
- **THEN** helper SHALL call entity.Validate()
- **AND** on validation error SHALL return wrapped error
- **AND** on success SHALL proceed with repository.Create
- **AND** SHALL wrap database errors appropriately

#### Scenario: Validate before update
- **GIVEN** service updates existing entity
- **WHEN** calling `s.ValidateAndUpdate(ctx, entity)`
- **THEN** helper SHALL validate entity
- **AND** SHALL call repository.Update on success
- **AND** SHALL return appropriate errors

## MODIFIED Requirements

### Requirement: Active Service Session Methods Consolidation
The ActiveService SHALL consolidate duplicate session creation methods into a single method with optional parameters.

**Context**: Currently `StartActivitySession` (lines 1405-1542) and `StartActivitySessionWithSupervisors` (lines 1568-1726) are 90% identical, differing only in supervisor handling.

#### Scenario: Session creation without supervisors
- **GIVEN** device wants to start activity session without specifying supervisors
- **WHEN** calling `StartActivitySession(ctx, activityID, deviceID, roomID, [])`
- **THEN** service SHALL create active group for activity
- **AND** SHALL NOT create supervisor assignments (empty slice)
- **AND** SHALL broadcast activity_start SSE event
- **AND** SHALL return created active group

#### Scenario: Session creation with supervisors
- **GIVEN** device wants to start activity session with specific supervisors
- **WHEN** calling `StartActivitySession(ctx, activityID, deviceID, roomID, supervisors)`
- **THEN** service SHALL create active group
- **AND** SHALL create supervisor assignments from provided list
- **AND** SHALL validate each supervisor exists
- **AND** SHALL broadcast activity_start SSE event with supervisor info

#### Scenario: Deprecated method compatibility
- **GIVEN** existing code calls old `StartActivitySessionWithSupervisors` method
- **WHEN** migration is in progress
- **THEN** old method SHALL delegate to new consolidated method
- **AND** SHALL be marked as deprecated in comments
- **AND** SHALL be removed after 1 release cycle

### Requirement: Service SSE Broadcasting Helpers
The ActiveService SHALL extract duplicated SSE broadcasting logic into helper methods.

#### Scenario: Safe SSE broadcast
- **GIVEN** service needs to broadcast SSE event
- **WHEN** calling `s.broadcastEventSafely(ctx, groupID, eventType, data)`
- **THEN** helper SHALL create realtime.Event
- **AND** SHALL call broadcaster.BroadcastToGroup
- **AND** SHALL handle nil broadcaster gracefully (no-op if testing)
- **AND** SHALL log errors but not fail operation (fire-and-forget)

#### Scenario: Student name lookup for events
- **GIVEN** SSE event needs student display name
- **WHEN** calling `s.getStudentDisplayName(ctx, studentID)`
- **THEN** helper SHALL query student and person records
- **AND** SHALL return "FirstName LastName" format
- **AND** SHALL return empty string on error (event still broadcasts)

### Requirement: Active Service Room Helpers
The ActiveService SHALL extract duplicated room validation logic.

#### Scenario: Room availability check
- **GIVEN** service needs to validate room is available
- **WHEN** calling `s.checkRoomAvailability(ctx, roomID, excludeGroupID)`
- **THEN** helper SHALL query for conflicting active groups
- **AND** SHALL exclude specified group ID from conflict check
- **AND** SHALL return error if room occupied by another group
- **AND** SHALL return nil if room available

#### Scenario: End all visits in group
- **GIVEN** activity session is ending
- **WHEN** calling `s.endAllGroupVisits(ctx, activeGroupID)`
- **THEN** helper SHALL query all visits for the group
- **AND** SHALL end each active visit
- **AND** SHALL broadcast checkout events for each student
- **AND** SHALL return error if any visit fails to end

## Migration Notes
- Base service adopted incrementally - services can extend base when migrated
- Old error types deprecated but not removed immediately (1 release cycle)
- Transaction helpers provide same behavior as manual txHandler.RunInTx calls
- SSE broadcasting helpers maintain fire-and-forget behavior (errors logged, not returned)
- Consolidated session methods maintain identical API behavior for callers
