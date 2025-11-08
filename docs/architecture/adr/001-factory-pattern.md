# ADR-001: Factory Pattern for Dependency Injection

**Status:** Accepted **Date:** 2024-06-09 **Updated:** 2025-10-19 **Deciders:**
Backend Team **Impact:** High

## Context

Go does not have a built-in dependency injection (DI) framework like Java's
Spring or .NET's DI container. We needed a way to:

1. Wire up dependencies between layers (API → Service → Repository)
2. Make testing easier by allowing mock injection
3. Avoid global state and service locators
4. Maintain compile-time type safety
5. Keep the codebase simple and explicit

### Alternatives Considered

| Approach             | Pros                                                                                               | Cons                                                            | Decision   |
| -------------------- | -------------------------------------------------------------------------------------------------- | --------------------------------------------------------------- | ---------- |
| **Factory Pattern**  | ✅ Explicit wiring<br>✅ Compile-time safety<br>✅ No reflection overhead<br>✅ Easy to understand | ⚠️ Verbose<br>⚠️ Manual wiring                                  | **CHOSEN** |
| **Wire (Google)**    | ✅ Code generation<br>✅ Compile-time safety                                                       | ❌ Learning curve<br>❌ Build complexity<br>❌ Magic            | Rejected   |
| **Uber Fx**          | ✅ Reflection-based DI<br>✅ Less boilerplate                                                      | ❌ Runtime errors<br>❌ Performance overhead<br>❌ Complex      | Rejected   |
| **Global Variables** | ✅ Simple                                                                                          | ❌ Hard to test<br>❌ Hidden dependencies<br>❌ Not thread-safe | Rejected   |
| **Service Locator**  | ✅ Centralized                                                                                     | ❌ Hidden dependencies<br>❌ Hard to test<br>❌ Runtime errors  | Rejected   |

## Decision

**Use the Factory Pattern for all dependency injection.**

### Implementation

**Repository Factory** (`database/repositories/factory.go`):

```go
type Factory struct {
    // Auth domain
    Account        auth.AccountRepository
    Token          auth.TokenRepository
    Role           auth.RoleRepository
    Permission     auth.PermissionRepository

    // Users domain
    Person         users.PersonRepository
    Staff          users.StaffRepository
    Teacher        users.TeacherRepository
    Student        users.StudentRepository

    // Education domain
    Group          education.GroupRepository
    GroupTeacher   education.GroupTeacherRepository

    // ... 40+ repositories total
}

func NewFactory(db *bun.DB) *Factory {
    return &Factory{
        Account:      auth.NewAccountRepository(db),
        Token:        auth.NewTokenRepository(db),
        Person:       users.NewPersonRepository(db),
        Teacher:      users.NewTeacherRepository(db),
        Student:      users.NewStudentRepository(db),
        Group:        education.NewGroupRepository(db),
        GroupTeacher: education.NewGroupTeacherRepository(db),
        // ... initialize all repositories
    }
}
```

**Service Factory** (`services/factory.go`):

```go
type Factory struct {
    Auth      auth.Service
    Education education.Service
    Users     users.Service
    Active    active.Service
    IoT       iot.Service
    // ... all domain services
}

func NewFactory(repos *repositories.Factory, db *bun.DB) (*Factory, error) {
    // Education service needs multiple repositories
    educationService := education.NewService(
        repos.Group,
        repos.GroupTeacher,
        repos.GroupSubstitution,
        repos.Room,
        repos.Teacher,
        repos.Staff,
        db,
    )

    // Auth service
    authService := auth.NewService(
        repos.Account,
        repos.Token,
        repos.Role,
        repos.Permission,
        db,
    )

    return &Factory{
        Education: educationService,
        Auth:      authService,
        // ... initialize all services
    }, nil
}
```

**API Initialization** (`api/base.go`):

