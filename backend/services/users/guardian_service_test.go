// Package users_test tests the users service layer with hermetic testing pattern.
package users_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	usermodels "github.com/moto-nrw/project-phoenix/models/users"
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
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Guardian
}

// =============================================================================
// CreateGuardian Tests
// =============================================================================

func TestGuardianService_CreateGuardian(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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
		// ARRANGE - testing default language and contact method
		// Note: Phone numbers are now added separately via AddPhoneNumber
		req := users.GuardianCreateRequest{
			FirstName: "Default",
			LastName:  "Guardian",
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
		// ARRANGE - guardian can be created without email
		// Phone numbers are added separately via AddPhoneNumber
		req := users.GuardianCreateRequest{
			FirstName: "NoEmail",
			LastName:  "Guardian",
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

// TestGuardianService_CreateGuardian_DuplicateEmail verifies that creating a
// second guardian with an email already in use returns a *ValidationError
// (rendered as a 400 with a German "use the search" message) instead of letting
// the tenant-scoped UNIQUE(tenant_id, email) index fail the INSERT with a raw
// 23505 that would surface as a generic 500 (#1513).
func TestGuardianService_CreateGuardian_DuplicateEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	email := fmt.Sprintf("dup-guardian-%d@example.com", time.Now().UnixNano())
	req := users.GuardianCreateRequest{
		FirstName:              "First",
		LastName:               "Guardian",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}

	// First create succeeds.
	first, err := service.CreateGuardian(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, first)
	defer testpkg.CleanupActivityFixtures(t, db, first.ID)

	// Second create with the SAME email (different name) must be rejected as a
	// validation error, not a DB/operational failure — and nothing is inserted.
	dupReq := req
	dupReq.FirstName = "Second"
	second, err := service.CreateGuardian(ctx, dupReq)

	require.Error(t, err)
	assert.Nil(t, second)
	var validationErr *users.ValidationError
	require.ErrorAs(t, err, &validationErr, "duplicate email must be a ValidationError → 400, not a 500")
	assert.Contains(t, err.Error(), "bereits vergeben")
	assert.Contains(t, err.Error(), "Suche")
}

// =============================================================================
// GetGuardianByID Tests
// =============================================================================

func TestGuardianService_GetGuardianByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("links guardian to student successfully", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "link-to-student")
		student := testpkg.CreateTestStudent(t, db, "Linked", "Student", "1a")
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

	t.Run("is idempotent when the guardian is already linked", func(t *testing.T) {
		// ARRANGE — link a guardian once.
		guardian := testpkg.CreateTestGuardianProfile(t, db, "relink")
		student := testpkg.CreateTestStudent(t, db, "Relink", "Student", "1c")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID, student.ID)

		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			EmergencyPriority: 1,
		}
		first, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, first)

		// ACT — link the SAME guardian to the SAME student again.
		second, err := service.LinkGuardianToStudent(ctx, req)

		// ASSERT — no UNIQUE-constraint error; the existing relationship is
		// returned, and only one link exists.
		require.NoError(t, err, "re-linking an already-linked guardian must not error")
		require.NotNil(t, second)
		assert.Equal(t, first.ID, second.ID, "re-link must return the existing relationship, not create a new one")

		links, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		var count int
		for _, l := range links {
			if l.Profile != nil && l.Profile.ID == guardian.ID {
				count++
			}
		}
		assert.Equal(t, 1, count, "exactly one link must exist after re-linking")
	})

	t.Run("re-linking is a no-op and never overwrites the existing relationship flags", func(t *testing.T) {
		// Linking is a CREATE, not an edit: a re-link must return the existing
		// row untouched. Changing flags goes through the dedicated update path
		// (UpdateStudentGuardianRelationship). This guards against a stray or
		// retried link request silently flipping pickup/emergency-contact flags.
		// ARRANGE — link a guardian with a minimal relationship.
		guardian := testpkg.CreateTestGuardianProfile(t, db, "upsert")
		student := testpkg.CreateTestStudent(t, db, "Upsert", "Student", "1d")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID, student.ID)

		first, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			IsPrimary:         false,
			CanPickup:         false,
			EmergencyPriority: 1,
		})
		require.NoError(t, err)
		require.NotNil(t, first)

		// ACT — re-link the SAME guardian with DIFFERENT relationship flags.
		second, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:          student.ID,
			GuardianProfileID:  guardian.ID,
			RelationshipType:   "guardian",
			IsPrimary:          true,
			IsEmergencyContact: true,
			CanPickup:          true,
			EmergencyPriority:  2,
		})

		// ASSERT — same single row, and the flags STILL reflect the ORIGINAL
		// link, not the re-link request (the changed flags were ignored).
		require.NoError(t, err, "re-linking must not error")
		require.NotNil(t, second)
		assert.Equal(t, first.ID, second.ID, "re-link must return the existing row, not create a new one")

		links, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		var matched int
		for _, l := range links {
			if l.Profile == nil || l.Profile.ID != guardian.ID {
				continue
			}
			matched++
			require.NotNil(t, l.Relationship)
			assert.Equal(t, "parent", l.Relationship.RelationshipType, "relationship type must be unchanged by a re-link")
			assert.False(t, l.Relationship.IsPrimary, "is_primary must be unchanged by a re-link")
			assert.False(t, l.Relationship.IsEmergencyContact, "is_emergency_contact must be unchanged by a re-link")
			assert.False(t, l.Relationship.CanPickup, "can_pickup must be unchanged by a re-link")
			assert.Equal(t, 1, l.Relationship.EmergencyPriority, "emergency_priority must be unchanged by a re-link")
		}
		assert.Equal(t, 1, matched, "exactly one link must exist after re-linking")
	})

	t.Run("returns error when guardian not found", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "NoGuardian", "Student", "1b")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns guardians for student", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "student-guardian")
		student := testpkg.CreateTestStudent(t, db, "HasGuardian", "Student", "2a")
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
		student := testpkg.CreateTestStudent(t, db, "NoGuardians", "Student", "2b")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns students for guardian", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "has-students")
		student := testpkg.CreateTestStudent(t, db, "GuardianChild", "Student", "3a")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns relationship by ID", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-get")
		student := testpkg.CreateTestStudent(t, db, "RelGet", "Student", "4a")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("updates relationship successfully", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-update")
		student := testpkg.CreateTestStudent(t, db, "RelUpdate", "Student", "5a")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("removes guardian from student", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "to-remove")
		student := testpkg.CreateTestStudent(t, db, "RemoveGuardian", "Student", "6a")
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
		student := testpkg.CreateTestStudent(t, db, "NoRel", "Student", "6b")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

	t.Run("returns pending invitations after creating one", func(t *testing.T) {
		// ARRANGE - create a pending invitation
		guardianEmail := fmt.Sprintf("pending-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Pending",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Pending", "Teacher")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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
	dispatcher := email.NewDispatcher(mailer, slog.Default())
	// Use fast retry settings for tests
	dispatcher.SetDefaults(1, []time.Duration{10 * time.Millisecond})

	deps := users.GuardianServiceDependencies{
		GuardianProfileRepo:     repoFactory.GuardianProfile,
		GuardianPhoneNumberRepo: repoFactory.GuardianPhoneNumber,
		StudentGuardianRepo:     repoFactory.StudentGuardian,
		GuardianInvitationRepo:  repoFactory.GuardianInvitation,
		AccountRepo:             repoFactory.Account,
		AccountParentRepo:       repoFactory.AccountParent,
		AccountTenantRepo:       repoFactory.AccountTenant,
		AccountRoleRepo:         repoFactory.AccountRole,
		RoleRepo:                repoFactory.Role,
		StudentRepo:             repoFactory.Student,
		PersonRepo:              repoFactory.Person,
		Mailer:                  mailer,
		Dispatcher:              dispatcher,
		FrontendURL:             "http://localhost:3000",
		DefaultFrom:             email.NewEmail("Test", "test@example.com"),
		InvitationExpiry:        48 * time.Hour,
		DB:                      db,
	}

	return users.NewGuardianService(deps)
}

func TestGuardianService_SendInvitation_SendsEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

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
		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Inviter", "Teacher")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when guardian has no email", func(t *testing.T) {
		// ARRANGE - create guardian without email (phone numbers are added separately)
		req := users.GuardianCreateRequest{
			FirstName:              "NoEmail",
			LastName:               "Guardian",
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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

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
		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "First", "Inviter")
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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

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

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Creator", "Teacher")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when guardian already has account", func(t *testing.T) {
		// ARRANGE - create guardian, send invitation, accept it first
		guardianEmail := fmt.Sprintf("existing-account-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Existing",
			LastName:               "Account",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Teacher", "One")
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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

	t.Run("validates invitation and returns guardian info", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("validate-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Validate",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Validator", "Teacher")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error for already accepted invitation", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("accepted-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Accepted",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Accept", "Teacher")
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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

	t.Run("creates account and links to guardian", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("accept-success-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Accept",
			LastName:               "Success",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Invite", "Teacher")
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

func TestGuardianService_AcceptInvitation_ReusesExistingAccountWithoutPasswordChange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	repoFactory := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)

	guardianEmail := fmt.Sprintf("legacy-existing-%d@example.com", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, db, guardianEmail, "Existing!2026")
	existingHash := *account.PasswordHash
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.account_roles").Where("account_id = ?", account.ID).Exec(ctx)
		_, _ = db.NewDelete().TableExpr("auth.account_tenants").Where("account_id = ?", account.ID).Exec(ctx)
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
	})

	req := users.GuardianCreateRequest{
		FirstName:              "Existing",
		LastName:               "Guardian",
		Email:                  &guardianEmail,
		PreferredContactMethod: "email",
	}
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Existing", "Inviter")
	defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)
	profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

	acceptedAccount, err := service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
		Token:           invitation.Token,
		Password:        testValidPassword,
		ConfirmPassword: testValidPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, acceptedAccount)
	assert.Equal(t, account.ID, acceptedAccount.ID)

	stored, err := repoFactory.Account.FindByEmail(ctx, guardianEmail)
	require.NoError(t, err)
	require.NotNil(t, stored.PasswordHash)
	assert.Equal(t, existingHash, *stored.PasswordHash)
}

func TestGuardianService_AcceptInvitation_ReactivatesExistingTenantMapping(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	repoFactory := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)

	guardianEmail := fmt.Sprintf("legacy-inactive-%d@example.com", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, db, guardianEmail, "Existing!2026")
	deactivatedAt := time.Now().Add(-time.Hour)
	require.NoError(t, repoFactory.AccountTenant.Create(ctx, &authModels.AccountTenant{
		AccountID:     account.ID,
		TenantID:      1,
		Status:        authModels.AccountTenantStatusInactive,
		DeactivatedAt: &deactivatedAt,
	}))
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.account_roles").Where("account_id = ?", account.ID).Exec(ctx)
		_, _ = db.NewDelete().TableExpr("auth.account_tenants").Where("account_id = ?", account.ID).Exec(ctx)
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
	})

	req := users.GuardianCreateRequest{
		FirstName:              "Inactive",
		LastName:               "Guardian",
		Email:                  &guardianEmail,
		PreferredContactMethod: "email",
	}
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Inactive", "Inviter")
	defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.PersonID)
	profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, profile.ID)

	_, err = service.AcceptInvitation(ctx, users.GuardianInvitationAcceptRequest{
		Token:           invitation.Token,
		Password:        testValidPassword,
		ConfirmPassword: testValidPassword,
	})
	require.NoError(t, err)

	var mapping authModels.AccountTenant
	err = db.NewSelect().
		Model(&mapping).
		ModelTableExpr(`auth.account_tenants AS "account_tenant"`).
		Where(`"account_tenant".account_id = ?`, account.ID).
		Where(`"account_tenant".tenant_id = ?`, 1).
		Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, authModels.AccountTenantStatusActive, mapping.Status)
	assert.Nil(t, mapping.DeactivatedAt)
	assert.NotNil(t, mapping.ActivatedAt)
}

