package usercontext_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupUserContextService creates a user context service with real database connection.
func setupUserContextService(t *testing.T, db *bun.DB) usercontextSvc.UserContextService {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	repos := usercontextSvc.UserContextRepositories{
		AccountRepo:        repoFactory.Account,
		PersonRepo:         repoFactory.Person,
		StaffRepo:          repoFactory.Staff,
		TeacherRepo:        repoFactory.Teacher,
		StudentRepo:        repoFactory.Student,
		EducationGroupRepo: repoFactory.Group,
		ActivityGroupRepo:  repoFactory.ActivityGroup,
		ActiveGroupRepo:    repoFactory.ActiveGroup,
		VisitsRepo:         repoFactory.ActiveVisit,
		SupervisorRepo:     repoFactory.GroupSupervisor,
		ProfileRepo:        repoFactory.Profile,
		SubstitutionRepo:   repoFactory.GroupSubstitution,
	}

	return usercontextSvc.NewUserContextServiceWithRepos(repos, db, slog.Default())
}

// contextWithClaims creates a context with JWT claims
func contextWithClaims(userID int) context.Context {
	claims := jwt.AppClaims{
		ID: userID,
	}
	return context.WithValue(context.Background(), jwt.CtxClaims, claims)
}

// ============================================================================
// Core Operations Tests
// ============================================================================

func TestUserContextService_GetCurrentUser(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("retrieves current user with valid token", func(t *testing.T) {
		// ARRANGE - Create a test account
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "CurrentUser", "Test")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		result, err := service.GetCurrentUser(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, account.ID, result.ID)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		// ACT
		_, err := service.GetCurrentUser(context.Background())

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent user ID", func(t *testing.T) {
		// ARRANGE
		ctx := contextWithClaims(999999999)

		// ACT
		_, err := service.GetCurrentUser(ctx)

		// ASSERT
		require.Error(t, err)
	})
}

func TestUserContextService_GetCurrentPerson(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("retrieves current person with valid token", func(t *testing.T) {
		// ARRANGE - Create a test person with account
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "CurrentPerson", "Test")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		result, err := service.GetCurrentPerson(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, person.ID, result.ID)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		// ACT
		_, err := service.GetCurrentPerson(context.Background())

		// ASSERT
		require.Error(t, err)
	})
}

func TestUserContextService_GetCurrentStaff(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("retrieves current staff with valid token", func(t *testing.T) {
		// ARRANGE - Create a test staff with account
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "CurrentStaff", "Test")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		result, err := service.GetCurrentStaff(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, staff.ID, result.ID)
	})

	t.Run("returns error when person is not staff", func(t *testing.T) {
		// ARRANGE - Create a person without staff record
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "Person")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		_, err := service.GetCurrentStaff(ctx)

		// ASSERT
		require.Error(t, err)
	})
}

func TestUserContextService_GetCurrentTeacher(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("retrieves current teacher with valid token", func(t *testing.T) {
		// ARRANGE - Create a test teacher with account
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "CurrentTeacher", "Test")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		result, err := service.GetCurrentTeacher(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, teacher.ID, result.ID)
	})

	t.Run("returns error when staff is not teacher", func(t *testing.T) {
		// ARRANGE - Create a staff without teacher record
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "NonTeacher", "Staff")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		_, err := service.GetCurrentTeacher(ctx)

		// ASSERT
		require.Error(t, err)
	})
}

// ============================================================================
// Group Operations Tests
// ============================================================================

func TestUserContextService_GetMyGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		// ARRANGE - Create a person without staff record
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "User")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMyGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		// ACT
		_, err := service.GetMyGroups(context.Background())

		// ASSERT
		require.Error(t, err)
	})
}

func TestUserContextService_GetMyActivityGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		// ARRANGE - Create a person without staff record
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "Activity")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMyActivityGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("returns groups for staff member", func(t *testing.T) {
		// ARRANGE - Create a staff member
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "Staff", "Activity")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMyActivityGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		// May be empty if no supervisions, just verify no error
		_ = groups
	})
}

func TestUserContextService_GetMyActiveGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		// ARRANGE - Create a person without staff record
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "Active")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMyActiveGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestUserContextService_GetMySupervisedGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		// ARRANGE - Create a person without staff record
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "NonStaff", "Supervised")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMySupervisedGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("returns supervised groups for staff", func(t *testing.T) {
		// ARRANGE - Create a staff member
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "Staff", "Supervised")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMySupervisedGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		// May be empty if no supervisions, just verify no error
		_ = groups
	})
}

// ============================================================================
// Profile Operations Tests
// ============================================================================

