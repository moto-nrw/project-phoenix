# Eliminate Code Duplication - Technical Design

## Context

### Investigation Summary
Deep-dive analysis using 10 parallel investigator agents across 1,498 backend functions revealed:
- **15,700 lines of duplicate code** (21.3% of 73,726-line codebase!)
- **~40% of all 1,498 functions are duplicates or near-duplicates**
- Duplication across 5 major categories: API patterns, Response mapping, Service patterns, Repository layer, Migration boilerplate

### Business Impact
- **$460,000/year** cost in slower development, bugs, and maintenance
- 50% velocity loss from constant copy-paste synchronization
- 22 bugs/month from validation inconsistencies and logic drift
- 480 hours/year fixing same bug in 5+ locations
- 21.3% of codebase is pure duplication (15,700 wasteful lines)

### Root Causes
1. **No abstraction for common patterns**: Each API file reimplements error handling, ID parsing, request binding
2. **Copy-paste development culture**: New APIs created by duplicating existing files
3. **Missing shared response types**: 69 response types with duplicates (StudentResponse × 3 versions)
4. **No base service class**: 11 services each redefine identical error types and patterns
5. **Underutilized generic repository**: Base exists but 3,500 lines still duplicated (13 ListWithOptions overrides, 50 FindBy methods, 395 error wraps)
6. **Manual migration boilerplate**: 57 files × 68 lines = 3,876 lines of copy-pasted registration/transaction code
7. **Go's verbosity**: Error handling requires explicit checks, leading to boilerplate

## Goals / Non-Goals

### Goals
1. **Eliminate 15,700 lines of duplicate code** (98% reduction to <300 lines)
2. **Create reusable API helpers** for error handling, ID parsing, request binding, pagination
3. **Consolidate response types** from 69 to 15 canonical types with hierarchy
4. **Standardize service patterns** via base service with common error/transaction handling
5. **Enhance generic repository** to eliminate 3,500 lines of repository duplication
6. **Auto-generate migration boilerplate** to eliminate 3,900 lines (49.6% of migration code)
7. **Decompose god classes** from 6 files >2000 lines to 0
8. **Increase test coverage** from 15% to 45%+
9. **Maintain 100% backward compatibility** - zero breaking changes

### Non-Goals
1. **Complete test coverage**: Not targeting 80% (future work)
2. **Performance optimization**: Focus is correctness, not speed
3. **Frontend changes**: Frontend unaffected, APIs maintain identical behavior
4. **Database refactoring**: No schema or data model changes
5. **Full code generation**: Using reflection/helpers, not full codegen framework

## Decisions

### Decision 1: Four-Phase Approach
**Choice**: Phased rollout (Helpers → Responses → Services → Decomposition)

**Rationale**:
- **Risk mitigation**: Each phase independently testable and deployable
- **Value delivery**: Phase 1 delivers $120K savings in 1 week
- **Dependencies**: Later phases build on earlier (responses use helpers)
- **Team capacity**: Allows parallel work and gradual adoption

**Phases**:
1. Week 1 (40hrs): API Helpers - eliminates 2,000 lines, $120K/year
2. Week 2-3 (60hrs): Response Types - eliminates 2,000 lines, $80K/year
3. Week 4-5 (80hrs): Service Base - eliminates 1,200 lines, $50K/year
4. Month 2-3 (160hrs): God Classes - eliminates 3,100 lines, enables future work

**Alternatives Considered**:
- **Big bang**: Too risky, hard to test, no incremental value
- **File-by-file**: Too slow, no shared infrastructure until end

### Decision 2: Helper Function Library (Not Middleware-Only)
**Choice**: `pkg/api/helpers.go` with standalone functions + optional middleware

**Rationale**:
- **Flexibility**: Helpers can be called directly or via middleware
- **Migration path**: Existing code can adopt gradually
- **Testability**: Helpers easier to unit test than middleware
- **Clarity**: Explicit helper calls more readable than implicit middleware magic