func TestGuardianService_AcceptInvitation_PasswordMismatch(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when passwords do not match", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("mismatch-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Mismatch",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Mismatch", "Teacher")
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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error for weak password", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("weak-pwd-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Weak",
			LastName:               "Password",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Weak", "Teacher")
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

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

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

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when invitation already accepted", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("double-accept-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Double",
			LastName:               "Accept",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Double", "Teacher")
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

// =============================================================================
// AddPhoneNumber Tests
// =============================================================================

func TestGuardianService_AddPhoneNumber_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("adds first phone number as primary by default", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "first-phone")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 123 456789",
			PhoneType:   "mobile",
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, guardian.ID, req)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "+49 123 456789", result.PhoneNumber)
		assert.Equal(t, usermodels.PhoneTypeMobile, result.PhoneType)
		assert.True(t, result.IsPrimary, "First phone should be primary")
		assert.Equal(t, 1, result.Priority)
	})

	t.Run("adds second phone number as non-primary", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "second-phone")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Add first phone
		_, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		// Add second phone
		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, guardian.ID, req)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "+49 222 222222", result.PhoneNumber)
		assert.False(t, result.IsPrimary, "Second phone should not be primary")
		assert.Equal(t, 2, result.Priority)
	})

	t.Run("adds phone with explicit primary flag", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "explicit-primary")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Add first phone
		_, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		// Add second phone as primary
		label := "Hauptnummer"
		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 333 333333",
			PhoneType:   "home",
			Label:       &label,
			IsPrimary:   true,
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, guardian.ID, req)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result.IsPrimary)
		assert.Equal(t, &label, result.Label)

		// Verify first phone is no longer primary
		phones, err := service.GetGuardianPhoneNumbers(ctx, guardian.ID)
		require.NoError(t, err)
		for _, phone := range phones {
			if phone.PhoneNumber == "+49 111 111111" {
				assert.False(t, phone.IsPrimary, "Old primary should be unset")
			}
		}
	})

	t.Run("defaults to mobile phone type for invalid type", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "invalid-type")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 444 444444",
			PhoneType:   "invalid_type",
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, guardian.ID, req)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, usermodels.PhoneTypeMobile, result.PhoneType, "Should default to mobile")
	})
}

