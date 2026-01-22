// Package users_test tests the users service layer with hermetic testing pattern.
package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Test password constants - these are intentionally simple test values
// that meet password requirements but are clearly not production credentials.
const (
	testValidPassword    = "Testpass1!" // #nosec G101 -- test credential
	testMismatchPassword = "Mismatch2!" // #nosec G101 -- test credential
	testWeakPassword     = "weak"       // Intentionally weak for testing validation
)

// setupGuardianService creates a GuardianService with real database connection
func setupGuardianService(t *testing.T, db *bun.DB) users.GuardianService {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Guardian
}

// =============================================================================
// CreateGuardian Tests
// =============================================================================

func TestGuardianService_CreateGuardian(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("creates guardian successfully", func(t *testing.T) {
		// ARRANGE - use unique email to avoid collisions
		email := fmt.Sprintf("test-guardian-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Test",
			LastName:               "Guardian",
			Email:                  &email,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}

		// ACT
		result, err := service.CreateGuardian(ctx, req)
		defer func() {
			if result != nil {
				testpkg.CleanupActivityFixtures(t, db, result.ID)
			}
		}()

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.ID, int64(0))
		assert.Equal(t, "Test", result.FirstName)
		assert.Equal(t, "Guardian", result.LastName)
		assert.Equal(t, &email, result.Email)
		assert.False(t, result.HasAccount)
	})

	t.Run("creates guardian with defaults", func(t *testing.T) {
		// ARRANGE - phone provided, testing default language and contact method
		phone := "+49123456789"
		req := users.GuardianCreateRequest{
			FirstName: "Default",
			LastName:  "Guardian",
			Phone:     &phone, // At least one contact method required
		}

		// ACT
		result, err := service.CreateGuardian(ctx, req)
		defer func() {
			if result != nil {
				testpkg.CleanupActivityFixtures(t, db, result.ID)
			}
		}()

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, "phone", result.PreferredContactMethod)
		assert.Equal(t, "de", result.LanguagePreference)
	})

	t.Run("creates guardian without email", func(t *testing.T) {
		// ARRANGE - uses phone instead of email
		phone := "+49987654321"
		req := users.GuardianCreateRequest{
			FirstName: "NoEmail",
			LastName:  "Guardian",
			Phone:     &phone, // At least one contact method required
		}

		// ACT
		result, err := service.CreateGuardian(ctx, req)
		defer func() {
			if result != nil {
				testpkg.CleanupActivityFixtures(t, db, result.ID)
			}
		}()

		// ASSERT
		require.NoError(t, err)
		assert.Nil(t, result.Email)
	})
}

// =============================================================================
// GetGuardianByID Tests
// =============================================================================

func TestGuardianService_GetGuardianByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns guardian when found", func(t *testing.T) {
		// ARRANGE
		profile := testpkg.CreateTestGuardianProfile(t, db, "get-by-id")
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// ACT
		result, err := service.GetGuardianByID(ctx, profile.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, profile.ID, result.ID)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetGuardianByID(ctx, 99999999)

		// ASSERT - may return error or nil depending on repository
		if err != nil {
			assert.Nil(t, result)
		} else {
			assert.Nil(t, result)
		}
	})
}

// =============================================================================
// GetGuardianByEmail Tests
// =============================================================================

func TestGuardianService_GetGuardianByEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns guardian when found by email", func(t *testing.T) {
		// ARRANGE
		profile := testpkg.CreateTestGuardianProfile(t, db, "find-by-email")
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// ACT
		result, err := service.GetGuardianByEmail(ctx, *profile.Email)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, profile.ID, result.ID)
	})

	t.Run("returns nil when email not found", func(t *testing.T) {
		// ACT
		result, err := service.GetGuardianByEmail(ctx, "nonexistent@test.local")

		// ASSERT
		if err != nil {
			assert.Nil(t, result)
		} else {
			assert.Nil(t, result)
		}
	})
}

// =============================================================================
// UpdateGuardian Tests
// =============================================================================

