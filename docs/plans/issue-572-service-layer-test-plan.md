# Issue #572: Service Layer Test Coverage Plan

**Epic**: #569 - Increase Test Coverage to 80%+
**Target**: ≥80% coverage across all service packages
**Approach**: Hermetic integration tests with real database (no mocks)

---

## Current State Analysis

### Test Coverage Summary (as of 2025-01-10)

| Service Package | Current Coverage | Lines of Code | Test Files | Priority |
|-----------------|------------------|---------------|------------|----------|
| `services/active` | 17.7% | ~2,650 | 5 | HIGH |
| `services/auth` | 28.8% | ~1,816 | 6 | HIGH |
| `services/education` | 37.8% | ~800 | 1 | MEDIUM |
| `services/import` | 32.8% | ~2,648 | 3 | MEDIUM |
| `services/scheduler` | 8.9% | ~634 | 1 | LOW |
| `services/users` | 0% | ~1,479 | 0 | **CRITICAL** |
| `services/activities` | 0% | ~1,469 | 0 | **CRITICAL** |
| `services/iot` | 0% | ~395 | 0 | HIGH |
| `services/facilities` | 0% | ~483 | 0 | MEDIUM |
| `services/schedule` | 0% | ~617 | 0 | MEDIUM |
| `services/config` | 0% | ~662 | 0 | LOW |
| `services/feedback` | 0% | ~268 | 0 | LOW |
| `services/usercontext` | 0% | ~935 | 0 | LOW |
| `services/database` | 0% | ~169 | 0 | LOW |

### Key Observations

1. **9 services have ZERO test coverage** - these are the primary targets
2. **4 services have partial coverage** (17-38%) - need expansion
3. **1 service has minimal coverage** (scheduler at 8.9%)
4. **Existing test patterns are excellent** - session_conflict_test.go is the reference

---

## Testing Strategy

### Hermetic Test Pattern (Reference: `session_conflict_test.go`)

```go
package active_test  // External test package

import testpkg "github.com/moto-nrw/project-phoenix/test"

func TestFeature(t *testing.T) {
    // Setup
    db := testpkg.SetupTestDB(t)
    defer func() { _ = db.Close() }()

    service := setupService(t, db)  // Create via factory
    ctx := context.Background()

    t.Run("scenario name", func(t *testing.T) {
        // ARRANGE: Create fixtures
        fixture := testpkg.CreateTestXxx(t, db, "name")
        defer testpkg.CleanupActivityFixtures(t, db, fixture.ID)

        // ACT: Call service method
        result, err := service.DoSomething(ctx, fixture.ID)

        // ASSERT: Verify results
        require.NoError(t, err)
        assert.NotNil(t, result)
    })
}
```

### Service Setup Pattern

```go
func setupService(t *testing.T, db *bun.DB) ServiceType {
    repoFactory := repositories.NewFactory(db)
    serviceFactory, err := services.NewFactory(repoFactory, db)
    require.NoError(t, err)
    return serviceFactory.ServiceName
}
```

---

## Implementation Plan

### Phase 1: Critical Priority (0% → 80%+)

#### 1.1 Users Service (`services/users/`) - CRITICAL

**PersonService** (22 methods):
| Method | Test Scenarios | Estimated Tests |
|--------|----------------|-----------------|
| `Get` | Valid ID, not found, invalid ID | 3 |
| `GetByIDs` | Multiple valid, partial found, empty | 3 |
| `Create` | Valid person, duplicate, validation error | 3 |
| `Update` | Valid update, not found, validation | 3 |
| `Delete` | Existing, not found, with relationships | 3 |
| `List` | With filters, pagination, empty | 3 |
| `FindByTagID` | Found, not found, case sensitivity | 3 |
| `FindByAccountID` | Found, not found | 2 |
| `FindByName` | Exact match, partial, none | 3 |
| `LinkToAccount` | Valid link, already linked, invalid | 3 |
| `UnlinkFromAccount` | Valid unlink, not linked | 2 |
| `LinkToRFIDCard` | Valid, card already used | 2 |
| `UnlinkFromRFIDCard` | Valid, not linked | 2 |
| `GetFullProfile` | With all relations, partial | 2 |
| `FindByGuardianID` | Found, none | 2 |
| `ListAvailableRFIDCards` | Some available, none | 2 |
| `ValidateStaffPIN` | Valid, invalid, account locked | 3 |
| `ValidateStaffPINForSpecificStaff` | Valid, wrong staff | 2 |
| `GetStudentsByTeacher` | Has students, none | 2 |
| `GetStudentsWithGroupsByTeacher` | With groups | 2 |