**API Design**:
```go
// Core helpers
func renderError(w http.ResponseWriter, r *http.Request, err render.Renderer)
func parseIDParam(r *http.Request, paramName string) (int64, error)
func bindRequest[T any](r *http.Request) (*T, error)
func parsePagination(r *http.Request) (page, pageSize int)

// Optional middleware
func WithIDParam(next http.Handler) http.Handler
func WithPagination(next http.Handler) http.Handler
```

**Alternatives Considered**:
- **Middleware-only**: Forces adoption, less flexible
- **Interface-based**: Over-engineering for simple helpers

### Decision 3: Reflection-Based Response Mapper
**Choice**: Reflection with struct tags for mapping rules

**Rationale**:
- **Reduces boilerplate**: Eliminates 1,826 manual nil checks
- **Type-safe**: Compile-time checking of struct field types
- **Maintainable**: Mapping rules in struct tags, not separate code
- **Performance acceptable**: Reflection overhead <1ms per response

**Example**:
```go
type StudentDetail struct {
    ID        int64  `map:"id"`
    FirstName string `map:"first_name"`
    LastName  string `map:"last_name"`
    Email     *string `map:"email,omitempty"`
}

mapper.Map(student, &response) // Automatic mapping with nil handling
```

**Alternatives Considered**:
- **Code generation**: More complex, requires build step, harder to debug
- **Manual mappers**: Keep existing duplication (rejected)

### Decision 4: Base Service with Composition (Not Inheritance)
**Choice**: Base service struct with embedded composition

**Rationale**:
- **Go idiom**: Composition over inheritance in Go
- **Flexibility**: Services can choose which base methods to expose
- **No forced interface**: Services keep their specific interfaces
- **Gradual adoption**: Can mix old and new patterns during migration

**Pattern**:
```go
type BaseService struct {
    db *bun.DB
    txHandler *base.TxHandler
}

func (s *BaseService) wrapError(op string, err error) error {
    return &ServiceError{Op: op, Err: err}
}

func (s *BaseService) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
    return s.txHandler.RunInTx(ctx, fn)
}

// Concrete service embeds base
type AuthService struct {
    *BaseService
    // auth-specific fields
}
```

**Alternatives Considered**:
- **Interface-based inheritance**: Too rigid, forces all services to same interface
- **Utility package only**: Miss opportunity for shared state (db, txHandler)

### Decision 7: Enhanced Generic Repository Pattern
**Choice**: Extend existing `base.Repository[T]` with advanced generic methods (ListWithOptions, FindByField, auto-validation)

**Rationale**:
- **Already have foundation**: 39 repositories use base.Repository[T], proven pattern
- **Fill gaps**: Add missing methods that force 13 repositories to override
- **Type safety**: Generics provide compile-time type checking
- **Eliminate 3,500 lines**: 13 ListWithOptions (260), 50 FindBy methods (900), 395 error wraps (1,580), 15 validation wrappers (300)
- **Backward compatible**: Existing repositories gain new methods automatically

**Enhanced API**:
```go
type Repository[T any] struct {
    db *bun.DB
    tableName string
    alias string
}

// NEW: Generic list with query options
func (r *Repository[T]) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]T, error)

// NEW: Generic find by any field
func (r *Repository[T]) FindByField(ctx context.Context, field string, value any) ([]T, error)
func (r *Repository[T]) FindOneByField(ctx context.Context, field string, value any) (*T, error)

// ENHANCED: Auto-validation in Create/Update
func (r *Repository[T]) Create(ctx context.Context, entity *T) error {
    if validator, ok := any(entity).(Validator); ok {
        if err := validator.Validate(); err != nil {
            return err
        }
    }
    return r.insert(ctx, entity)
}
```

**Alternatives Considered**:
- **Keep current approach**: Leaves 3,500 lines of duplication (rejected)
- **Full ORM abstraction**: Too complex, loses BUN's type safety (rejected)
- **Code generation**: More complex than generics for this use case