func TestGuardianService_UpdateGuardian(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("updates guardian successfully", func(t *testing.T) {
		// ARRANGE
		profile := testpkg.CreateTestGuardianProfile(t, db, "to-update")
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// Use unique email to avoid collisions
		newEmail := fmt.Sprintf("updated-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Updated",
			LastName:               "Name",
			Email:                  &newEmail,
			PreferredContactMethod: "email",
			LanguagePreference:     "en",
		}

		// ACT
		err := service.UpdateGuardian(ctx, profile.ID, req)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetGuardianByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated", updated.FirstName)
		assert.Equal(t, "Name", updated.LastName)
	})

	t.Run("returns error when guardian not found", func(t *testing.T) {
		// ARRANGE
		req := users.GuardianCreateRequest{
			FirstName: "NonExistent",
			LastName:  "Guardian",
		}

		// ACT
		err := service.UpdateGuardian(ctx, 99999999, req)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// DeleteGuardian Tests
// =============================================================================

func TestGuardianService_DeleteGuardian(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("deletes guardian successfully", func(t *testing.T) {
		// ARRANGE
		profile := testpkg.CreateTestGuardianProfile(t, db, "to-delete")
		// No defer - we're testing deletion

		// ACT
		err := service.DeleteGuardian(ctx, profile.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		result, _ := service.GetGuardianByID(ctx, profile.ID)
		assert.Nil(t, result)
	})
}

// =============================================================================
// LinkGuardianToStudent Tests
// =============================================================================

func TestGuardianService_LinkGuardianToStudent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("links guardian to student successfully", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "link-to-student")
		student := testpkg.CreateTestStudent(t, db, "Linked", "Student", "1a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID, student.ID)

		req := users.StudentGuardianCreateRequest{
			StudentID:          student.ID,
			GuardianProfileID:  guardian.ID,
			RelationshipType:   "parent",
			IsPrimary:          true,
			IsEmergencyContact: true,
			CanPickup:          true,
			EmergencyPriority:  1,
		}

		// ACT
		result, err := service.LinkGuardianToStudent(ctx, req)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, student.ID, result.StudentID)
		assert.Equal(t, guardian.ID, result.GuardianProfileID)
		assert.Equal(t, "parent", result.RelationshipType)
		assert.True(t, result.IsPrimary)
	})

	t.Run("returns error when guardian not found", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "NoGuardian", "Student", "1b", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: 99999999,
			RelationshipType:  "parent",
		}

		// ACT
		result, err := service.LinkGuardianToStudent(ctx, req)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "guardian")
	})

	t.Run("returns error when student not found", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "orphan-guardian")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		req := users.StudentGuardianCreateRequest{
			StudentID:         99999999,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		}

		// ACT
		result, err := service.LinkGuardianToStudent(ctx, req)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "student")
	})
}

// =============================================================================
// GetStudentGuardians Tests
// =============================================================================

