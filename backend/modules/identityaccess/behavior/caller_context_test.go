package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The caller-context behaviour suites port the DB-backed tests of the
// dissolved modules/identityaccess/legacy/usercontext package (#3501). They
// drive the capability through services.NewUserContextTestModule, which
// composes it over the same owners as production.

// nonexistentCallerRowID names a row that no fixture creates.
const nonexistentCallerRowID int64 = 999999999

// setupCallerRows composes the caller context over db.
func setupCallerRows(t *testing.T, db *bun.DB) *repositories.CallerRows {
	t.Helper()
	module, err := services.NewUserContextTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return module.UserContext
}

// callerCtx returns a context with JWT claims for accountID in this test's
// tenant, bound to the tenant runtime of the test's own database.
func callerCtx(tb testing.TB, accountID int64) context.Context {
	return testpkg.WithTestTenantRuntime(tb, callerTenantCtx(accountID, testpkg.Tenant(tb)))
}

func callerTenantCtx(accountID, tenantID int64, roles ...string) context.Context {
	claims := jwt.AppClaims{ID: int(accountID), TenantID: tenantID, Roles: roles}
	return context.WithValue(testpkg.TenantContext(tenantID), jwt.CtxClaims, claims)
}

// ============================================================================
// Core Operations Tests
// ============================================================================

func TestCallerContext_Account(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("retrieves current user with valid token", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "CurrentUser", "Test")

		result, err := caller.Account(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Equal(t, account.ID, result.ID)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := caller.Account(context.Background())
		require.Error(t, err)
	})

	t.Run("returns error for non-existent user ID", func(t *testing.T) {
		_, err := caller.Account(callerCtx(t, nonexistentCallerRowID))
		require.Error(t, err)
	})
}

func TestCallerContext_Person(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("retrieves current person with valid token", func(t *testing.T) {
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "CurrentPerson", "Test")

		result, err := caller.Person(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Equal(t, person.ID, result.ID)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := caller.Person(context.Background())
		require.Error(t, err)
	})
}

func TestCallerContext_StaffID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("retrieves current staff with valid token", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "CurrentStaff", "Test")

		result, err := caller.StaffID(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Equal(t, staff.ID, result)
	})

	t.Run("returns error when person is not staff", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "Person")

		_, err := caller.StaffID(callerCtx(t, account.ID))

		require.Error(t, err)
	})

	t.Run("retrieves current staff for tenant custom role after permission checks", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "CustomRole", "Staff")

		ctx := callerTenantCtx(account.ID, testpkg.Tenant(t), "betreuung-plus")
		result, err := caller.StaffID(ctx)

		require.NoError(t, err)
		assert.Equal(t, staff.ID, result)
	})
}

func TestCallerContext_TeacherID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("retrieves current teacher with valid token", func(t *testing.T) {
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "CurrentTeacher", "Test")

		result, err := caller.TeacherID(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Equal(t, teacher.ID, result)
	})

	t.Run("retrieves current teacher for explicit teacher-only role", func(t *testing.T) {
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "RoleScoped", "Teacher")

		result, err := caller.TeacherID(callerTenantCtx(account.ID, testpkg.Tenant(t), "teacher"))

		require.NoError(t, err)
		assert.Equal(t, teacher.ID, result)
	})

	t.Run("returns error when staff is not teacher", func(t *testing.T) {
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "NonTeacher", "Staff")

		_, err := caller.TeacherID(callerCtx(t, account.ID))

		require.Error(t, err)
	})
}

// ============================================================================
// Group Operations Tests
// ============================================================================

func TestCallerRows_GetMyGroups(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := setupCallerRows(t, db)

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "User")

		groups, err := rows.GetMyGroups(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := rows.GetMyGroups(context.Background())
		require.Error(t, err)
	})
}

func TestCallerContext_Navigation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("returns one complete navigation projection", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Navigation", "Staff")
		group := testpkg.CreateTestEducationGroup(t, db, "Navigation Group")
		today := testpkg.TodayDate()
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, staff.ID, today, today.AddDays(1))

		result, err := caller.Navigation(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Equal(t, staff.ID, result.StaffID)
		require.Len(t, result.Groups, 1)
		assert.Equal(t, group.ID, result.Groups[0].ID)
		assert.True(t, result.Groups[0].ViaSubstitution)
		assert.Empty(t, result.SupervisedSessionIDs)
	})

	t.Run("keeps current staff null for a non-staff account", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Navigation", "NonStaff")

		result, err := caller.Navigation(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Zero(t, result.StaffID)
		assert.Empty(t, result.Groups)
		assert.Empty(t, result.SupervisedSessionIDs)
	})
}