**File**: `services/users/person_service_test.go` (~50 tests)

**GuardianService** (22 methods):
| Method | Test Scenarios | Estimated Tests |
|--------|----------------|-----------------|
| `CreateGuardian` | Valid, duplicate email, validation | 3 |
| `CreateGuardianWithInvitation` | Valid, email send | 2 |
| `GetGuardianByID` | Found, not found | 2 |
| `GetGuardianByEmail` | Found, not found | 2 |
| `UpdateGuardian` | Valid, not found | 2 |
| `DeleteGuardian` | Existing, with relationships | 2 |
| `SendInvitation` | Valid, already invited, no email | 3 |
| `ValidateInvitation` | Valid token, expired, used | 3 |
| `AcceptInvitation` | Valid, password mismatch, weak pwd | 3 |
| `GetStudentGuardians` | Has guardians, none | 2 |
| `GetGuardianStudents` | Has students, none | 2 |
| `LinkGuardianToStudent` | Valid, duplicate | 2 |
| `GetStudentGuardianRelationship` | Found, not found | 2 |
| `UpdateStudentGuardianRelationship` | Valid update | 1 |
| `RemoveGuardianFromStudent` | Valid, not linked | 2 |
| `ListGuardians` | With filters, pagination | 2 |
| `GetGuardiansWithoutAccount` | Some, none | 2 |
| `GetInvitableGuardians` | Some, none | 2 |
| `GetPendingInvitations` | Some, none | 2 |
| `CleanupExpiredInvitations` | Some expired, none | 2 |

**File**: `services/users/guardian_service_test.go` (~45 tests)

---

#### 1.2 Activities Service (`services/activities/`) - CRITICAL

**ActivityService** (30+ methods):
| Category | Methods | Test Scenarios | Estimated Tests |
|----------|---------|----------------|-----------------|
| **Category ops** | Create, Get, Update, Delete, List | CRUD + validation | 10 |
| **Group ops** | Create, Get, Update, Delete, List, GetWithDetails | CRUD + relations | 12 |
| **Schedule ops** | Add, Get, GetByGroup, Delete, Update | Schedule management | 10 |
| **Supervisor ops** | Add, Get, GetByGroup, Delete, SetPrimary, Update, GetStaffAssignments, UpdateGroupSupervisors | Supervisor management | 15 |
| **Enrollment ops** | Enroll, Unenroll, UpdateEnrollments, GetEnrolled, GetStudentEnrollments, GetAvailable, UpdateStatus, GetByDate, GetHistory | Student enrollment | 18 |
| **Public ops** | GetPublicGroups, GetPublicCategories, GetOpenGroups | Public access | 6 |
| **Device ops** | GetTeacherTodaysActivities | Device integration | 3 |

**File**: `services/activities/activity_service_test.go` (~75 tests)

---

### Phase 2: High Priority (Expand/Create)

#### 2.1 IoT Service (`services/iot/`) - 0% → 80%+

| Method | Test Scenarios | Estimated Tests |
|--------|----------------|-----------------|
| **Core CRUD** | Create, Get, Update, Delete, List | 10 |
| **Status ops** | UpdateStatus, Ping | 4 |
| **Filtered lookups** | ByType, ByStatus, ByRegisteredBy | 6 |
| **Monitoring** | Active, Maintenance, Offline, Stats | 8 |
| **Network ops** | DetectNew, ScanNetwork | 4 |
| **Auth ops** | GetByAPIKey | 3 |

**File**: `services/iot/iot_service_test.go` (~35 tests)

---

#### 2.2 Active Service (`services/active/`) - 17.7% → 80%+

**Current test files cover**: Session conflicts, attendance, daily cleanup, dashboard analytics, timeouts

**Missing coverage** (methods not tested):
| Category | Methods to Add | Estimated Tests |
|----------|----------------|-----------------|
| **Active Group CRUD** | Create, Update, Delete, List, FindBy... | 15 |
| **Visit CRUD** | Create, Update, Delete, List, FindBy... | 15 |
| **Supervisor ops** | Create, Update, Delete, List, FindBy... | 12 |
| **Combined Group ops** | All 9 methods | 18 |
| **Group Mapping** | Add, Remove, GetBy... | 8 |
| **Dynamic supervisors** | UpdateActiveGroupSupervisors | 3 |
| **Analytics** | GetActiveGroupsCount, GetTotalVisitsCount, etc. | 10 |
| **Unclaimed groups** | GetUnclaimed, Claim | 5 |