func TestGuardianService_GetStudentGuardians(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns guardians for student", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "student-guardian")
		student := testpkg.CreateTestStudent(t, db, "HasGuardian", "Student", "2a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID, student.ID)

		// Link guardian to student
		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			IsPrimary:         true,
		}
		_, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)

		// ACT
		result, err := service.GetStudentGuardians(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, guardian.ID, result[0].Profile.ID)
	})

	t.Run("returns empty list when no guardians", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "NoGuardians", "Student", "2b", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		result, err := service.GetStudentGuardians(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// GetGuardianStudents Tests
// =============================================================================

func TestGuardianService_GetGuardianStudents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns students for guardian", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "has-students")
		student := testpkg.CreateTestStudent(t, db, "GuardianChild", "Student", "3a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID, student.ID)

		// Link guardian to student
		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "guardian",
		}
		_, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)

		// ACT
		result, err := service.GetGuardianStudents(ctx, guardian.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, student.ID, result[0].Student.ID)
	})

	t.Run("returns empty list when no students", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-students")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// ACT
		result, err := service.GetGuardianStudents(ctx, guardian.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// GetStudentGuardianRelationship Tests
// =============================================================================

func TestGuardianService_GetStudentGuardianRelationship(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns relationship by ID", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-get")
		student := testpkg.CreateTestStudent(t, db, "RelGet", "Student", "4a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID, student.ID)

		// Create relationship
		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		}
		created, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)

		// ACT
		result, err := service.GetStudentGuardianRelationship(ctx, created.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, created.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetStudentGuardianRelationship(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// UpdateStudentGuardianRelationship Tests
// =============================================================================

func TestGuardianService_UpdateStudentGuardianRelationship(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("updates relationship successfully", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-update")
		student := testpkg.CreateTestStudent(t, db, "RelUpdate", "Student", "5a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID, student.ID)

		// Create relationship
		createReq := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			IsPrimary:         false,
		}
		created, err := service.LinkGuardianToStudent(ctx, createReq)
		require.NoError(t, err)

		// Update
		newType := "guardian"
		isPrimary := true
		updateReq := users.StudentGuardianUpdateRequest{
			RelationshipType: &newType,
			IsPrimary:        &isPrimary,
		}

		// ACT
		err = service.UpdateStudentGuardianRelationship(ctx, created.ID, updateReq)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetStudentGuardianRelationship(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "guardian", updated.RelationshipType)
		assert.True(t, updated.IsPrimary)
	})
}

// =============================================================================
// RemoveGuardianFromStudent Tests
// =============================================================================

func TestGuardianService_RemoveGuardianFromStudent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("removes guardian from student", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "to-remove")
		student := testpkg.CreateTestStudent(t, db, "RemoveGuardian", "Student", "6a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID, student.ID)

		// Create relationship
		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		}
		_, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)

		// ACT
		err = service.RemoveGuardianFromStudent(ctx, student.ID, guardian.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify removal
		guardians, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, guardians)
	})

	t.Run("returns error when relationship not found", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "NoRel", "Student", "6b", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		err := service.RemoveGuardianFromStudent(ctx, student.ID, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ListGuardians Tests
// =============================================================================

func TestGuardianService_ListGuardians(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns list of guardians", func(t *testing.T) {
		// ARRANGE
		guardian1 := testpkg.CreateTestGuardianProfile(t, db, "list-1")
		guardian2 := testpkg.CreateTestGuardianProfile(t, db, "list-2")
		defer testpkg.CleanupActivityFixtures(t, db, guardian1.ID, guardian2.ID)

		// ACT
		result, err := service.ListGuardians(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

// =============================================================================
// GetGuardiansWithoutAccount Tests
// =============================================================================

func TestGuardianService_GetGuardiansWithoutAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns guardians without accounts", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-account")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// ACT
		result, err := service.GetGuardiansWithoutAccount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// Verify our guardian is in the list
		found := false
		for _, g := range result {
			if g.ID == guardian.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Created guardian should be in list")
	})
}

// =============================================================================
// GetInvitableGuardians Tests
// =============================================================================

func TestGuardianService_GetInvitableGuardians(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns invitable guardians", func(t *testing.T) {
		// ARRANGE - create guardian with email (invitable)
		guardian := testpkg.CreateTestGuardianProfile(t, db, "invitable")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// ACT
		result, err := service.GetInvitableGuardians(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		// Our guardian should be invitable (has email, no account)
	})
}

// =============================================================================
// GetPendingInvitations Tests
// =============================================================================

func TestGuardianService_GetPendingInvitations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("returns pending invitations after creating one", func(t *testing.T) {
		// ARRANGE - create a pending invitation
		guardianEmail := fmt.Sprintf("pending-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Pending",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Pending", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		profile, _, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// ACT - get pending invitations
		result, err := service.GetPendingInvitations(ctx)

		// ASSERT
		require.NoError(t, err, "GetPendingInvitations should not return error")
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1, "should have at least one pending invitation")
	})

	t.Run("returns empty or nil when no pending invitations", func(t *testing.T) {
		// This test just verifies no error is returned
		// Result can be nil or empty slice - both are valid
		result, err := service.GetPendingInvitations(ctx)

		require.NoError(t, err, "GetPendingInvitations should not return error")
		// nil or empty slice are both acceptable when no invitations exist
		if result != nil {
			t.Logf("Found %d pending invitations", len(result))
		}
	})
}

// =============================================================================
// CleanupExpiredInvitations Tests
// =============================================================================

