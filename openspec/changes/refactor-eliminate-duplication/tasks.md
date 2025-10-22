# Implementation Tasks - Eliminate Code Duplication

## PHASE 1: API Helper Extraction (Week 1, 40 hours)

### 1.1 Error Handling Helpers (8 hours)
- [ ] 1.1.1 Create `backend/pkg/api/helpers.go` with `renderError(w, r, err)` function
- [ ] 1.1.2 Create comprehensive tests for error helper (test all error types)
- [ ] 1.1.3 Migrate `api/auth/api.go` to use error helper (108 occurrences)
- [ ] 1.1.4 Migrate `api/iot/api.go` to use error helper (179 occurrences)
- [ ] 1.1.5 Verify Bruno API tests pass after auth migration
- [ ] 1.1.6 Verify Bruno API tests pass after IoT migration

### 1.2 ID Parsing Helpers (6 hours)
- [ ] 1.2.1 Add `parseIDParam(r, paramName string) (int64, error)` to helpers
- [ ] 1.2.2 Add `requireIDParam(w, r, paramName)` variant that handles errors
- [ ] 1.2.3 Create unit tests for ID parsing (valid, invalid, missing cases)
- [ ] 1.2.4 Migrate 5 API files with highest ID parsing duplication
- [ ] 1.2.5 Run integration tests to verify ID validation behavior unchanged

### 1.3 Request Binding Helpers (8 hours)
- [ ] 1.3.1 Add generic `bindRequest[T any](r *http.Request) (*T, error)` to helpers
- [ ] 1.3.2 Add `bindAndValidate[T Validator](r)` variant for interfaces with Bind method
- [ ] 1.3.3 Create tests for request binding (valid JSON, invalid JSON, validation errors)
- [ ] 1.3.4 Migrate 3 API files to use binding helper
- [ ] 1.3.5 Verify request validation behavior unchanged (test invalid requests)

### 1.4 Pagination Helpers (6 hours)
- [ ] 1.4.1 Add `parsePagination(r *http.Request) (page, pageSize int)` to helpers
- [ ] 1.4.2 Add constants: `DefaultPage = 1`, `DefaultPageSize = 50`, `MaxPageSize = 100`
- [ ] 1.4.3 Create tests for pagination (defaults, max limits, invalid values)
- [ ] 1.4.4 Migrate all list endpoints to use pagination helper
- [ ] 1.4.5 Verify pagination behavior consistent across all list APIs

### 1.5 Additional Common Helpers (8 hours)
- [ ] 1.5.1 Add `parseIntQuery(r, paramName, defaultValue int) int` for optional int params
- [ ] 1.5.2 Add `parseBoolQuery(r, paramName string, defaultValue bool) bool`
- [ ] 1.5.3 Add `parseStringQuery(r, paramName, defaultValue string) string`
- [ ] 1.5.4 Create middleware: `WithIDParam` for automatic ID extraction to context
- [ ] 1.5.5 Create middleware: `WithPagination` for automatic pagination parsing

### 1.6 Phase 1 Validation (4 hours)
- [ ] 1.6.1 Run full test suite: `go test ./... -v -race`
- [ ] 1.6.2 Run Bruno API tests: `cd bruno && ./dev-test.sh all`
- [ ] 1.6.3 Verify test coverage on helpers: `go test -coverprofile=coverage.out ./pkg/api`
- [ ] 1.6.4 Run linter: `golangci-lint run --timeout 10m`
- [ ] 1.6.5 Measure LOC reduction: count lines eliminated vs baseline
- [ ] 1.6.6 Document helpers in `pkg/api/README.md` with usage examples

## PHASE 2: Response Type Consolidation (Week 2-3, 60 hours)

### 2.1 Response Type Library Structure (8 hours)
- [ ] 2.1.1 Create `backend/api/common/responses/` package
- [ ] 2.1.2 Define canonical `StudentResponse` (consolidate 3 versions)
- [ ] 2.1.3 Define canonical `TeacherResponse` (consolidate 2 versions)
- [ ] 2.1.4 Define canonical `SupervisorResponse` (consolidate 2 versions)
- [ ] 2.1.5 Define canonical `GroupResponse` (consolidate 2 versions)
- [ ] 2.1.6 Create response type hierarchy: `StudentSummary` vs `StudentDetail`

### 2.2 Response Mapper Framework (12 hours)
- [ ] 2.2.1 Design mapper interface: `type Mapper[From, To any] interface`
- [ ] 2.2.2 Implement reflection-based mapper for struct → struct conversion
- [ ] 2.2.3 Add automatic nil handling for optional fields
- [ ] 2.2.4 Add time conversion handling (time.Time → common.Time)
- [ ] 2.2.5 Create builder pattern for complex responses
- [ ] 2.2.6 Create comprehensive mapper tests (95% coverage target)