func TestCallerContext_MyActivityGroupIDs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "Activity")

		groups, err := caller.MyActivityGroupIDs(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("returns groups for staff member", func(t *testing.T) {
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "Staff", "Activity")

		_, err := caller.MyActivityGroupIDs(callerCtx(t, account.ID))

		// May be empty if no supervisions, just verify no error.
		require.NoError(t, err)
	})
}

func TestCallerContext_MyActiveSessionIDs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "Active")

		groups, err := caller.MyActiveSessionIDs(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestCallerRows_GetMySupervisedGroups(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := setupCallerRows(t, db)

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "Supervised")

		groups, err := rows.GetMySupervisedGroups(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("returns supervised groups for staff", func(t *testing.T) {
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "Staff", "Supervised")

		_, err := rows.GetMySupervisedGroups(callerCtx(t, account.ID))

		// May be empty if no supervisions, just verify no error.
		require.NoError(t, err)
	})
}

// ============================================================================
// Profile Operations Tests
// ============================================================================

func TestCallerContext_Profile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("retrieves profile for authenticated user", func(t *testing.T) {
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "Profile", "Test")

		result, err := caller.Profile(callerCtx(t, account.ID))

		require.NoError(t, err)
		require.NotNil(t, result.Person)
		assert.Equal(t, person.FirstName, result.Person.FirstName)
		assert.Equal(t, person.LastName, result.Person.LastName)
	})

	t.Run("returns profile with fallback data for account without person", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "nolink@example.com")

		result, err := caller.Profile(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.Equal(t, account.Email, result.Account.Email)
	})

	t.Run("returns global avatar even when current tenant has no tenant profile row", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "globalavatar@example.com")

		avatarPath := "/uploads/avatars/global/account-global.jpg"
		_, err := db.ExecContext(context.Background(),
			`UPDATE auth.accounts SET avatar = ? WHERE id = ?`,
			avatarPath, account.ID)
		require.NoError(t, err)

		// A tenant the account has no profile row in.
		otherTenantID, _ := testpkg.CreateTestTenant(t, db)
		result, err := caller.Profile(callerTenantCtx(account.ID, otherTenantID))

		require.NoError(t, err)
		assert.Equal(t, avatarPath, result.Account.Avatar)
		assert.Equal(t, account.Email, result.Account.Email)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := caller.Profile(context.Background())
		require.Error(t, err)
	})
}

func TestCallerContext_UpdateProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("updates profile fields", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Update", "Profile")

		first, last := "UpdatedFirst", "UpdatedLast"
		result, err := caller.UpdateProfile(callerCtx(t, account.ID), identityaccess.CallerProfileUpdate{
			FirstName: &first,
			LastName:  &last,
		})

		require.NoError(t, err)
		require.NotNil(t, result.Person)
		assert.Equal(t, "UpdatedFirst", result.Person.FirstName)
		assert.Equal(t, "UpdatedLast", result.Person.LastName)
	})

	t.Run("updates username", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Username", "Update")

		// Unique username to avoid a duplicate key error.
		uniqueUsername := fmt.Sprintf("newusername_%d", time.Now().UnixNano())
		result, err := caller.UpdateProfile(callerCtx(t, account.ID), identityaccess.CallerProfileUpdate{
			Username: &uniqueUsername,
		})

		require.NoError(t, err)
		require.NotNil(t, result.Account.Username)
		assert.Equal(t, uniqueUsername, *result.Account.Username)
	})

	t.Run("updates bio", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Bio", "Update")

		bio := "This is my bio"
		result, err := caller.UpdateProfile(callerCtx(t, account.ID), identityaccess.CallerProfileUpdate{Bio: &bio})

		require.NoError(t, err)
		assert.Equal(t, "This is my bio", result.Bio)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := caller.UpdateProfile(context.Background(), identityaccess.CallerProfileUpdate{})
		require.Error(t, err)
	})
}

func TestCallerContext_UpdateAvatar(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("updates avatar URL", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Avatar", "Update")

		ctx := callerCtx(t, account.ID)
		avatarURL := "/uploads/avatars/global/test.jpg"

		result, err := caller.UpdateAvatar(ctx, avatarURL)

		require.NoError(t, err)
		assert.Equal(t, avatarURL, result.Account.Avatar)

		accountRecord, err := testpkg.ReadAccountState(ctx, db, account.ID)
		require.NoError(t, err)
		assert.Equal(t, avatarURL, accountRecord.Avatar)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := caller.UpdateAvatar(context.Background(), "/test.jpg")
		require.Error(t, err)
	})
}