func TestGuardianService_CleanupExpiredInvitations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("cleans up expired invitations", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredInvitations(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

// =============================================================================
// Invitation Email Tests (with capturing mailer)
// =============================================================================

// setupGuardianServiceWithMailer creates a GuardianService with injected mailer for testing email flows
func setupGuardianServiceWithMailer(db *bun.DB, mailer *testpkg.CapturingMailer) users.GuardianService {
	repoFactory := repositories.NewFactory(db)

	// Create dispatcher from the capturing mailer
	dispatcher := email.NewDispatcher(mailer)
	// Use fast retry settings for tests
	dispatcher.SetDefaults(1, []time.Duration{10 * time.Millisecond})

	deps := users.GuardianServiceDependencies{
		GuardianProfileRepo:    repoFactory.GuardianProfile,
		StudentGuardianRepo:    repoFactory.StudentGuardian,
		GuardianInvitationRepo: repoFactory.GuardianInvitation,
		AccountParentRepo:      repoFactory.AccountParent,
		StudentRepo:            repoFactory.Student,
		PersonRepo:             repoFactory.Person,
		Mailer:                 mailer,
		Dispatcher:             dispatcher,
		FrontendURL:            "http://localhost:3000",
		DefaultFrom:            email.NewEmail("Test", "test@example.com"),
		InvitationExpiry:       48 * time.Hour,
		DB:                     db,
	}

	return users.NewGuardianService(deps)
}

func TestGuardianService_SendInvitation_SendsEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("sends invitation email to guardian", func(t *testing.T) {
		// ARRANGE - create guardian with email
		guardianEmail := fmt.Sprintf("invite-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Invite",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}
		guardian, err := service.CreateGuardian(ctx, req)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Create a teacher to be the inviter
		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Inviter", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		// ACT - send invitation
		invitation, err := service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: guardian.ID,
			CreatedBy:         *teacher.Staff.Person.AccountID,
		})

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, invitation)
		assert.NotEmpty(t, invitation.Token)

		// Wait for async email dispatch
		emailSent := mailer.WaitForMessages(1, 500*time.Millisecond)
		assert.True(t, emailSent, "Expected invitation email to be sent")

		if emailSent {
			msgs := mailer.Messages()
			assert.Equal(t, "Einladung zum Eltern-Portal", msgs[0].Subject)
			assert.Equal(t, guardianEmail, msgs[0].To.Address)
		}
	})
}

func TestGuardianService_SendInvitation_GuardianNotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns error for nonexistent guardian", func(t *testing.T) {
		// ACT
		invitation, err := service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: 99999999,
			CreatedBy:         1,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, invitation)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianService_SendInvitation_NoEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns error when guardian has no email", func(t *testing.T) {
		// ARRANGE - create guardian without email (phone only)
		phone := "+49123456789"
		req := users.GuardianCreateRequest{
			FirstName:              "NoEmail",
			LastName:               "Guardian",
			Phone:                  &phone,
			PreferredContactMethod: "phone",
		}
		guardian, err := service.CreateGuardian(ctx, req)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// ACT
		invitation, err := service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: guardian.ID,
			CreatedBy:         1,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, invitation)
		assert.Contains(t, err.Error(), "cannot be invited")
	})
}

func TestGuardianService_SendInvitation_DuplicatePending(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("returns error when guardian has pending invitation", func(t *testing.T) {
		// ARRANGE - create guardian
		guardianEmail := fmt.Sprintf("duplicate-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Duplicate",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}
		guardian, err := service.CreateGuardian(ctx, req)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Create first invitation
		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "First", "Inviter", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		_, err = service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: guardian.ID,
			CreatedBy:         *teacher.Staff.Person.AccountID,
		})
		require.NoError(t, err)

		// ACT - try to send another invitation
		invitation, err := service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: guardian.ID,
			CreatedBy:         *teacher.Staff.Person.AccountID,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, invitation)
		assert.Contains(t, err.Error(), "pending invitation")
	})
}

// =============================================================================
// CreateGuardianWithInvitation Tests
// =============================================================================

func TestGuardianService_CreateGuardianWithInvitation_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("creates guardian and sends invitation in one transaction", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("combined-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Combined",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Creator", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		// ACT
		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		defer func() {
			if profile != nil {
				testpkg.CleanupActivityFixtures(t, db, profile.ID)
			}
		}()

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.NotNil(t, invitation)
		assert.Equal(t, "Combined", profile.FirstName)
		assert.Equal(t, guardianEmail, *profile.Email)
		assert.NotEmpty(t, invitation.Token)
		assert.Equal(t, profile.ID, invitation.GuardianProfileID)

		// Verify email was sent
		emailSent := mailer.WaitForMessages(1, 500*time.Millisecond)
		assert.True(t, emailSent, "Expected invitation email to be sent")
	})
}

func TestGuardianService_CreateGuardianWithInvitation_NoEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns error when email not provided", func(t *testing.T) {
		// ARRANGE - no email
		req := users.GuardianCreateRequest{
			FirstName: "NoEmail",
			LastName:  "Guardian",
		}

		// ACT
		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, 1)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Nil(t, invitation)
		assert.Contains(t, err.Error(), "email is required")
	})
}