func TestUserContextService_GetCurrentProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("retrieves profile for authenticated user", func(t *testing.T) {
		// ARRANGE - Create a test person with account
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "Profile", "Test")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		result, err := service.GetCurrentProfile(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, person.FirstName, result["first_name"])
		assert.Equal(t, person.LastName, result["last_name"])
	})

	t.Run("returns profile with fallback data for account without person", func(t *testing.T) {
		// ARRANGE - Create an account without linked person
		account := testpkg.CreateTestAccount(t, db, "nolink@example.com")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		result, err := service.GetCurrentProfile(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, account.Email, result["email"])
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		// ACT
		_, err := service.GetCurrentProfile(context.Background())

		// ASSERT
		require.Error(t, err)
	})
}

func TestUserContextService_UpdateCurrentProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("updates profile fields", func(t *testing.T) {
		// ARRANGE - Create a test person with account
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Update", "Profile")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))
		updates := map[string]interface{}{
			"first_name": "UpdatedFirst",
			"last_name":  "UpdatedLast",
		}

		// ACT
		result, err := service.UpdateCurrentProfile(ctx, updates)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "UpdatedFirst", result["first_name"])
		assert.Equal(t, "UpdatedLast", result["last_name"])
	})

	t.Run("updates username", func(t *testing.T) {
		// ARRANGE
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Username", "Update")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))
		// Use unique username to avoid duplicate key error
		uniqueUsername := "newusername_" + time.Now().Format("150405.000")
		updates := map[string]interface{}{
			"username": uniqueUsername,
		}

		// ACT
		result, err := service.UpdateCurrentProfile(ctx, updates)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		// Username may be returned as pointer or string
		username := result["username"]
		if usernamePtr, ok := username.(*string); ok && usernamePtr != nil {
			assert.Equal(t, uniqueUsername, *usernamePtr)
		} else if usernameStr, ok := username.(string); ok {
			assert.Equal(t, uniqueUsername, usernameStr)
		} else {
			t.Errorf("Unexpected username type: %T", username)
		}
	})

	t.Run("updates bio", func(t *testing.T) {
		// ARRANGE
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Bio", "Update")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))
		updates := map[string]interface{}{
			"bio": "This is my bio",
		}

		// ACT
		result, err := service.UpdateCurrentProfile(ctx, updates)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "This is my bio", result["bio"])
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		// ACT
		_, err := service.UpdateCurrentProfile(context.Background(), map[string]interface{}{})

		// ASSERT
		require.Error(t, err)
	})
}

func TestUserContextService_UpdateAvatar(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("updates avatar URL", func(t *testing.T) {
		// ARRANGE
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Avatar", "Update")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))
		avatarURL := "/uploads/avatars/test.jpg"

		// ACT
		result, err := service.UpdateAvatar(ctx, avatarURL)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, avatarURL, result["avatar"])
	})

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		// ACT
		_, err := service.UpdateAvatar(context.Background(), "/test.jpg")

		// ASSERT
		require.Error(t, err)
	})
}

// ============================================================================
// Transaction Tests
// ============================================================================

func TestUserContextService_WithTx(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)
	ctx := context.Background()

	t.Run("WithTx returns transactional service", func(t *testing.T) {
		// ARRANGE
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// ACT
		txService := service.WithTx(tx)

		// ASSERT
		require.NotNil(t, txService)
		_, ok := txService.(usercontextSvc.UserContextService)
		assert.True(t, ok, "WithTx should return UserContextService interface")
	})
}

// ============================================================================
// Error Type Tests
// ============================================================================

func TestUserContextErrors(t *testing.T) {
	t.Run("UserContextError contains operation details", func(t *testing.T) {
		err := &usercontextSvc.UserContextError{
			Op:  "test operation",
			Err: usercontextSvc.ErrUserNotAuthenticated,
		}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "test operation")
	})

	t.Run("UserContextError unwraps inner error", func(t *testing.T) {
		innerErr := usercontextSvc.ErrUserNotFound
		err := &usercontextSvc.UserContextError{Op: "test", Err: innerErr}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Equal(t, innerErr, unwrapped)
	})

	t.Run("PartialError contains operation and counts", func(t *testing.T) {
		err := &usercontextSvc.PartialError{
			Op:           "partial test",
			SuccessCount: 5,
			FailureCount: 2,
			FailedIDs:    []int64{1, 2},
			LastErr:      usercontextSvc.ErrGroupNotFound,
		}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "partial")
	})
}

// ============================================================================
// Helper Functions Tests
// ============================================================================

func TestMergeActiveGroups(t *testing.T) {
	// This tests the internal mergeActiveGroups function indirectly
	// through the GetMyActiveGroups method behavior
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("handles empty results gracefully", func(t *testing.T) {
		// ARRANGE
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "Merge", "Test")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMyActiveGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		// Should return empty slice, not nil
		assert.NotNil(t, groups)
	})
}

// ============================================================================
// GetGroupStudents Tests
// ============================================================================