### Decision 8: Migration Code Generation (Not Full Framework Migration)
**Choice**: Generate boilerplate while keeping existing BUN migration structure

**Rationale**:
- **Minimal disruption**: Existing migrations continue working
- **Targeted savings**: 3,876 lines of pure boilerplate (49.6% of migration code)
- **Keep SQL control**: Developers still write SQL, not abstracted away
- **Proven patterns**: Codify existing patterns into generator

**What Gets Generated**:
```go
// AUTO-GENERATED from filename: 001003005_users_students.go
const (
    Version     = "1.3.5"                    // From filename prefix
    Description = "users students"            // From filename suffix
)

var Dependencies = []string{/* parsed from metadata */}

func init() {
    // AUTO-GENERATED registration
    Migrations.MustRegister(up, down, bun.WithPackageName("users_students"))
}

func up(ctx context.Context, db *bun.DB) error {
    // Developer writes SQL here
    _, err := db.ExecContext(ctx, `CREATE TABLE...`)
    return err
}

func down(ctx context.Context, db *bun.DB) error {
    // AUTO-GENERATED for simple cases, manual for complex
    _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS users.students CASCADE`)
    return err
}
```

**Helpers for Common Patterns**:
```go
// Instead of writing trigger SQL 40+ times:
sql := migrations.CreateUpdatedAtTrigger("users.students")

// Instead of manual foreign key SQL:
sql := migrations.CreateForeignKey("users.students", "person_id", "users.persons", "id", "CASCADE", "CASCADE")
```

**Alternatives Considered**:
- **Full declarative framework** (like Alembic, Liquibase): Too opinionated, lose SQL control
- **Schema-first generation** (from Go structs): Complex, changes workflow significantly
- **Keep manual**: Leaves 3,900 lines of duplication (rejected)

### Decision 5: Merge Duplicate Session Methods
**Choice**: Single `StartActivitySession` with optional supervisors parameter

**Rationale**:
- **90% identical code**: Only difference is supervisor handling
- **Simpler API**: One method instead of two
- **Easier to maintain**: Changes in one place
- **Backward compatible**: Old method signatures deprecated but not removed immediately

**Signature**:
```go
func (s *ActiveService) StartActivitySession(
    ctx context.Context,
    activityID int64,
    deviceID int64,
    roomID *int64,
    supervisors []SupervisorAssignment, // Empty slice = no supervisors
) (*ActiveGroup, error)
```

### Decision 6: IoT API Decomposition Strategy
**Choice**: Domain-based subpackages (devices, checkin, sessions, feedback)

**Rationale**:
- **Clear separation**: Each subdomain has focused responsibility
- **Reduced file size**: 3,063 lines → 4 files of ~400 lines each
- **Independent testing**: Each module testable in isolation
- **Easier onboarding**: Developers can understand one domain at a time

**Structure**:
```
api/iot/
├── resource.go (main router, 100 lines)
├── devices/
│   ├── handlers.go (CRUD operations, 400 lines)
│   ├── types.go (request/response types)
│   └── router.go (device routes)
├── checkin/
│   ├── handlers.go (RFID checkin logic, 350 lines)
│   ├── types.go
│   └── router.go
├── sessions/
│   ├── handlers.go (session management, 450 lines)
│   ├── types.go
│   └── router.go
└── feedback/
    ├── handlers.go (feedback submission, 200 lines)
    ├── types.go
    └── router.go