**New files**:
- `services/active/active_group_service_test.go` (~30 tests)
- `services/active/visit_service_test.go` (~30 tests)
- `services/active/supervisor_service_test.go` (~20 tests)
- `services/active/combined_group_test.go` (~25 tests)

---

#### 2.3 Auth Service (`services/auth/`) - 28.8% → 80%+

**Current test files cover**: Invitation, password reset, rate limiting, race conditions

**Missing coverage**:
| Category | Methods | Estimated Tests |
|----------|---------|-----------------|
| **Login/Register** | Login, LoginWithAudit, Register | 10 |
| **Token ops** | ValidateToken, RefreshToken, Logout | 10 |
| **Role CRUD** | Create, Get, Update, Delete, List, Assign/Remove | 15 |
| **Permission CRUD** | Create, Get, Update, Delete, List, Grant/Deny/Remove | 18 |
| **Account mgmt** | Activate, Deactivate, Update, List, GetByRole | 10 |
| **Token cleanup** | CleanupExpired, RevokeAll, GetActive | 6 |
| **Parent accounts** | Create, Get, Update, Activate, Deactivate, List | 12 |

**New files**:
- `services/auth/login_service_test.go` (~20 tests)
- `services/auth/role_service_test.go` (~20 tests)
- `services/auth/permission_service_test.go` (~20 tests)
- `services/auth/account_service_test.go` (~15 tests)

---

### Phase 3: Medium Priority

#### 3.1 Facilities Service (`services/facilities/`) - 0% → 80%+

| Category | Methods | Estimated Tests |
|----------|---------|-----------------|
| **Room CRUD** | Get, Create, Update, Delete, List | 10 |
| **Lookups** | FindByName, FindByBuilding, Category, Floor | 8 |
| **Availability** | CheckAvailability, GetAvailable | 5 |
| **Analytics** | GetUtilization, GetHistory | 4 |
| **Lists** | GetBuildingList, GetCategoryList | 4 |

**File**: `services/facilities/facility_service_test.go` (~31 tests)

---

#### 3.2 Schedule Service (`services/schedule/`) - 0% → 80%+

| Category | Methods | Estimated Tests |
|----------|---------|-----------------|
| **Dateframe ops** | CRUD + FindByDate, FindOverlapping | 10 |
| **Timeframe ops** | CRUD + FindActive, FindByRange | 10 |
| **Recurrence ops** | CRUD + FindByFrequency, FindByWeekday | 10 |
| **Advanced** | GenerateEvents, CheckConflict, FindSlots, GetCurrent | 8 |

**File**: `services/schedule/schedule_service_test.go` (~38 tests)

---

#### 3.3 Education Service (`services/education/`) - 37.8% → 80%+

Review existing `education_service_test.go` and expand:
- Add edge cases for existing tests
- Test teacher substitution flows
- Test group assignment/unassignment

**File**: Expand `services/education/education_service_test.go` (~20 additional tests)

---

#### 3.4 Import Service (`services/import/`) - 32.8% → 80%+

Review existing tests and expand:
- Test XLSX parser scenarios
- Test import conflict resolution
- Test batch processing edge cases

**Files**: Expand existing test files (~25 additional tests)

---

### Phase 4: Lower Priority

#### 4.1 Config Service (`services/config/`) - 0% → 80%+

| Method | Test Scenarios | Estimated Tests |
|--------|----------------|-----------------|
| Get, Create, Update, Delete | CRUD operations | 8 |
| GetByKey, ListByCategory | Lookups | 4 |
| Bulk operations | Batch get/set | 4 |

**File**: `services/config/config_service_test.go` (~16 tests)

---

#### 4.2 Feedback Service (`services/feedback/`) - 0% → 80%+

| Method | Test Scenarios | Estimated Tests |
|--------|----------------|-----------------|
| Create, Get, Update, Delete, List | CRUD | 10 |
| GetByUser, GetByCategory | Lookups | 4 |

**File**: `services/feedback/feedback_service_test.go` (~14 tests)

---

#### 4.3 Scheduler Service (`services/scheduler/`) - 8.9% → 80%+

| Method | Test Scenarios | Estimated Tests |
|--------|----------------|-----------------|
| RunCleanupJobs | All cleanup jobs | 8 |
| StartScheduler, StopScheduler | Lifecycle | 4 |
| Individual job execution | Each job type | 6 |

**File**: Expand `services/scheduler/scheduler_test.go` (~15 additional tests)

---