```go
func New(enableCORS bool) (*API, error) {
    // Database connection
    db, err := database.DBConn()
    if err != nil {
        return nil, err
    }

    // Repository factory
    repoFactory := repositories.NewFactory(db)

    // Service factory
    serviceFactory, err := services.NewFactory(repoFactory, db)
    if err != nil {
        return nil, err
    }

    // API resources
    api := &API{
        Services: serviceFactory,
    }

    // Initialize domain APIs
    api.Groups = groupsAPI.NewResource(
        api.Services.Education,
        api.Services.Active,
        api.Services.Users,
    )

    api.Students = studentsAPI.NewResource(
        api.Services.Users,
        api.Services.Education,
    )

    // ... initialize all API resources

    return api, nil
}
```

### Testing with Factories

**Mock Repository Factory**:

```go
func TestGroupService_CreateGroup(t *testing.T) {
    // Create test database
    db := setupTestDB(t)
    defer cleanupTestDB(db)

    // Create repository factory
    repoFactory := repositories.NewFactory(db)

    // Or use mocks
    mockGroupRepo := &mocks.MockGroupRepository{}
    mockGroupRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

    // Create service with mock
    service := education.NewService(
        mockGroupRepo,  // Mock instead of real repository
        repoFactory.GroupTeacher,
        repoFactory.GroupSubstitution,
        repoFactory.Room,
        repoFactory.Teacher,
        repoFactory.Staff,
        db,
    )

    // Test the service
    err := service.CreateGroup(ctx, &education.Group{Name: "Test Group"})
    assert.NoError(t, err)
    mockGroupRepo.AssertExpectations(t)
}
```

## Consequences

### Positive

1. **Compile-Time Safety**
   - Missing dependencies cause build errors, not runtime panics
   - Type mismatches caught immediately
   - Refactoring is safer (change signatures → compilation errors guide fixes)

2. **Testability**
   - Easy to inject mocks for unit testing
   - Can create test-specific factories
   - Clear what dependencies a service needs

3. **Explicit Dependencies**
   - No hidden service locator pattern
   - Easy to see what each service depends on
   - Dependency graph is visible in code

4. **No Magic**
   - No reflection (better performance)
   - No code generation (simpler build process)
   - No framework learning curve

5. **Debuggability**
   - Stack traces are clear
   - No proxy objects or generated code
   - Easy to step through in debugger

### Negative

1. **Boilerplate**
   - Lots of manual wiring code
   - Factory files can become large (40+ repositories)
   - Repetitive initialization code

2. **Maintenance Overhead**
   - Every new dependency requires factory updates
   - Easy to forget to wire up new repositories
   - Refactoring factories is tedious

3. **Circular Dependencies**
   - Not prevented at compile time
   - Must be detected manually
   - Can cause initialization failures

### Mitigations

**For Boilerplate**:

- Group repositories by domain in factory
- Use clear naming conventions
- Add comments for complex dependencies

**For Maintenance**:

- Lint rule: Every repository must be in factory (future)
- Factory tests to verify all repositories initialized
- Documentation of factory pattern

**For Circular Dependencies**:

- Enforce layered architecture (API → Service → Repository)
- Code review checks
- Dependency graph visualization tool (future)

## Experience Report

**After 6 months of use:**

✅ **What worked well:**

- Compile-time safety caught many errors early
- Testing became much easier with mock injection
- New developers understood the pattern quickly
- Refactoring confidence increased

⚠️ **What was challenging:**

- Repository factory file grew to 100+ lines
- Forgetting to wire new repositories happened occasionally
- Circular dependency detection required manual review

❌ **What we would change:**

- Nothing major; the benefits outweigh the boilerplate

## Related Decisions

- [ADR-006: Repository Pattern](006-repository-pattern.md) - Defines what
  factories wire together
- [ADR-003: BUN ORM](003-bun-orm.md) - ORM passed to repositories via factory

## References

- [Dependency Injection in Go](https://www.reddit.com/r/golang/comments/8hhg7u/whats_the_best_way_to_do_dependency_injection_in/)
- [Google Wire](https://github.com/google/wire) - Alternative we considered
- [Uber Fx](https://github.com/uber-go/fx) - Reflection-based alternative
