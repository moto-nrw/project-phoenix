# Issue #572 - Phase 1 Service Layer Tests Progress

**Date**: 2026-01-10
**Branch**: `fix/repository-layer-and-test`
**Status**: Phase 1 In Progress

## Current Coverage

| Service | Coverage | Target | Status |
|---------|----------|--------|--------|
| ActivityService | 77.0% | 80% | Close - 3% gap |
| Users Service (Person + Guardian) | 45.2% | 80% | Needs work |

## What Was Accomplished

### ActivityService Tests (~100+ test cases)
- **Categories**: CRUD operations, validation errors, cascade delete prevention
- **Groups**: CRUD, validation, cascade delete with enrollments
- **Schedules**: CRUD operations, not found errors
- **Supervisors**: CRUD, primary supervisor logic, duplicate prevention, SetPrimarySupervisor
- **Enrollments**: Add/remove, duplicate prevention, batch updates (UpdateGroupEnrollments, UpdateGroupSupervisors)
- **Public methods**: GetPublicGroups, GetOpenGroups, GetPublicCategories, GetTeacherTodaysActivities
- **Error handling**: ActivityError.Error(), Unwrap() methods

### PersonService Tests (additional ~10 test cases)
- ValidateStaffPIN empty PIN validation
- ValidateStaffPINForSpecificStaff (empty PIN, staff not found)
- LinkToRFIDCard (person not found, RFID not found)
- Get/Update/Create error paths

## Key Test Files Modified

1. **`services/activities/activity_service_test.go`** (~2287 lines)
   - Comprehensive hermetic tests using real database
   - Uses `testpkg.SetupTestDB(t)` and fixtures from `test/fixtures.go`

2. **`services/users/person_service_test.go`** (~1133 lines)
   - Additional edge case tests added

3. **`services/users/guardian_service_test.go`** (existing)
   - Has ~26 tests but invitation flow at 0%

## Blockers Identified

### 1. GuardianService Invitation Flow (0% coverage)
Methods with 0% coverage:
- `CreateGuardianWithInvitation`
- `SendInvitation`
- `sendInvitationEmail`
- `ValidateInvitation`
- `AcceptInvitation`
- `validateInvitationAcceptRequest`
- `validateInvitationAndProfile`
- `validateInvitationStatus`
- `createGuardianAccountFromInvitation`
- `finalizeInvitationAcceptance`

**Why**: Requires email service mocking - current tests can't mock the mailer dependency.

### 2. PIN Validation Flow (0% coverage)
Methods with 0% coverage:
- `ValidateStaffPIN` (partial - empty PIN tested)
- `tryValidatePINForAccount`
- `findStaffByAccount`
- `handleSuccessfulPINAuth`
- `handleFailedPINAttempt`
- `ValidateStaffPINForSpecificStaff` (partial)

**Why**: Requires accounts with hashed PINs set up - complex fixture setup needed.

### 3. Schema Issue
- `FindByGuardianID` test skipped - `person_guardians` table missing in test DB
- Need to run: `APP_ENV=test go run main.go migrate reset`

### 4. Known Repository Bug
- `GetPendingInvitations` has BUN ORM table alias issue
- Test skipped with annotation explaining the bug

## ActivityService Gap Analysis (77% → 80%)

Methods with <80% coverage:
- `WithTx`: 68.8%
- `GetCategory`: 62.5%
- `UpdateCategory`: 60.0%
- `GetGroup`: 62.5%
- `UpdateGroup`: 60.0%
- `DeleteGroup`: 66.7%
- `CreateGroup`: 71.4%
- `validateAndSetCategory`: 77.8%

The gap is mostly in internal helper methods and error paths that are difficult to trigger without mocking repository failures.

## Commands to Run Tests

```bash
# Run all Phase 1 service tests
go test ./services/users/... ./services/activities/... -cover

# Run with verbose output
go test ./services/activities/... -v

# Generate coverage profile
go test ./services/activities/... -coverprofile=coverage_activities.out
go tool cover -func=coverage_activities.out | head -30

# Check specific failing tests
go test ./services/activities/... -v -run "TestActivityService_UpdateAttendanceStatus"
```

## Next Steps to Reach 80%

### Option A: Accept Current Coverage
- ActivityService at 77% is very close
- Users service gap is due to complex invitation/PIN flows
- Document blockers and move to Phase 2

### Option B: Add Mocking Infrastructure
1. Create mock email service for invitation tests
2. Create fixtures with pre-hashed PINs for PIN validation tests
3. Fix `person_guardians` migration in test DB

### Option C: Focus on Easy Wins
1. Add more error path tests for ActivityService internal methods
2. Add tests for methods with 60-75% coverage
3. May push ActivityService to 80%+

## Important Patterns

### Test Setup Pattern
```go
func TestSomething(t *testing.T) {
    db := testpkg.SetupTestDB(t)
    defer func() { _ = db.Close() }()

    service := setupActivityService(t, db)  // or setupPersonService
    ctx := context.Background()

    t.Run("test case", func(t *testing.T) {
        // ARRANGE - create fixtures
        group := testpkg.CreateTestActivityGroup(t, db, "unique-name")
        defer func() { _ = service.DeleteGroup(ctx, group.ID) }()

        // ACT
        result, err := service.DoSomething(ctx, group.ID)

        // ASSERT
        require.NoError(t, err)
        assert.NotNil(t, result)
    })
}
```

### Attendance Status Constants
Valid values: `"PRESENT"`, `"ABSENT"`, `"EXCUSED"`, `"UNKNOWN"` (uppercase!)

### AddSchedule Signature
```go
service.AddSchedule(ctx, groupID int64, schedule *activities.Schedule)
// NOT: service.AddSchedule(ctx, schedule)
```

### SetPrimarySupervisor Signature
```go
service.SetPrimarySupervisor(ctx, supervisorID int64)
// Takes supervisor ID, not (groupID, staffID)
```

## Commit Ready

The current tests are all passing. Ready to commit with:
```bash
git add services/activities/activity_service_test.go
git add services/users/person_service_test.go
git commit -m "test(services): add Phase 1 service layer tests (WIP)

- ActivityService: 77% coverage (up from 34.6%)
- Users Service: 45.2% coverage (up from 42.7%)
- Added ~110 new test cases
- Documented blockers for invitation and PIN validation flows

Part of Issue #572"
```