#### 4.4 UserContext Service (`services/usercontext/`) - 0% → 80%+

| Method | Test Scenarios | Estimated Tests |
|--------|----------------|-----------------|
| GetUserContext | Different user types | 6 |
| Permission checks | Access control | 5 |
| Profile data | With/without relations | 4 |

**File**: `services/usercontext/usercontext_service_test.go` (~15 tests)

---

#### 4.5 Database Service (`services/database/`) - 0% → 80%+

| Method | Test Scenarios | Estimated Tests |
|--------|----------------|-----------------|
| GetDatabaseStats | Valid stats | 3 |
| GetTableSizes | All tables | 2 |
| Health checks | Connection status | 2 |

**File**: `services/database/database_service_test.go` (~7 tests)

---

## Required Test Fixtures

### New Fixtures Needed

| Fixture | Used By | Notes |
|---------|---------|-------|
| `CreateTestDateframe(t, db)` | schedule | With date range |
| `CreateTestTimeframe(t, db)` | schedule | With time range |
| `CreateTestRecurrenceRule(t, db)` | schedule | Weekly pattern |
| `CreateTestActivitySchedule(t, db, groupID)` | activities | Schedule with group |
| `CreateTestPlannedSupervisor(t, db, groupID, staffID)` | activities | Supervisor assignment |
| `CreateTestStudentEnrollment(t, db, groupID, studentID)` | activities | Enrollment record |
| `CreateTestConfig(t, db, key, value)` | config | System setting |
| `CreateTestFeedback(t, db, userID)` | feedback | Feedback entry |
| `CreateTestCombinedGroup(t, db)` | active | Combined group |

### Existing Fixtures (Available)

- `CreateTestPerson`, `CreateTestStudent`, `CreateTestStaff`, `CreateTestTeacher`
- `CreateTestStudentWithAccount`, `CreateTestTeacherWithAccount`
- `CreateTestActivityGroup`, `CreateTestActivityCategory`
- `CreateTestRoom`, `CreateTestDevice`
- `CreateTestEducationGroup`, `CreateTestGroupTeacher`
- `CreateTestActiveGroup`, `CreateTestVisit`, `CreateTestAttendance`
- `CreateTestAccount`, `CreateTestRole`, `CreateTestPermission`
- `CreateTestGuardianProfile`, `CreateTestGuest`
- `CleanupActivityFixtures`, `CleanupAuthFixtures`

---

## Execution Order

### Week 1-2: Critical Services (0% coverage)
1. **Users PersonService** - 50 tests
2. **Users GuardianService** - 45 tests
3. **Activities Service** - 75 tests

### Week 3: High Priority Services
4. **IoT Service** - 35 tests
5. **Active Service expansion** - 105 tests
6. **Auth Service expansion** - 75 tests

### Week 4: Medium Priority
7. **Facilities Service** - 31 tests
8. **Schedule Service** - 38 tests
9. **Education Service expansion** - 20 tests
10. **Import Service expansion** - 25 tests

### Week 5: Lower Priority
11. **Config Service** - 16 tests
12. **Feedback Service** - 14 tests
13. **Scheduler Service expansion** - 15 tests
14. **UserContext Service** - 15 tests
15. **Database Service** - 7 tests

---

## Estimated Effort Summary

| Phase | New Tests | Coverage Impact |
|-------|-----------|-----------------|
| Phase 1 (Critical) | ~170 | +25% overall |
| Phase 2 (High) | ~215 | +20% overall |
| Phase 3 (Medium) | ~114 | +10% overall |
| Phase 4 (Lower) | ~67 | +5% overall |
| **Total** | **~566** | **80%+ target** |

---

## Verification Commands

```bash
# Run all service tests
go test ./services/... -v

# Check coverage
go test ./services/... -cover

# Generate coverage report
go test ./services/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run specific package
go test ./services/users/... -v -cover

# Run with race detection
go test -race ./services/...
```

---

## Success Criteria

1. **All service packages have test files**
2. **Each service package ≥80% coverage**
3. **All tests pass with `go test ./services/...`**
4. **No mocks used** - hermetic tests with real DB only
5. **All error paths covered** (not found, validation, conflicts)
6. **Edge cases included** (empty results, boundary conditions)
7. **Transaction tests** included where applicable

---

## Notes

- Tests should run in ~5-10 seconds per file with test DB
- Each test creates and cleans up its own fixtures
- Use `t.Parallel()` where safe (no shared state)
- Document complex business logic in test comments
- Follow existing naming conventions from `session_conflict_test.go`