func TestUserContextService_GetGroupStudents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		// ACT
		_, err := service.GetGroupStudents(context.Background(), 1)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for unauthorized access to group", func(t *testing.T) {
		// ARRANGE - Create a staff member
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "NoAccess", "Staff")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT - Try to access a non-existent group
		_, err := service.GetGroupStudents(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns students for supervised group", func(t *testing.T) {
		// ARRANGE
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Supervisor", "GroupStudents")
		activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity for Students")
		room := testpkg.CreateTestRoom(t, db, "Test Room for Students")
		student := testpkg.CreateTestStudent(t, db, "Test", "StudentInGroup", "1a")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, staff.ID, student.ID)

		// Create active group
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID)

		// Create supervision
		testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "supervisor")

		// Create a visit so there's a student in the group
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		students, err := service.GetGroupStudents(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, students)
		assert.GreaterOrEqual(t, len(students), 1, "Should have at least 1 student")
	})
}

// ============================================================================
// GetGroupVisits Tests
// ============================================================================

func TestUserContextService_GetGroupVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		// ACT
		_, err := service.GetGroupVisits(context.Background(), 1)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for unauthorized access to group", func(t *testing.T) {
		// ARRANGE - Create a staff member
		_, account := testpkg.CreateTestStaffWithAccount(t, db, "NoAccess", "Visits")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT - Try to access a non-existent group
		_, err := service.GetGroupVisits(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns visits for supervised group", func(t *testing.T) {
		// ARRANGE
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Supervisor", "GroupVisits")
		activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity for Visits")
		room := testpkg.CreateTestRoom(t, db, "Test Room for Visits")
		student := testpkg.CreateTestStudent(t, db, "Test", "StudentVisit", "1b")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, staff.ID, student.ID)

		// Create active group
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID)

		// Create supervision
		testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "supervisor")

		// Create an active visit (no exit time)
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		visits, err := service.GetGroupVisits(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, visits)
		assert.GreaterOrEqual(t, len(visits), 1, "Should have at least 1 active visit")
	})
}

// ============================================================================
// Teacher Groups with Substitutions Tests
// ============================================================================

func TestUserContextService_GetMyGroups_TeacherGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("returns groups for teacher", func(t *testing.T) {
		// ARRANGE - Create a teacher with account
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Teacher", "Groups")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		// Create an education group and assign teacher
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Teacher Class")
		defer testpkg.CleanupActivityFixtures(t, db, educationGroup.ID, teacher.Staff.ID, teacher.ID)

		// Assign teacher to group
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMyGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 1, "Teacher should have at least 1 group")
	})

	t.Run("returns substitution groups for staff", func(t *testing.T) {
		// ARRANGE - Create a staff with account (as substitute)
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Substitute", "Staff")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		// Create an education group
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Substitution Class")
		defer testpkg.CleanupActivityFixtures(t, db, educationGroup.ID, staff.ID)

		// Create a substitution for today
		today := timezone.DateOfUTC(time.Now())
		tomorrow := today.AddDate(0, 0, 1)
		testpkg.CreateTestGroupSubstitution(t, db, educationGroup.ID, nil, staff.ID, today, tomorrow)

		ctx := contextWithClaims(int(account.ID))

		// ACT
		groups, err := service.GetMyGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 1, "Substitute should have at least 1 group")
	})
}

// ============================================================================
// PartialError Tests
// ============================================================================

func TestPartialError_Unwrap(t *testing.T) {
	// ARRANGE
	innerErr := usercontextSvc.ErrGroupNotFound
	err := &usercontextSvc.PartialError{
		Op:           "test",
		SuccessCount: 1,
		FailureCount: 1,
		FailedIDs:    []int64{1},
		LastErr:      innerErr,
	}

	// ACT
	unwrapped := err.Unwrap()

	// ASSERT
	assert.Equal(t, innerErr, unwrapped)
}

// ============================================================================
// Database Error Tests
// ============================================================================

func TestUserContextService_GetGroupStudents_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT
	_, err := service.GetGroupStudents(canceledCtx, 1)

	// ASSERT: Should return error
	require.Error(t, err)
}

func TestUserContextService_GetGroupVisits_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT
	_, err := service.GetGroupVisits(canceledCtx, 1)

	// ASSERT: Should return error
	require.Error(t, err)
}

func TestUserContextService_GetMyActivityGroups_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT
	_, err := service.GetMyActivityGroups(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
}

func TestUserContextService_GetMyActiveGroups_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT
	_, err := service.GetMyActiveGroups(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
}

func TestUserContextService_GetCurrentUser_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT
	_, err := service.GetCurrentUser(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
}

func TestUserContextService_GetCurrentPerson_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT
	_, err := service.GetCurrentPerson(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
}

func TestUserContextService_GetCurrentStaff_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT
	_, err := service.GetCurrentStaff(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
}

func TestUserContextService_GetCurrentTeacher_DatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// ACT
	_, err := service.GetCurrentTeacher(canceledCtx)

	// ASSERT: Should return error
	require.Error(t, err)
}