### 2.3 Student Response Migration (8 hours)
- [ ] 2.3.1 Update `api/students/api.go` to use `responses.StudentDetail`
- [ ] 2.3.2 Update `api/groups/api.go` to use `responses.StudentSummary`
- [ ] 2.3.3 Update `api/activities/api.go` to use `responses.StudentSummary`
- [ ] 2.3.4 Update `api/feedback/api.go` to use `responses.StudentSummary`
- [ ] 2.3.5 Verify all student endpoints return correct JSON structure
- [ ] 2.3.6 Run Bruno student API tests

### 2.4 Teacher Response Migration (6 hours)
- [ ] 2.4.1 Update `api/groups/api.go` to use `responses.TeacherResponse`
- [ ] 2.4.2 Update `api/staff/api.go` to use `responses.TeacherResponse`
- [ ] 2.4.3 Verify teacher endpoints return correct JSON
- [ ] 2.4.4 Run Bruno staff/groups API tests

### 2.5 Other Response Types Migration (12 hours)
- [ ] 2.5.1 Migrate SupervisorResponse (active, activities APIs)
- [ ] 2.5.2 Migrate GroupResponse (groups, staff APIs)
- [ ] 2.5.3 Migrate DeviceResponse (IoT API)
- [ ] 2.5.4 Migrate RoomResponse (facilities, groups APIs)
- [ ] 2.5.5 Migrate ActivityResponse (activities API)

### 2.6 Response Building Helpers (10 hours)
- [ ] 2.6.1 Create `buildListResponse[T any](items []T, mapper func(T) R) []R` helper
- [ ] 2.6.2 Create `buildPaginatedResponse(items, page, pageSize, total)` helper
- [ ] 2.6.3 Update common.Respond to work seamlessly with new response types
- [ ] 2.6.4 Create response middleware for automatic wrapping

### 2.7 Phase 2 Validation (4 hours)
- [ ] 2.7.1 Run full test suite with response type changes
- [ ] 2.7.2 Run ALL Bruno API tests (verify JSON responses unchanged)
- [ ] 2.7.3 Create API contract tests to prevent response structure drift
- [ ] 2.7.4 Verify frontend integration (smoke test with running frontend)
- [ ] 2.7.5 Measure LOC reduction: count eliminated response definitions
- [ ] 2.7.6 Document response type library in `api/common/responses/README.md`

## PHASE 3: Service Layer Base Patterns (Week 4-5, 80 hours)

### 3.1 Base Service Design (12 hours)
- [ ] 3.1.1 Create `backend/services/base/service.go` with base service struct
- [ ] 3.1.2 Define common service interface with CRUD operations
- [ ] 3.1.3 Implement standard error wrapping: `wrapError(op, err)`
- [ ] 3.1.4 Remove 11 duplicate ServiceError type definitions
- [ ] 3.1.5 Create shared error types in `services/base/errors.go`
- [ ] 3.1.6 Create comprehensive base service tests

### 3.2 Transaction Helpers (10 hours)
- [ ] 3.2.1 Create `WithTransaction(ctx, fn) error` helper in base service
- [ ] 3.2.2 Create `RunInTx[T any](ctx, fn func(ctx) (T, error)) (T, error)` generic helper
- [ ] 3.2.3 Standardize transaction error handling
- [ ] 3.2.4 Create transaction tests (commit, rollback, nested transactions)
- [ ] 3.2.5 Document transaction patterns

### 3.3 Validation Helpers (8 hours)
- [ ] 3.3.1 Create `ValidateAndCreate(ctx, entity)` pattern in base service
- [ ] 3.3.2 Create `ValidateAndUpdate(ctx, entity)` pattern
- [ ] 3.3.3 Extract common validation checks to base service
- [ ] 3.3.4 Create validation tests
- [ ] 3.3.5 Integrate with pkg/validation framework from earlier work

### 3.4 Service Migration: Auth (10 hours)
- [ ] 3.4.1 Refactor AuthService to extend base service
- [ ] 3.4.2 Replace custom error handling with base error wrappers
- [ ] 3.4.3 Replace transaction patterns with base transaction helpers
- [ ] 3.4.4 Run auth service tests
- [ ] 3.4.5 Run Bruno auth API tests