// ============================================================================
// Error Type Tests
// ============================================================================

func TestCallerContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("CallerError contains operation details", func(t *testing.T) {
		err := &identityaccess.CallerError{
			Op:  "test operation",
			Err: identityaccess.ErrCallerNotAuthenticated,
		}

		assert.Contains(t, err.Error(), "test operation")
	})

	t.Run("CallerError unwraps inner error", func(t *testing.T) {
		innerErr := identityaccess.ErrCallerNotFound
		err := &identityaccess.CallerError{Op: "test", Err: innerErr}

		assert.Equal(t, innerErr, err.Unwrap())
	})

	t.Run("CallerGroupsPartialError contains operation and counts", func(t *testing.T) {
		err := &identityaccess.CallerGroupsPartialError{
			Op:           "partial test",
			SuccessCount: 5,
			FailureCount: 2,
			FailedIDs:    []int64{nonexistentCallerRowID, nonexistentCallerRowID + 1},
			LastErr:      identityaccess.ErrCallerGroupNotFound,
		}

		assert.Contains(t, err.Error(), "partial")
	})
}

// ============================================================================
// Helper Functions Tests
// ============================================================================

// TestMergeActiveSessions tests the session merge indirectly through
// MyActiveSessionIDs.
func TestMergeActiveSessions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("handles empty results gracefully", func(t *testing.T) {
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "Merge", "Test")

		groups, err := caller.MyActiveSessionIDs(callerCtx(t, account.ID))

		require.NoError(t, err)
		// Should return an empty slice, not nil.
		assert.NotNil(t, groups)
	})
}

// ============================================================================
// GroupStudentIDs Tests
// ============================================================================

func TestCallerContext_GroupStudentIDs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := caller.GroupStudentIDs(context.Background(), nonexistentCallerRowID)
		require.Error(t, err)
	})

	t.Run("returns error for unauthorized access to group", func(t *testing.T) {
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "NoAccess", "Staff")

		// Try to access a non-existent group.
		_, err := caller.GroupStudentIDs(callerCtx(t, account.ID), nonexistentCallerRowID)

		require.Error(t, err)
	})

	t.Run("returns students for supervised group", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Supervisor", "GroupStudents")
		activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity for Students")
		room := testpkg.CreateTestRoom(t, db, "Test Room for Students")
		student := testpkg.CreateTestStudent(t, db, "Test", "StudentInGroup", "1a")

		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "supervisor")
		// A visit so there's a student in the group.
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)

		students, err := caller.GroupStudentIDs(callerCtx(t, account.ID), activeGroup.ID)

		require.NoError(t, err)
		require.NotNil(t, students)
		assert.GreaterOrEqual(t, len(students), 1, "Should have at least 1 student")
	})
}

// ============================================================================
// GroupVisits Tests
// ============================================================================

func TestCallerContext_GroupVisits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caller := setupCallerRows(t, db).Caller()

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := caller.GroupVisits(context.Background(), nonexistentCallerRowID)
		require.Error(t, err)
	})

	t.Run("returns error for unauthorized access to group", func(t *testing.T) {
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "NoAccess", "Visits")

		// Try to access a non-existent group.
		_, err := caller.GroupVisits(callerCtx(t, account.ID), nonexistentCallerRowID)

		require.Error(t, err)
	})

	t.Run("returns visits for supervised group", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Supervisor", "GroupVisits")
		activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity for Visits")
		room := testpkg.CreateTestRoom(t, db, "Test Room for Visits")
		student := testpkg.CreateTestStudent(t, db, "Test", "StudentVisit", "1b")

		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "supervisor")

		// An active visit (no exit time) and a closed one.
		_ = testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		closedAt := time.Now().Add(-time.Hour)
		_ = testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, closedAt.Add(-time.Hour), &closedAt)

		ctx := callerCtx(t, account.ID)

		// Verify prerequisite: staff is findable via account→person→staff chain.
		var personID int64
		err := db.NewRaw(`SELECT id FROM users.persons WHERE account_id = ? AND tenant_id = ?`,
			account.ID, testpkg.Tenant(t)).Scan(context.Background(), &personID)
		require.NoError(t, err, "prerequisite: person should be findable by account_id")
		require.Equal(t, staff.Person.ID, personID, "prerequisite: person ID should match")

		var staffID int64
		err = db.NewRaw(`SELECT id FROM users.staff WHERE person_id = ? AND tenant_id = ?`,
			personID, testpkg.Tenant(t)).Scan(context.Background(), &staffID)
		require.NoError(t, err, "prerequisite: staff should be findable by person_id")
		require.Equal(t, staff.ID, staffID, "prerequisite: staff ID should match")

		visits, err := caller.GroupVisits(ctx, activeGroup.ID)

		require.NoError(t, err)
		require.NotNil(t, visits)
		assert.GreaterOrEqual(t, len(visits), 1, "Should have at least 1 active visit")
		for _, visit := range visits {
			assert.Nil(t, visit.ExitTime, "closed visits must not be exposed as active")
			assert.Equal(t, activeGroup.ID, visit.ActiveGroupID)
		}
	})
}