func TestGuardianService_AddPhoneNumber_Errors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when guardian not found", func(t *testing.T) {
		// ARRANGE
		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 555 555555",
			PhoneType:   "mobile",
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, 99999999, req)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// UpdatePhoneNumber Tests
// =============================================================================

func TestGuardianService_UpdatePhoneNumber_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("updates phone number fields", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-fields")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 666 666666",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		newNumber := "+49 777 777777"
		newType := "work"
		newLabel := "Büro"
		req := users.PhoneNumberUpdateRequest{
			PhoneNumber: &newNumber,
			PhoneType:   &newType,
			Label:       &newLabel,
		}

		// ACT
		err = service.UpdatePhoneNumber(ctx, phone.ID, req)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetPhoneNumberByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, newNumber, updated.PhoneNumber)
		assert.Equal(t, usermodels.PhoneTypeWork, updated.PhoneType)
		assert.Equal(t, &newLabel, updated.Label)
	})

	t.Run("sets phone as primary and unsets others", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Create two phones
		phone1, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		phone2, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		// Make phone2 primary
		isPrimary := true
		req := users.PhoneNumberUpdateRequest{
			IsPrimary: &isPrimary,
		}

		// ACT
		err = service.UpdatePhoneNumber(ctx, phone2.ID, req)

		// ASSERT
		require.NoError(t, err)

		// Verify phone2 is primary
		updated2, err := service.GetPhoneNumberByID(ctx, phone2.ID)
		require.NoError(t, err)
		assert.True(t, updated2.IsPrimary)

		// Verify phone1 is not primary
		updated1, err := service.GetPhoneNumberByID(ctx, phone1.ID)
		require.NoError(t, err)
		assert.False(t, updated1.IsPrimary, "Old primary should be unset")
	})

	t.Run("unsets primary flag", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "unset-primary")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 888 888888",
			PhoneType:   "mobile",
			IsPrimary:   true,
		})
		require.NoError(t, err)

		isPrimary := false
		req := users.PhoneNumberUpdateRequest{
			IsPrimary: &isPrimary,
		}

		// ACT
		err = service.UpdatePhoneNumber(ctx, phone.ID, req)

		// ASSERT
		require.NoError(t, err)

		updated, err := service.GetPhoneNumberByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.False(t, updated.IsPrimary)
	})

	t.Run("updates priority", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-priority")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 999 999999",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		newPriority := 5
		req := users.PhoneNumberUpdateRequest{
			Priority: &newPriority,
		}

		// ACT
		err = service.UpdatePhoneNumber(ctx, phone.ID, req)

		// ASSERT
		require.NoError(t, err)

		updated, err := service.GetPhoneNumberByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, updated.Priority)
	})
}