### 3.5 Service Migration: Active (12 hours)
- [ ] 3.5.1 **CRITICAL**: Merge `StartActivitySession` and `StartActivitySessionWithSupervisors`
- [ ] 3.5.2 Extract SSE broadcasting to `broadcastEventSafely(ctx, groupID, event)` helper
- [ ] 3.5.3 Extract student name lookup to `getStudentDisplayName(ctx, studentID)` helper
- [ ] 3.5.4 Extract room conflict check to `checkRoomAvailability(ctx, roomID, excludeID)` helper
- [ ] 3.5.5 Refactor ActiveService to extend base service
- [ ] 3.5.6 Run active service tests (critical - many complex tests)
- [ ] 3.5.7 Run Bruno active/sessions API tests

### 3.6 Service Migration: Others (16 hours)
- [ ] 3.6.1 Migrate Users service to base patterns
- [ ] 3.6.2 Migrate Activities service to base patterns
- [ ] 3.6.3 Migrate Education service to base patterns
- [ ] 3.6.4 Migrate IoT service to base patterns
- [ ] 3.6.5 Migrate Facilities service to base patterns
- [ ] 3.6.6 Migrate remaining services (Feedback, Config, Schedule, UserContext)

### 3.7 Phase 3 Validation (12 hours)
- [ ] 3.7.1 Run full service layer test suite
- [ ] 3.7.2 Run ALL Bruno API tests (services changed, verify behavior)
- [ ] 3.7.3 Performance benchmark: compare before/after service call latency
- [ ] 3.7.4 Verify transaction rollback behavior unchanged
- [ ] 3.7.5 Measure LOC reduction in services
- [ ] 3.7.6 Document base service patterns in `services/base/README.md`

## PHASE 4: God Class Decomposition (Month 2-3, 160 hours)

### 4.1 IoT API Decomposition (40 hours)
- [ ] 4.1.1 Create `api/iot/devices/` subpackage
- [ ] 4.1.2 Create `api/iot/checkin/` subpackage
- [ ] 4.1.3 Create `api/iot/sessions/` subpackage
- [ ] 4.1.4 Create `api/iot/feedback/` subpackage
- [ ] 4.1.5 Move device CRUD handlers to devices package (~400 lines)
- [ ] 4.1.6 Move RFID checkin logic to checkin package (~350 lines)
- [ ] 4.1.7 Move session management to sessions package (~450 lines)
- [ ] 4.1.8 Move feedback submission to feedback package (~200 lines)
- [ ] 4.1.9 Update main IoT router to mount subpackages
- [ ] 4.1.10 Apply Phase 1-3 helpers throughout decomposed modules
- [ ] 4.1.11 Run IoT integration tests
- [ ] 4.1.12 Run Bruno IoT API tests (all 30+ scenarios)
- [ ] 4.1.13 Verify final file sizes: all modules <500 lines

### 4.2 Active Service Refactoring (40 hours)
- [ ] 4.2.1 Create `services/analytics/` package
- [ ] 4.2.2 Extract dashboard analytics methods to analytics service
- [ ] 4.2.3 Extract room utilization calculations to analytics service
- [ ] 4.2.4 Extract count/metrics methods to analytics service
- [ ] 4.2.5 Update ActiveService to use analytics service via composition
- [ ] 4.2.6 Apply base service patterns to analytics service
- [ ] 4.2.7 Run active service tests
- [ ] 4.2.8 Run Bruno active/analytics API tests
- [ ] 4.2.9 Verify active_service.go reduced from 2,836 → ~1,800 lines

### 4.3 Auth API Refactoring (20 hours)
- [ ] 4.3.1 Apply all Phase 1 helpers to api/auth/api.go
- [ ] 4.3.2 Consolidate duplicate error handling (108 occurrences)
- [ ] 4.3.3 Consolidate duplicate ID parsing (33 occurrences)
- [ ] 4.3.4 Consolidate request binding patterns (12 occurrences)
- [ ] 4.3.5 Extract authentication flow helpers
- [ ] 4.3.6 Run auth API tests
- [ ] 4.3.7 Run Bruno auth tests
- [ ] 4.3.8 Verify auth/api.go reduced from 1,981 → ~1,100 lines

### 4.4 Students API Refactoring (20 hours)
- [ ] 4.4.1 Apply Phase 1 helpers (61 error renders, 11 ID parses)
- [ ] 4.4.2 Apply Phase 2 response types (StudentDetail, StudentSummary)
- [ ] 4.4.3 Extract permission checking logic to helpers
- [ ] 4.4.4 Extract privacy consent checking to helper function
- [ ] 4.4.5 Run student API tests
- [ ] 4.4.6 Run Bruno student tests
- [ ] 4.4.7 Verify students/api.go reduced from 1,725 → ~1,100 lines