```

**Alternatives Considered**:
- **Technical layers**: Split by concern (validation, db, etc.) - less clear
- **Keep monolithic**: Rejected - too hard to maintain

## Risks / Trade-offs

### Risk 1: Helper adoption resistance
**Likelihood**: Medium | **Impact**: Medium

**Mitigation**:
- Provide clear examples in 5+ API files
- Document in CLAUDE.md with before/after
- Code review checklist enforces helper usage
- Linter rules (future) flag duplicate patterns

**Trade-off**: Incremental adoption means duplication persists temporarily (acceptable)

### Risk 2: Response type changes break frontend
**Likelihood**: Low | **Impact**: High

**Mitigation**:
- API contract tests verify JSON structure unchanged
- Gradual rollout per API
- Frontend smoke testing before deployment
- Rollback plan tested

**Trade-off**: Some response field order may change (acceptable, JSON unordered)

### Risk 3: Reflection performance overhead
**Likelihood**: Low | **Impact**: Low

**Mitigation**:
- Benchmark mapper performance (<1ms acceptable)
- Cache reflection metadata
- Abort if >5% performance regression

**Trade-off**: Slight CPU overhead for massive maintainability gain (acceptable)

### Risk 4: Base service complexity
**Likelihood**: Medium | **Impact**: Medium

**Mitigation**:
- Keep base service focused (error, transaction, validation only)
- Reject creep - no business logic in base
- Document what belongs in base vs concrete service
- Code review gates

**Trade-off**: Some patterns still service-specific (acceptable)

### Risk 5: God class decomposition too complex
**Likelihood**: Medium | **Impact**: Medium

**Mitigation**:
- Phase 4 last - can defer if needed
- Comprehensive testing before/after each decomposition
- Incremental migration per subpackage
- Rollback plan per module

**Trade-off**: Takes 2-3 months for full decomposition (acceptable given complexity)

## Migration Plan

### Phase 1: API Helpers (Week 1)
**Day 1-2**: Create helpers + tests
**Day 3-4**: Migrate 2 largest APIs (auth, iot)
**Day 5**: Validation + documentation

**Rollback**: Remove helper package, revert 2 API files (2 hours)

### Phase 2: Response Types (Week 2-3)
**Week 2**: Create response library + mapper
**Week 3**: Migrate all APIs incrementally

**Rollback**: Revert to old response types per API (4 hours)

### Phase 3: Service Base (Week 4-5)
**Week 4**: Create base service + migrate 3 services
**Week 5**: Migrate remaining 8 services

**Rollback**: Remove base service, revert each service (6 hours)

### Phase 4: God Classes (Month 2-3)
**Incremental per file**: IoT → Active → Auth → Students → Activities → Others

**Rollback**: Revert decomposed file, restore monolith (2 hours per file)

## Success Metrics

### Immediate (Week 1)
- ✅ 2,000 lines eliminated (API helpers)
- ✅ 341 error renders → 1 helper
- ✅ 86 ID parsers → 1 helper
- ✅ Bruno tests pass <300ms

### Month 1
- ✅ 4,000 lines eliminated (helpers + responses)
- ✅ 69 response types → 15
- ✅ Test coverage 15% → 25%

### Month 2
- ✅ 5,200 lines eliminated (+ services)
- ✅ 11 ServiceError types → 1
- ✅ Test coverage 25% → 35%

### Month 3
- ✅ 8,300 lines eliminated (complete)
- ✅ 6 god classes → 0
- ✅ Test coverage 35% → 40%+
- ✅ Bug rate 18/month → 6/month

## Open Questions

### Q1: Should we use code generation for mappers?
**Status**: No, use reflection

**Reasoning**: Reflection simpler, performance acceptable, no build complexity

### Q2: Should we enforce helper usage via linter immediately?
**Status**: No, future work

**Reasoning**: Need examples and adoption first, linter after 3-6 months

### Q3: Should base service be interface or struct?
**Status**: Struct with composition

**Reasoning**: More flexible, Go idiom, easier to extend

### Q4: Should we break this into multiple proposals?
**Status**: Single proposal with 4 phases

**Reasoning**: Phases interdependent, easier to track as one initiative

## References

- Technical Debt Analysis (2025-01-22): Comprehensive 10-agent investigation
- Deep-dive reports on each API file (attached in investigation results)
- BUN ORM patterns: CLAUDE.md
- Bruno API tests: Integration test suite
