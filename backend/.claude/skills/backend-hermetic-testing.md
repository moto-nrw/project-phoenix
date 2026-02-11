# Backend Hermetic Testing Skill

Use this skill when writing Go integration tests that access the database. Ensures tests are isolated, repeatable, and don't leave stale data.

## Core Principle: Hermetic Tests

**Hermetic tests** are self-contained and isolated - they create their own data, clean up after themselves, and can run in any order without affecting other tests.

## Critical Pattern: Defer BEFORE Assertions

**THE #1 MISTAKE**: Placing `defer` cleanup AFTER `require.NoError()`.

```go
// WRONG - Non-hermetic! Defer never runs if assertion fails
account, err := service.Register(ctx, input)
require.NoError(t, err)  // ← If this fails, t.FailNow() is called
defer testpkg.CleanupAuthFixtures(t, db, account.ID)  // ← NEVER REACHED

// CORRECT - Hermetic! Cleanup always runs
account, err := service.Register(ctx, input)
if account != nil {
    defer testpkg.CleanupAuthFixtures(t, db, account.ID)
}
require.NoError(t, err)
```

**Why this matters:**
- `require.NoError()` calls `t.FailNow()` on failure
- `t.FailNow()` stops test execution immediately
- Any code after the assertion (including `defer`) is never reached
- Stale data accumulates, causing "already exists" errors on next run

## Test Setup Pattern

```go
import testpkg "github.com/moto-nrw/project-phoenix/test"

func TestMyFeature(t *testing.T) {
    // 1. Setup database connection
    db := testpkg.SetupTestDB(t)  // Note: uppercase S
    defer func() { _ = db.Close() }()

    // 2. Create fixtures with cleanup BEFORE assertions
    student, err := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
    if student != nil {
        defer testpkg.CleanupStudentFixtures(t, db, student.ID)
    }
    require.NoError(t, err)

    // 3. Run your test
    result, err := myService.DoSomething(ctx, student.ID)
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

## Available Test Fixtures

| Function | Creates | Cleanup Function |
|----------|---------|------------------|
| `CreateTestStudent(t, db, first, last, class)` | Person + Student | `CleanupStudentFixtures` |
| `CreateTestStaff(t, db, first, last)` | Person + Staff | `CleanupStaffFixtures` |
| `CreateTestTeacherWithAccount(t, db, first, last)` | Teacher + Staff + Person + Account | `CleanupTeacherFixtures` |
| `CreateTestActivityGroup(t, db, name)` | Category + Activity | `CleanupActivityFixtures` |
| `CreateTestRoom(t, db, name)` | Facilities Room | `CleanupRoomFixtures` |
| `CreateTestDevice(t, db, deviceID)` | IoT Device | `CleanupDeviceFixtures` |
| `CreateTestAccount(t, db, staffID, username, email)` | Auth Account | `CleanupAuthFixtures` |

## BUN ORM Cleanup Pattern

When writing custom cleanup code, **DO NOT** use `Model((*any)(nil))`:

```go
// WRONG - BUN errors with "Model(nil interface *interface {})"
db.NewDelete().
    Model((*any)(nil)).
    Table("auth.accounts").
    Where("id = ?", id).
    Exec(ctx)

// CORRECT - Just use Table() and Where()
db.NewDelete().
    Table("auth.accounts").
    Where("id = ?", id).
    Exec(ctx)
```

## Subtest Pattern

For table-driven tests, each subtest needs its own cleanup:

```go
func TestRegister(t *testing.T) {
    db := testpkg.SetupTestDB(t)
    defer func() { _ = db.Close() }()

    tests := []struct {
        name     string
        input    RegisterInput
        wantErr  bool
    }{
        {"valid registration", RegisterInput{...}, false},
        {"duplicate email", RegisterInput{...}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create test-specific fixtures
            account, err := service.Register(ctx, tt.input)

            // Cleanup BEFORE assertions
            if account != nil {
                defer testpkg.CleanupAuthFixtures(t, db, account.ID)
            }

            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.NotNil(t, account)
            }
        })
    }
}
```

## Running Tests

```bash
# Start test database (port 5433)
docker compose --profile test up -d postgres-test

# Setup test database schema
APP_ENV=test go run main.go migrate reset

# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Run specific package
go test ./services/auth/...

# Clear cache and run (for debugging flaky tests)
go clean -testcache && go test ./...

# Run with verbose output
go test -v ./services/auth/...
```

## Debugging Flaky Tests

If tests pass once but fail on second run:

1. **Check defer placement** - Is cleanup after assertions?
2. **Check cleanup functions** - Are they actually deleting data?
3. **Check UNIQUE constraints** - Is stale data triggering uniqueness errors?
4. **Add logging** - Use `t.Logf()` to trace cleanup execution

```go
// Debug cleanup execution
defer func() {
    t.Logf("Cleaning up account ID: %d", account.ID)
    testpkg.CleanupAuthFixtures(t, db, account.ID)
    t.Logf("Cleanup complete")
}()
```

## Common Mistakes

| Mistake | Symptom | Fix |
|---------|---------|-----|
| Defer after assertion | "already exists" on second run | Move defer before `require.NoError` |
| Hardcoded IDs | "no rows" errors | Use fixtures that return real IDs |
| Missing cleanup | Data accumulates | Add defer cleanup for all created entities |
| Wrong database | Tests skip silently | Ensure postgres-test is running on 5433 |
| Shared state | Tests affect each other | Each test creates its own fixtures |

## Checklist for New Tests

- [ ] Uses `testpkg.SetupTestDB(t)` for database connection
- [ ] Defer cleanup placed BEFORE any assertions
- [ ] Nil check before defer (for error cases)
- [ ] Uses fixtures instead of hardcoded IDs
- [ ] Each subtest has its own cleanup
- [ ] No shared mutable state between tests
- [ ] Works with `go clean -testcache && go test ./...`