### 4.5 Activities API Refactoring (20 hours)
- [ ] 4.5.1 Apply Phase 1-2 helpers and response types
- [ ] 4.5.2 Consolidate activity validation patterns
- [ ] 4.5.3 Consolidate supervisor response building
- [ ] 4.5.4 Consolidate schedule response building
- [ ] 4.5.5 Run activities tests
- [ ] 4.5.6 Run Bruno activities tests
- [ ] 4.5.7 Verify activities/api.go reduced from 1,951 → ~1,200 lines

### 4.6 Remaining APIs (20 hours)
- [ ] 4.6.1 Refactor schedules API (1,259 lines)
- [ ] 4.6.2 Refactor staff API (1,001 lines)
- [ ] 4.6.3 Refactor groups API (788 lines)
- [ ] 4.6.4 Refactor config API (756 lines)
- [ ] 4.6.5 Refactor usercontext API (637 lines)
- [ ] 4.6.6 Apply all Phase 1-3 patterns consistently

## PHASE 5: Repository Generic Enhancements (Month 4, 100 hours)

### 5.1 Enhanced Generic Repository (40 hours)
- [ ] 5.1.1 Add `ListWithOptions()` to `base.Repository[T]`
- [ ] 5.1.2 Implement automatic ModelTableExpr derivation from type name
- [ ] 5.1.3 Add comprehensive tests for generic ListWithOptions
- [ ] 5.1.4 Remove 13 duplicate ListWithOptions overrides from repositories
- [ ] 5.1.5 Verify all repository List calls work correctly
- [ ] 5.1.6 Run repository integration tests

### 5.2 Generic FindByField Methods (30 hours)
- [ ] 5.2.1 Add `FindByField(ctx, fieldName, value)` to base repository
- [ ] 5.2.2 Add `FindOneByField(ctx, fieldName, value)` variant
- [ ] 5.2.3 Implement field name validation (prevent SQL injection)
- [ ] 5.2.4 Create tests for FindByField (various types, nil handling)
- [ ] 5.2.5 Migrate 20 simple FindByForeignKey methods to use FindByField
- [ ] 5.2.6 Keep complex query methods custom (30 remain custom)
- [ ] 5.2.7 Run integration tests for all migrated finders

### 5.3 Automatic Validation Hooks (16 hours)
- [ ] 5.3.1 Add Validator interface detection to base.Repository Create/Update
- [ ] 5.3.2 Auto-call entity.Validate() before insert/update
- [ ] 5.3.3 Create tests for validation hooks (valid, invalid, no validator)
- [ ] 5.3.4 Remove 15 duplicate Create/Update validation wrappers
- [ ] 5.3.5 Verify all entity validations still execute

### 5.4 Error Wrapping Utilities (8 hours)
- [ ] 5.4.1 Create `wrapDBError(op, err)` utility in base repository
- [ ] 5.4.2 Update all base repository methods to use wrapper
- [ ] 5.4.3 Remove 395 manual DatabaseError wrapping calls from repositories
- [ ] 5.4.4 Verify error unwrapping works correctly (errors.Is, errors.As)

### 5.5 Phase 5 Validation (6 hours)
- [ ] 5.5.1 Run ALL repository tests: `go test ./database/repositories/... -v -race`
- [ ] 5.5.2 Run Bruno API tests (repositories changed, verify behavior)
- [ ] 5.5.3 Measure LOC reduction in repository layer: target 3,500 lines eliminated
- [ ] 5.5.4 Verify test coverage on base.Repository: target 90%+
- [ ] 5.5.5 Document enhanced generic repository patterns

## PHASE 6: Migration Framework Improvements (Month 4, 60 hours - CAN RUN IN PARALLEL)

### 6.1 Migration Code Generator (24 hours)
- [ ] 6.1.1 Create `database/migrations/generator/` package
- [ ] 6.1.2 Implement filename parser (extract version, description from `001003005_users_students.go`)
- [ ] 6.1.3 Implement boilerplate generator (Version, Description, init() registration)
- [ ] 6.1.4 Auto-generate transaction defer pattern for up/down functions
- [ ] 6.1.5 Create generator tests (parse various filename formats)
- [ ] 6.1.6 Generate boilerplate for 5 test migrations, verify correctness

### 6.2 Common Migration Helpers (16 hours)
- [ ] 6.2.1 Create `CreateUpdatedAtTrigger(tableName string) string` helper
- [ ] 6.2.2 Create `CreateForeignKey(table, column, refTable, refColumn)` helper
- [ ] 6.2.3 Create `CreateIndex(table, columns, unique)` helper
- [ ] 6.2.4 Create `DropTableCascade(tableName)` helper
- [ ] 6.2.5 Create tests for all migration helpers
- [ ] 6.2.6 Migrate 10 existing migrations to use helpers as proof-of-concept