func TestGuardianService_UpdatePhoneNumber_Errors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when phone not found", func(t *testing.T) {
		// ARRANGE
		newNumber := "+49 000 000000"
		req := users.PhoneNumberUpdateRequest{
			PhoneNumber: &newNumber,
		}

		// ACT
		err := service.UpdatePhoneNumber(ctx, 99999999, req)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// DeletePhoneNumber Tests
// =============================================================================

func TestGuardianService_DeletePhoneNumber_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes non-primary phone", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-non-primary")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		phone1, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		phone2, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		// ACT - delete non-primary phone
		err = service.DeletePhoneNumber(ctx, phone2.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify phone2 is deleted
		_, err = service.GetPhoneNumberByID(ctx, phone2.ID)
		assert.Error(t, err, "Deleted phone should not be found")

		// Verify phone1 still exists and is still primary
		phone1After, err := service.GetPhoneNumberByID(ctx, phone1.ID)
		require.NoError(t, err)
		assert.True(t, phone1After.IsPrimary)
	})

	t.Run("deletes primary phone and promotes next", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-primary")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		phone1, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 333 333333",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		phone2, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 444 444444",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		// Verify phone1 is primary
		assert.True(t, phone1.IsPrimary)

		// ACT - delete primary phone
		err = service.DeletePhoneNumber(ctx, phone1.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify phone1 is deleted
		_, err = service.GetPhoneNumberByID(ctx, phone1.ID)
		assert.Error(t, err)

		// Verify phone2 was promoted to primary
		phone2After, err := service.GetPhoneNumberByID(ctx, phone2.ID)
		require.NoError(t, err)
		assert.True(t, phone2After.IsPrimary, "Next phone should be promoted to primary")
	})

	t.Run("deletes last phone number", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-last")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 555 555555",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		// ACT
		err = service.DeletePhoneNumber(ctx, phone.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify guardian has no phones
		phones, err := service.GetGuardianPhoneNumbers(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Empty(t, phones)
	})
}

