# API Decomposition Capability

## MODIFIED Requirements

### Requirement: IoT API Domain-Based Decomposition
The IoT API SHALL be decomposed from a single 3,063-line file into focused domain modules, applying all helper patterns from earlier phases.

**Context**: `api/iot/api.go` is the largest file with 65+ functions, 8 service dependencies, and massive duplication (179 error renders, 23 device validations, etc.). This god class makes maintenance and testing extremely difficult.

#### Scenario: Device management module
- **GIVEN** IoT API handles device CRUD operations
- **WHEN** decomposed into `api/iot/devices/` package
- **THEN** module SHALL contain device listing, creation, update, deletion handlers
- **AND** SHALL be approximately 400 lines
- **AND** SHALL use API helpers for error handling, ID parsing, request binding
- **AND** SHALL use shared DeviceResponse type
- **AND** SHALL have focused router mounting only device routes

#### Scenario: RFID checkin module
- **GIVEN** IoT API handles RFID check-in/check-out logic
- **WHEN** decomposed into `api/iot/checkin/` package
- **THEN** module SHALL contain deviceCheckin, checkRFIDTagAssignment handlers
- **AND** SHALL be approximately 350 lines
- **AND** SHALL use device authentication middleware
- **AND** SHALL focus solely on student check-in/out workflows

#### Scenario: Session management module
- **GIVEN** IoT API handles activity session lifecycle
- **WHEN** decomposed into `api/iot/sessions/` package
- **THEN** module SHALL contain session start, end, timeout handlers
- **AND** SHALL be approximately 450 lines
- **AND** SHALL use service layer for session business logic
- **AND** SHALL handle supervisor management within sessions

#### Scenario: Feedback submission module
- **GIVEN** IoT API handles device feedback submission
- **WHEN** decomposed into `api/iot/feedback/` package
- **THEN** module SHALL contain feedback creation handlers
- **AND** SHALL be approximately 200 lines
- **AND** SHALL integrate with FeedbackService

#### Scenario: Main router composition
- **GIVEN** IoT API decomposed into subpackages
- **WHEN** requests route to `/api/iot/*`
- **THEN** main `api/iot/resource.go` SHALL mount subpackage routers
- **AND** SHALL be approximately 100 lines (routing only)
- **AND** each subpackage SHALL expose `Router() chi.Router` method
- **AND** routes SHALL maintain identical URL structure (backward compatible)

### Requirement: Active Service Analytics Extraction
The ActiveService SHALL extract analytics/dashboard logic into separate analytics service, reducing file from 2,836 to ~1,800 lines.

#### Scenario: Analytics service creation
- **GIVEN** ActiveService contains dashboard analytics methods
- **WHEN** extracting to `services/analytics/` package
- **THEN** AnalyticsService SHALL contain:
  - GetDashboardAnalytics
  - GetDashboardCounts
  - GetRoomUtilization
  - GetStudentAttendanceStats
- **AND** SHALL extend base service patterns
- **AND** SHALL be approximately 800 lines

#### Scenario: ActiveService composition
- **GIVEN** ActiveService needs analytics functionality
- **WHEN** refactored with analytics extraction
- **THEN** ActiveService SHALL embed AnalyticsService via composition
- **AND** SHALL delegate analytics calls to embedded service
- **AND** external callers SHALL see no API changes

### Requirement: Large API File Refactoring
API files exceeding 1,500 lines SHALL apply all Phase 1-3 patterns to achieve target size <1,200 lines.

#### Scenario: Auth API consolidation
- **GIVEN** `api/auth/api.go` has 1,981 lines with 802 duplicate lines
- **WHEN** applying helper patterns
- **THEN** SHALL eliminate 108 duplicate error renders using renderError helper
- **AND** SHALL eliminate 33 duplicate ID parsers using parseIDParam helper
- **AND** SHALL eliminate 12 duplicate request binders using bindRequest helper
- **AND** SHALL reduce to approximately 1,100 lines
- **AND** SHALL maintain identical API behavior

#### Scenario: Students API consolidation
- **GIVEN** `api/students/api.go` has 1,725 lines with 625 duplicate lines
- **WHEN** applying all patterns
- **THEN** SHALL use API helpers (61 error renders, 11 ID parsers)
- **AND** SHALL use shared StudentDetail/StudentSummary response types
- **AND** SHALL extract permission checking to helper functions
- **AND** SHALL reduce to approximately 1,100 lines

#### Scenario: Activities API consolidation
- **GIVEN** `api/activities/api.go` has 1,951 lines with 726 duplicate lines
- **WHEN** applying all patterns
- **THEN** SHALL eliminate error handling duplication (79 occurrences)
- **AND** SHALL eliminate ID parsing duplication (25 occurrences)
- **AND** SHALL consolidate activity/supervisor/schedule response building
- **AND** SHALL reduce to approximately 1,200 lines

### Requirement: File Size Enforcement
After decomposition, NO API handler file SHALL exceed 500 lines per module or 1,200 lines for non-decomposed files.

#### Scenario: Module size limits
- **GIVEN** API decomposed into domain modules
- **WHEN** measuring module file sizes
- **THEN** each module's handlers.go SHALL be ≤500 lines
- **AND** types.go and router.go SHALL each be ≤200 lines
- **AND** total module size SHALL be reasonable for focused domain

#### Scenario: God class prevention
- **GIVEN** developer adds new functionality to API
- **WHEN** file size approaches 1,200 lines
- **THEN** code review SHALL flag for refactoring
- **AND** SHALL suggest domain decomposition
- **AND** developer SHALL split into focused modules

### Requirement: Decomposition Testing Strategy
Decomposed modules SHALL have comprehensive tests to prevent regression.

#### Scenario: Module isolation testing
- **GIVEN** API decomposed into subpackages
- **WHEN** writing tests for each module
- **THEN** each module SHALL have dedicated `*_test.go` file
- **AND** SHALL test module handlers in isolation
- **AND** SHALL use mocked service dependencies

#### Scenario: Integration test coverage
- **GIVEN** decomposed modules work together via router
- **WHEN** running integration tests
- **THEN** SHALL verify routing to correct module handlers
- **AND** SHALL verify cross-module interactions work correctly
- **AND** Bruno API tests SHALL pass without modification

#### Scenario: Backward compatibility verification
- **GIVEN** API decomposed but external behavior unchanged
- **WHEN** running Bruno API test suite
- **THEN** all tests SHALL pass with identical response times (<300ms)
- **AND** response JSON structures SHALL be identical
- **AND** error messages SHALL be consistent

## Migration Notes
- IoT API decomposition is largest refactoring - done last after patterns proven
- Decomposition done incrementally per subpackage (devices → checkin → sessions → feedback)
- Each subpackage tested independently before integration
- Main router maintains identical URL structure - zero breaking changes
- Active Service analytics extraction via composition preserves external interface
- Large file refactoring applies cumulative savings from all previous phases
- File size limits enforced via linter rules and code review checklist