### 6.3 Rollback Auto-Generation (12 hours)
- [ ] 6.3.1 Implement SQL parser to detect CREATE TABLE statements
- [ ] 6.3.2 Auto-generate DROP TABLE rollback from CREATE statements
- [ ] 6.3.3 Handle complex migrations (require manual rollback)
- [ ] 6.3.4 Create tests for rollback generation
- [ ] 6.3.5 Apply to 10 simple migrations as proof-of-concept

### 6.4 Dependency Graph Validation (6 hours)
- [ ] 6.4.1 Create compile-time dependency validator
- [ ] 6.4.2 Detect circular dependencies in migration graph
- [ ] 6.4.3 Detect missing dependencies
- [ ] 6.4.4 Compute and visualize dependency tree
- [ ] 6.4.5 Integrate into CI: fail build on dependency issues

### 6.5 Phase 6 Validation (2 hours)
- [ ] 6.5.1 Run migration tests: `go run main.go migrate validate`
- [ ] 6.5.2 Test generator on 5 new migrations
- [ ] 6.5.3 Verify helpers work correctly (up/down cycles)
- [ ] 6.5.4 Measure boilerplate reduction: target 3,876 lines eliminated
- [ ] 6.5.5 Document migration framework usage

## FINAL VALIDATION (12 hours)

### 7.1 Comprehensive Testing
- [ ] 7.1.1 Run FULL backend test suite: `go test ./... -v -race -coverprofile=coverage.out`
- [ ] 7.1.2 Run ALL Bruno API tests: `cd bruno && ./dev-test.sh all`
- [ ] 7.1.3 Verify test coverage increased: target 45%+ overall (was 15%)
- [ ] 7.1.4 Run linter across entire codebase: `golangci-lint run --timeout 10m`
- [ ] 7.1.5 Verify zero linting warnings

### 7.2 Performance Validation
- [ ] 7.2.1 Benchmark API response times before/after
- [ ] 7.2.2 Verify Bruno test suite still completes in <300ms
- [ ] 7.2.3 Benchmark service layer operation latency
- [ ] 7.2.4 Benchmark repository layer operation latency
- [ ] 7.2.5 Verify no performance regression >5% in any layer

### 7.3 Metrics Collection
- [ ] 7.3.1 Count total LOC: target 58,026 (was 73,726) - 15,700 line reduction!
- [ ] 7.3.2 Count remaining duplicate code: target <300 lines (was 15,700)
- [ ] 7.3.3 Count god classes: target 0 (was 6)
- [ ] 7.3.4 Count response types: target 15 (was 69)
- [ ] 7.3.5 Count repository methods: measure generic vs custom ratio
- [ ] 7.3.6 Measure migration boilerplate reduction
- [ ] 7.3.7 Generate comprehensive code quality report

### 7.4 Documentation
- [ ] 7.4.1 Update CLAUDE.md with all new patterns
- [ ] 7.4.2 Document API helper usage in `pkg/api/README.md`
- [ ] 7.4.3 Document response type library in `api/common/responses/README.md`
- [ ] 7.4.4 Document base service patterns in `services/base/README.md`
- [ ] 7.4.5 Document enhanced repository patterns in `database/repositories/base/README.md`
- [ ] 7.4.6 Document migration framework in `database/migrations/generator/README.md`
- [ ] 7.4.7 Create comprehensive migration guide for future development
- [ ] 7.4.8 Update code review checklist with all new patterns

## Success Criteria (UPDATED FOR 6 PHASES)
- ✅ All 341 duplicate error renders eliminated
- ✅ All 86 duplicate ID parsers eliminated
- ✅ All 69 duplicate request binders eliminated
- ✅ 69 response types consolidated to 15 canonical types
- ✅ 1,826 manual nil checks automated via mapper
- ✅ 11 duplicate ServiceError types eliminated
- ✅ 57 duplicate transaction patterns standardized
- ✅ 6 god classes decomposed to <500 lines per module
- ✅ 13 duplicate ListWithOptions eliminated
- ✅ 50 FindByForeignKey methods replaced with generic FindByField
- ✅ 395 duplicate error wraps eliminated
- ✅ 57 migration files with auto-generated boilerplate
- ✅ 40+ updated_at triggers use helper function
- ✅ **Total LOC reduced by 15,700 lines (21.3%)**
- ✅ Test coverage increased from 15% to 45%+
- ✅ All Bruno API tests pass (<300ms)
- ✅ Zero linting warnings
- ✅ No performance regression >5%