// ============================================================================
// Teacher Groups with Substitutions Tests
// ============================================================================

func TestCallerRows_GetMyGroups_TeacherGroups(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := setupCallerRows(t, db)

	t.Run("returns groups for teacher", func(t *testing.T) {
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Teacher", "Groups")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Teacher Class")
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)

		groups, err := rows.GetMyGroups(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 1, "Teacher should have at least 1 group")
	})

	t.Run("returns substitution groups for staff", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Substitute", "Staff")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Substitution Class")

		today := testpkg.TodayDate()
		testpkg.CreateTestGroupSubstitution(t, db, educationGroup.ID, nil, staff.ID, today, today.AddDays(1))

		groups, err := rows.GetMyGroups(callerCtx(t, account.ID))

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 1, "Substitute should have at least 1 group")
	})

	t.Run("returns groups for explicit teacher-only role", func(t *testing.T) {
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Explicit", "TeacherRole")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Teacher Role Class")
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)

		groups, err := rows.GetMyGroups(callerTenantCtx(account.ID, testpkg.Tenant(t), "teacher"))

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 1, "Teacher-only role should have at least 1 group")
	})
}

// TestParentRequestReviews_UsesRealTeacherGroupAssignments pins the review
// scope over real group-teacher and substitution rows. The old test injected
// fixed setting resolvers; the composed capability reads the tenant
// settings, so each scenario stores the values the resolver returned.
func TestParentRequestReviews_UsesRealTeacherGroupAssignments(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reviews := setupCallerRows(t, db).Caller().ParentRequestReviews
	require.NotNil(t, reviews)
	settingsModule, err := services.NewSettingsTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	settings := settingsModule.Settings

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Request", "Reviewer")
	ownGroup := testpkg.CreateTestEducationGroup(t, db, "Reviewer's Group")
	otherGroup := testpkg.CreateTestEducationGroup(t, db, "Another Group")
	activeSubstitutionGroup := testpkg.CreateTestEducationGroup(t, db, "Active Substitution Group")
	expiredSubstitutionGroup := testpkg.CreateTestEducationGroup(t, db, "Expired Substitution Group")
	testpkg.CreateTestGroupTeacher(t, db, ownGroup.ID, teacher.ID)
	today := testpkg.TodayDate()
	testpkg.CreateTestGroupSubstitution(t, db, activeSubstitutionGroup.ID, nil, teacher.StaffID, today, today.AddDays(1))
	testpkg.CreateTestGroupSubstitution(t, db, expiredSubstitutionGroup.ID, nil, teacher.StaffID, today.AddDays(-2), today.AddDays(-1))

	settingsCtx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	setSettings := func(groupLeaderEnabled bool, absenceScope string) {
		t.Helper()
		require.NoError(t, settings.SetValue(settingsCtx, settingKeyGroupLeaderReviewEnabled, groupLeaderEnabled, nil, nil))
		require.NoError(t, settings.SetValue(settingsCtx, settingKeyParentAbsenceReviewScope, absenceScope, nil, nil))
	}
	permissions := []string{"users:update"}

	// Group leaders enabled: exactly the own group and the active
	// substitution; neither the expired substitution nor a foreign group,
	// and never school-wide (a student without a group stays out).
	setSettings(true, parentAbsenceReviewScopeInherit)
	wide, ids, err := reviews.ReviewScope(callerCtx(t, account.ID), permissions)
	require.NoError(t, err)
	assert.False(t, wide)
	assert.ElementsMatch(t, []int64{ownGroup.ID, activeSubstitutionGroup.ID}, ids)
	assert.NotContains(t, ids, expiredSubstitutionGroup.ID)
	assert.NotContains(t, ids, otherGroup.ID)

	setSettings(false, parentAbsenceReviewScopeInherit)
	wide, ids, err = reviews.ReviewScope(callerCtx(t, account.ID), permissions)
	require.NoError(t, err)
	assert.False(t, wide)
	assert.NotContains(t, ids, ownGroup.ID)

	// The absence-only selection uses the same live group/substitution
	// resolver, without granting access to the other request kinds.
	setSettings(false, parentAbsenceReviewScopeGroupLeaders)
	wide, ids, err = reviews.AbsenceReviewScope(callerCtx(t, account.ID), permissions)
	require.NoError(t, err)
	assert.False(t, wide)
	assert.ElementsMatch(t, []int64{ownGroup.ID, activeSubstitutionGroup.ID}, ids)
	wide, ids, err = reviews.ReviewScope(callerCtx(t, account.ID), permissions)
	require.NoError(t, err)
	assert.False(t, wide)
	assert.Empty(t, ids)

	setSettings(false, parentAbsenceReviewScopeAllStaff)
	wide, ids, err = reviews.AbsenceReviewScope(callerCtx(t, account.ID), permissions)
	require.NoError(t, err)
	assert.True(t, wide)
	assert.Empty(t, ids)
}