func TestGuardianService_DeletePhoneNumber_Errors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when phone not found", func(t *testing.T) {
		// ACT
		err := service.DeletePhoneNumber(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// SetPrimaryPhone Tests
// =============================================================================

func TestGuardianService_SetPrimaryPhone_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("sets phone as primary", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		phone1, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		phone2, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		// ACT - set phone2 as primary
		err = service.SetPrimaryPhone(ctx, phone2.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify phone2 is now primary
		phone2After, err := service.GetPhoneNumberByID(ctx, phone2.ID)
		require.NoError(t, err)
		assert.True(t, phone2After.IsPrimary)

		// Verify phone1 is no longer primary
		phone1After, err := service.GetPhoneNumberByID(ctx, phone1.ID)
		require.NoError(t, err)
		assert.False(t, phone1After.IsPrimary)
	})
}

func TestGuardianService_SetPrimaryPhone_Errors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when phone not found", func(t *testing.T) {
		// ACT
		err := service.SetPrimaryPhone(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// GetGuardianPhoneNumbers Tests
// =============================================================================

func TestGuardianService_GetGuardianPhoneNumbers_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns all phone numbers sorted by priority", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "get-phones")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Add three phones
		_, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		_, err = service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		_, err = service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 333 333333",
			PhoneType:   "home",
		})
		require.NoError(t, err)

		// ACT
		phones, err := service.GetGuardianPhoneNumbers(ctx, guardian.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, phones, 3)

		// Verify they're sorted by priority
		for i := 0; i < len(phones)-1; i++ {
			assert.LessOrEqual(t, phones[i].Priority, phones[i+1].Priority,
				"Phones should be sorted by priority")
		}

		// Verify first phone is primary
		assert.True(t, phones[0].IsPrimary, "First phone should be primary")
	})

	t.Run("returns empty list when guardian has no phones", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-phones")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// ACT
		phones, err := service.GetGuardianPhoneNumbers(ctx, guardian.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, phones)
	})
}

// =============================================================================
// GetPhoneNumberByID Tests
// =============================================================================

func TestGuardianService_GetPhoneNumberByID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns phone number by ID", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "get-by-id")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		label := "Hauptnummer"
		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 444 444444",
			PhoneType:   "mobile",
			Label:       &label,
		})
		require.NoError(t, err)

		// ACT
		result, err := service.GetPhoneNumberByID(ctx, phone.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, phone.ID, result.ID)
		assert.Equal(t, "+49 444 444444", result.PhoneNumber)
		assert.Equal(t, usermodels.PhoneTypeMobile, result.PhoneType)
		assert.Equal(t, &label, result.Label)
	})
}

func TestGuardianService_GetPhoneNumberByID_Errors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupGuardianService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when phone not found", func(t *testing.T) {
		// ACT
		result, err := service.GetPhoneNumberByID(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}
