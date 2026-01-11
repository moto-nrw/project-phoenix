# Issue #572 - Phase 1 Service Layer Tests Progress

**Date**: 2026-01-11 (Updated)
**Branch**: `test/service-layer-tests`
**Status**: Phase 1 Infrastructure Blockers Resolved

## Current Coverage

| Service | Coverage | Target | Status |
|---------|----------|--------|--------|
| ActivityService | 77.0% | 80% | Close - 3% gap |
| Users Service (Person + Guardian) | ~55-60% (estimated) | 80% | Improved with PIN + invitation tests |

## Session 2 Progress (2026-01-11)

### Infrastructure Fixes Completed

1. **BUN ORM Bug Fixed** (`database/repositories/auth/guardian_invitation.go`)
   - Changed `Order()` to `OrderExpr()` on lines 123, 142, 161
   - Fixes "missing FROM-clause entry for table" errors
   - `GetPendingInvitations` test no longer skipped

2. **PIN Test Fixture Created** (`test/fixtures.go`)
   - Added `CreateTestStaffWithPIN(tb, db, firstName, lastName, pin)`
   - Creates staff with account and hashed PIN (Argon2id)
   - Enables testing PIN validation flows

3. **Public Test Mailers Created** (`test/mailers.go`)
   - `CapturingMailer` - records sent messages for verification
   - `FlakyMailer` - simulates failures for retry testing
   - `FailingMailer` - always fails for error handling tests
   - Enables testing guardian invitation email flows

### New Tests Added

#### PersonService PIN Validation Tests (~6 tests)
- `TestPersonService_ValidateStaffPIN_Success` - validates correct PIN
- `TestPersonService_ValidateStaffPIN_WrongPIN` - rejects incorrect PIN
- `TestPersonService_ValidateStaffPIN_NoPINSet` - handles staff without PIN
- `TestPersonService_ValidateStaffPINForSpecificStaff_Success`
- `TestPersonService_ValidateStaffPINForSpecificStaff_WrongPIN`
- `TestPersonService_ValidateStaffPINForSpecificStaff_NoPINSet`

#### GuardianService Invitation Email Tests (~4 tests)
- `TestGuardianService_SendInvitation_SendsEmail` - verifies email dispatch
- `TestGuardianService_SendInvitation_GuardianNotFound` - error handling
- `TestGuardianService_SendInvitation_NoEmail` - validates CanInvite()
- `TestGuardianService_SendInvitation_DuplicatePending` - prevents duplicates

### Files Modified

| File | Changes |
|------|---------|
| `database/repositories/auth/guardian_invitation.go` | Fixed Order → OrderExpr |
| `test/fixtures.go` | Added CreateTestStaffWithPIN |
| `test/mailers.go` | New file - test mailer helpers |
| `services/users/person_service_test.go` | Added PIN validation tests |
| `services/users/guardian_service_test.go` | Added invitation tests, removed skip |

## Blockers Resolved

| Blocker | Resolution |
|---------|------------|
| BUN ORM alias bug | Fixed `Order()` → `OrderExpr()` |
| PIN validation untestable | Created `CreateTestStaffWithPIN` fixture |
| Mailer not mockable | Created public `test.CapturingMailer` |
| GetPendingInvitations skipped | Removed skip (bug fixed) |

## Remaining Gaps

### Still at 0% Coverage (Invitation Acceptance Flow)
- `ValidateInvitation`
- `AcceptInvitation`
- `validateInvitationAcceptRequest`
- `validateInvitationAndProfile`
- `validateInvitationStatus`
- `createGuardianAccountFromInvitation`
- `finalizeInvitationAcceptance`

**Note**: These require creating invitations and then testing the acceptance flow. The infrastructure is now in place to add these tests.

### Test DB Migration
- `person_guardians` table may still be missing in test DB
- Run: `docker compose --profile test up -d postgres-test`
- Then: `APP_ENV=test go run main.go migrate reset`

## Commands to Run Tests

```bash
# Start test database
docker compose --profile test up -d postgres-test

# Reset test DB migrations
APP_ENV=test go run main.go migrate reset

# Run all Phase 1 service tests
go test ./services/users/... ./services/activities/... -cover

# Run with verbose output
go test ./services/users/... -v

# Generate coverage report
go test ./services/users/... -coverprofile=coverage_users.out
go tool cover -func=coverage_users.out | head -50
```

## Test Patterns Added

### PIN Validation Test Pattern
```go
func TestPersonService_ValidateStaffPIN_Success(t *testing.T) {
    db := testpkg.SetupTestDB(t)
    defer func() { _ = db.Close() }()

    service := setupPersonService(t, db)
    ctx := context.Background()

    t.Run("validates correct PIN and returns staff", func(t *testing.T) {
        // ARRANGE - create staff with known PIN
        testPIN := "1234"
        staff, _ := testpkg.CreateTestStaffWithPIN(t, db, "PIN", "Test", testPIN)
        defer testpkg.CleanupActivityFixtures(t, db, staff.PersonID)

        // ACT
        result, err := service.ValidateStaffPIN(ctx, testPIN)

        // ASSERT
        require.NoError(t, err)
        require.NotNil(t, result)
        assert.Equal(t, staff.ID, result.ID)
    })
}
```

### Guardian Invitation Email Test Pattern
```go
func TestGuardianService_SendInvitation_SendsEmail(t *testing.T) {
    db := testpkg.SetupTestDB(t)
    defer func() { _ = db.Close() }()

    mailer := testpkg.NewCapturingMailer()
    service := setupGuardianServiceWithMailer(t, db, mailer)
    ctx := context.Background()

    t.Run("sends invitation email to guardian", func(t *testing.T) {
        // ... create guardian with email ...

        // ACT - send invitation
        invitation, err := service.SendInvitation(ctx, req)

        // ASSERT
        require.NoError(t, err)

        // Wait for async email dispatch
        emailSent := mailer.WaitForMessages(1, 500*time.Millisecond)
        assert.True(t, emailSent, "Expected invitation email to be sent")

        if emailSent {
            msgs := mailer.Messages()
            assert.Equal(t, "Einladung zum Eltern-Portal", msgs[0].Subject)
        }
    })
}
```

## Next Steps

1. **Run Tests with Test DB** - Verify all tests pass with live database
2. **Check Coverage** - Measure actual coverage after new tests
3. **Add Invitation Acceptance Tests** - Use capturing mailer pattern
4. **Consider Phase 2** - If Users Service reaches 80%, move on

## Summary

All infrastructure blockers from Session 1 have been resolved. The codebase now has:
- Fixed repository bug
- PIN test fixture
- Public test mailers
- New test coverage for PIN validation and invitation flows

Ready for test execution once Docker Desktop is available.