func TestGuardianService_CreateGuardianWithInvitation_ExistingAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("returns error when guardian already has account", func(t *testing.T) {
		// ARRANGE - create guardian, send invitation, accept it first
		guardianEmail := fmt.Sprintf("existing-account-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Existing",
			LastName:               "Account",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Teacher", "One", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		// Create first guardian with invitation
		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// Accept the invitation to create account
		_, err = service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
			Token:           invitation.Token,
			Password:        testValidPassword,
			ConfirmPassword: testValidPassword,
		})
		require.NoError(t, err)

		// ACT - try to create another guardian with same email
		_, _, err = service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already has an account")
	})
}

// =============================================================================
// ValidateInvitation Tests
// =============================================================================

func TestGuardianService_ValidateInvitation_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("validates invitation and returns guardian info", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("validate-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Validate",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Validator", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// ACT
		result, err := service.ValidateInvitation(ctx, invitation.Token)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "Validate", result.GuardianFirstName)
		assert.Equal(t, "Test", result.GuardianLastName)
		assert.Equal(t, guardianEmail, result.Email)
		assert.NotEmpty(t, result.ExpiresAt)
	})
}

func TestGuardianService_ValidateInvitation_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns error for invalid token", func(t *testing.T) {
		// ACT
		result, err := service.ValidateInvitation(ctx, "invalid-token-12345")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianService_ValidateInvitation_AlreadyAccepted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("returns error for already accepted invitation", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("accepted-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Accepted",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Accept", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// Accept the invitation
		_, err = service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
			Token:           invitation.Token,
			Password:        testValidPassword,
			ConfirmPassword: testValidPassword,
		})
		require.NoError(t, err)

		// ACT - try to validate again
		result, err := service.ValidateInvitation(ctx, invitation.Token)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "already been accepted")
	})
}

// =============================================================================
// AcceptInvitation Tests
// =============================================================================

func TestGuardianService_AcceptInvitation_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("creates account and links to guardian", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("accept-success-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Accept",
			LastName:               "Success",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Invite", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// ACT
		account, err := service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
			Token:           invitation.Token,
			Password:        testValidPassword,
			ConfirmPassword: testValidPassword,
		})

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, account)
		assert.Equal(t, guardianEmail, account.Email)
		assert.True(t, account.Active)

		// Verify guardian now has account
		updatedProfile, err := service.GetGuardianByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.True(t, updatedProfile.HasAccount)
	})
}

func TestGuardianService_AcceptInvitation_PasswordMismatch(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("returns error when passwords do not match", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("mismatch-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Mismatch",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Mismatch", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// ACT
		account, err := service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
			Token:           invitation.Token,
			Password:        testValidPassword,
			ConfirmPassword: testMismatchPassword,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account)
		assert.Contains(t, err.Error(), "do not match")
	})
}

func TestGuardianService_AcceptInvitation_WeakPassword(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("returns error for weak password", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("weak-pwd-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Weak",
			LastName:               "Password",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Weak", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// ACT - weak password (no special chars, too short)
		account, err := service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
			Token:           invitation.Token,
			Password:        testWeakPassword,
			ConfirmPassword: testWeakPassword,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account)
		assert.Contains(t, err.Error(), "password")
	})
}

func TestGuardianService_AcceptInvitation_InvalidToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupGuardianService(t, db)
	ctx := context.Background()

	t.Run("returns error for invalid token", func(t *testing.T) {
		// ACT
		account, err := service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
			Token:           "invalid-token-xyz",
			Password:        testValidPassword,
			ConfirmPassword: testValidPassword,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianService_AcceptInvitation_AlreadyAccepted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := context.Background()

	t.Run("returns error when invitation already accepted", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("double-accept-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Double",
			LastName:               "Accept",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Double", "Teacher", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)

		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

		// Accept first time
		_, err = service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
			Token:           invitation.Token,
			Password:        testValidPassword,
			ConfirmPassword: testValidPassword,
		})
		require.NoError(t, err)

		// ACT - try to accept again
		account, err := service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
			Token:           invitation.Token,
			Password:        testValidPassword,
			ConfirmPassword: testValidPassword,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account)
		assert.Contains(t, err.Error(), "already been accepted")
	})
}