// ============================================================================
// CallerGroupsPartialError Tests
// ============================================================================

func TestCallerGroupsPartialError_Unwrap(t *testing.T) {
	t.Parallel()
	innerErr := identityaccess.ErrCallerGroupNotFound
	err := &identityaccess.CallerGroupsPartialError{
		Op:           "test",
		SuccessCount: 1,
		FailureCount: 1,
		FailedIDs:    []int64{nonexistentCallerRowID},
		LastErr:      innerErr,
	}

	assert.Equal(t, innerErr, err.Unwrap())
}

// ============================================================================
// Database Error Tests
// ============================================================================

// canceledCtx triggers the database error paths: every read fails.
func canceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestCallerContext_GroupStudentIDs_DatabaseError(t *testing.T) {
	t.Parallel()
	caller := setupCallerRows(t, testpkg.SetupTestDB(t)).Caller()

	_, err := caller.GroupStudentIDs(canceledCtx(), nonexistentCallerRowID)
	require.Error(t, err)
}

func TestCallerContext_GroupVisits_DatabaseError(t *testing.T) {
	t.Parallel()
	caller := setupCallerRows(t, testpkg.SetupTestDB(t)).Caller()

	_, err := caller.GroupVisits(canceledCtx(), nonexistentCallerRowID)
	require.Error(t, err)
}

func TestCallerContext_MyActivityGroupIDs_DatabaseError(t *testing.T) {
	t.Parallel()
	caller := setupCallerRows(t, testpkg.SetupTestDB(t)).Caller()

	_, err := caller.MyActivityGroupIDs(canceledCtx())
	require.Error(t, err)
}

func TestCallerContext_MyActiveSessionIDs_DatabaseError(t *testing.T) {
	t.Parallel()
	caller := setupCallerRows(t, testpkg.SetupTestDB(t)).Caller()

	_, err := caller.MyActiveSessionIDs(canceledCtx())
	require.Error(t, err)
}

func TestCallerContext_Account_DatabaseError(t *testing.T) {
	t.Parallel()
	caller := setupCallerRows(t, testpkg.SetupTestDB(t)).Caller()

	_, err := caller.Account(canceledCtx())
	require.Error(t, err)
}

func TestCallerContext_Person_DatabaseError(t *testing.T) {
	t.Parallel()
	caller := setupCallerRows(t, testpkg.SetupTestDB(t)).Caller()

	_, err := caller.Person(canceledCtx())
	require.Error(t, err)
}

func TestCallerContext_StaffID_DatabaseError(t *testing.T) {
	t.Parallel()
	caller := setupCallerRows(t, testpkg.SetupTestDB(t)).Caller()

	_, err := caller.StaffID(canceledCtx())
	require.Error(t, err)
}

func TestCallerContext_TeacherID_DatabaseError(t *testing.T) {
	t.Parallel()
	caller := setupCallerRows(t, testpkg.SetupTestDB(t)).Caller()

	_, err := caller.TeacherID(canceledCtx())
	require.Error(t, err)
}
