package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTestStudentGuardian creates a student-guardian relationship in the database.
func createTestStudentGuardian(t *testing.T, db *bun.DB, studentID, guardianProfileID int64, relType string, isPrimary bool) *users.StudentGuardian {
	t.Helper()

	ctx := context.Background()
	sg := &users.StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardianProfileID,
		RelationshipType:   relType,
		IsPrimary:          isPrimary,
		IsEmergencyContact: false,
		CanPickup:          true,
		Permissions:        map[string]interface{}{},
	}

	_, err := db.NewInsert().
		Model(sg).
		ModelTableExpr(`users.students_guardians`).
		Exec(ctx)
	require.NoError(t, err)

	return sg
}

// cleanupStudentGuardians removes student_guardian records by ID.
func cleanupStudentGuardians(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	testpkg.CleanupTableRecords(t, db, "users.students_guardians", ids...)
}

// ============================================================================
// FindByStudentID Tests
// ============================================================================

func TestStudentGuardianRepository_FindByStudentID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "Guardian", "Student", "1a")
	guardian1 := testpkg.CreateTestGuardianProfile(t, db, "guardian1")
	guardian2 := testpkg.CreateTestGuardianProfile(t, db, "guardian2")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian1.ID, guardian2.ID)

	// Create relationships
	sg1 := createTestStudentGuardian(t, db, student.ID, guardian1.ID, "parent", true)
	sg2 := createTestStudentGuardian(t, db, student.ID, guardian2.ID, "parent", false)
	defer cleanupStudentGuardians(t, db, sg1.ID, sg2.ID)

	// ACT
	results, err := repo.FindByStudentID(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)

	for _, sg := range results {
		assert.Equal(t, student.ID, sg.StudentID)
	}
}

func TestStudentGuardianRepository_FindByStudentID_Empty(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create student with no guardians
	student := testpkg.CreateTestStudent(t, db, "NoGuardian", "Student", "1b")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// ACT
	results, err := repo.FindByStudentID(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ============================================================================
// FindByGuardianProfileID Tests
// ============================================================================

func TestStudentGuardianRepository_FindByGuardianProfileID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student1 := testpkg.CreateTestStudent(t, db, "Multi1", "Student", "2a")
	student2 := testpkg.CreateTestStudent(t, db, "Multi2", "Student", "2b")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "multi-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, guardian.ID)

	// Create relationships - same guardian, different students
	sg1 := createTestStudentGuardian(t, db, student1.ID, guardian.ID, "parent", true)
	sg2 := createTestStudentGuardian(t, db, student2.ID, guardian.ID, "parent", true)
	defer cleanupStudentGuardians(t, db, sg1.ID, sg2.ID)

	// ACT
	results, err := repo.FindByGuardianProfileID(ctx, guardian.ID)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)

	for _, sg := range results {
		assert.Equal(t, guardian.ID, sg.GuardianProfileID)
	}
}

// ============================================================================
// FindByID Tests
// ============================================================================

func TestStudentGuardianRepository_FindByID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "FindByID", "Student", "3a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "findbyid-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", true)
	defer cleanupStudentGuardians(t, db, sg.ID)

	found, err := repo.FindByID(ctx, sg.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, sg.ID, found.ID)
	assert.Equal(t, student.ID, found.StudentID)
}

func TestStudentGuardianRepository_FindByID_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	_, err := repo.FindByID(ctx, int64(99999999))

	require.Error(t, err)
}

// ============================================================================
// Update Tests
// ============================================================================

func TestStudentGuardianRepository_Update_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Update", "Student", "4a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "update-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", true)
	defer cleanupStudentGuardians(t, db, sg.ID)

	sg.RelationshipType = "guardian"
	sg.IsPrimary = false

	err := repo.Update(ctx, sg)
	require.NoError(t, err)

	updated, err := repo.FindByID(ctx, sg.ID)
	require.NoError(t, err)
	assert.Equal(t, "guardian", updated.RelationshipType)
	assert.False(t, updated.IsPrimary)
}

func TestStudentGuardianRepository_Update_NilReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	err := repo.Update(ctx, nil)

	require.Error(t, err)
}

// ============================================================================
// Delete Tests
// ============================================================================

func TestStudentGuardianRepository_Delete_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Delete", "Student", "5a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", true)

	err := repo.Delete(ctx, sg.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, sg.ID)
	require.Error(t, err)
}

// ============================================================================
// Create Tests
// ============================================================================

func TestStudentGuardianRepository_Create_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "CreateSG", "Student", "10a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "create-sg-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// ACT
	sg := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  guardian.ID,
		RelationshipType:   "parent",
		IsPrimary:          true,
		IsEmergencyContact: false,
		CanPickup:          true,
		Permissions:        map[string]interface{}{},
	}
	err := repo.Create(ctx, sg)

	// ASSERT
	require.NoError(t, err)
	assert.NotZero(t, sg.ID)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// Verify it was created
	results, err := repo.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestStudentGuardianRepository_Create_NilReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// ACT
	err := repo.Create(ctx, nil)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestStudentGuardianRepository_Create_ValidationError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// ACT - Create with invalid data
	sg := &users.StudentGuardian{
		StudentID:         0, // Invalid
		GuardianProfileID: 0, // Invalid
		RelationshipType:  "",
	}
	err := repo.Create(ctx, sg)

	// ASSERT
	require.Error(t, err)
}
