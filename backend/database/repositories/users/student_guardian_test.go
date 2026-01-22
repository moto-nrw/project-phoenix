package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "Guardian", "Student", "1a", ogsID)
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
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create student with no guardians
	student := testpkg.CreateTestStudent(t, db, "NoGuardian", "Student", "1b", ogsID)
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
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student1 := testpkg.CreateTestStudent(t, db, "Multi1", "Student", "2a", ogsID)
	student2 := testpkg.CreateTestStudent(t, db, "Multi2", "Student", "2b", ogsID)
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
// FindPrimaryByStudentID Tests
// ============================================================================

func TestStudentGuardianRepository_FindPrimaryByStudentID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "Primary", "Student", "3a", ogsID)
	primaryGuardian := testpkg.CreateTestGuardianProfile(t, db, "primary-guardian")
	secondaryGuardian := testpkg.CreateTestGuardianProfile(t, db, "secondary-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, primaryGuardian.ID, secondaryGuardian.ID)

	// Create relationships - one primary, one not
	sgPrimary := createTestStudentGuardian(t, db, student.ID, primaryGuardian.ID, "parent", true)
	sgSecondary := createTestStudentGuardian(t, db, student.ID, secondaryGuardian.ID, "parent", false)
	defer cleanupStudentGuardians(t, db, sgPrimary.ID, sgSecondary.ID)

	// ACT
	result, err := repo.FindPrimaryByStudentID(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.True(t, result.IsPrimary)
	assert.Equal(t, primaryGuardian.ID, result.GuardianProfileID)
}

func TestStudentGuardianRepository_FindPrimaryByStudentID_NoPrimary(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create student with no guardians
	student := testpkg.CreateTestStudent(t, db, "NoPrimary", "Student", "3b", ogsID)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// ACT
	_, err := repo.FindPrimaryByStudentID(ctx, student.ID)

	// ASSERT
	require.Error(t, err) // No primary found
}

// ============================================================================
// FindEmergencyContactsByStudentID Tests
// ============================================================================

func TestStudentGuardianRepository_FindEmergencyContactsByStudentID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "Emergency", "Student", "4a", ogsID)
	emergencyGuardian := testpkg.CreateTestGuardianProfile(t, db, "emergency-contact")
	regularGuardian := testpkg.CreateTestGuardianProfile(t, db, "regular-contact")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, emergencyGuardian.ID, regularGuardian.ID)

	// Create emergency contact
	sgEmergency := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  emergencyGuardian.ID,
		RelationshipType:   "parent",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
		Permissions:        map[string]interface{}{},
	}
	_, err := db.NewInsert().Model(sgEmergency).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(t, err)

	// Create regular contact
	sgRegular := createTestStudentGuardian(t, db, student.ID, regularGuardian.ID, "parent", false)
	defer cleanupStudentGuardians(t, db, sgEmergency.ID, sgRegular.ID)

	// ACT
	results, err := repo.FindEmergencyContactsByStudentID(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	for _, sg := range results {
		assert.True(t, sg.IsEmergencyContact)
	}
}

// ============================================================================
// FindPickupAuthoritiesByStudentID Tests
// ============================================================================

func TestStudentGuardianRepository_FindPickupAuthoritiesByStudentID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "Pickup", "Student", "5a", ogsID)
	pickupGuardian := testpkg.CreateTestGuardianProfile(t, db, "pickup-guardian")
	noPickupGuardian := testpkg.CreateTestGuardianProfile(t, db, "no-pickup-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, pickupGuardian.ID, noPickupGuardian.ID)

	// Create guardian who can pickup
	sgPickup := createTestStudentGuardian(t, db, student.ID, pickupGuardian.ID, "parent", true)

	// Create guardian who cannot pickup
	sgNoPickup := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  noPickupGuardian.ID,
		RelationshipType:   "guardian",
		IsPrimary:          false,
		IsEmergencyContact: false,
		CanPickup:          false,
		Permissions:        map[string]interface{}{},
	}
	_, err := db.NewInsert().Model(sgNoPickup).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(t, err)
	defer cleanupStudentGuardians(t, db, sgPickup.ID, sgNoPickup.ID)

	// ACT
	results, err := repo.FindPickupAuthoritiesByStudentID(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	for _, sg := range results {
		assert.True(t, sg.CanPickup)
	}
}

// ============================================================================
// SetPrimary Tests
// ============================================================================

func TestStudentGuardianRepository_SetPrimary_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "SetPrimary", "Student", "6a", ogsID)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// Create relationship that is NOT primary
	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", false)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// ACT
	err := repo.SetPrimary(ctx, sg.ID, true)

	// ASSERT
	require.NoError(t, err)

	// Verify it was updated
	results, err := repo.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)

	var found bool
	for _, r := range results {
		if r.ID == sg.ID {
			found = true
			assert.True(t, r.IsPrimary)
			break
		}
	}
	assert.True(t, found)
}

// ============================================================================
// SetEmergencyContact Tests
// ============================================================================

func TestStudentGuardianRepository_SetEmergencyContact_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "SetEmergency", "Student", "7a", ogsID)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "set-emergency-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// Create relationship
	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", false)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// ACT
	err := repo.SetEmergencyContact(ctx, sg.ID, true)

	// ASSERT
	require.NoError(t, err)

	// Verify it was updated
	results, err := repo.FindEmergencyContactsByStudentID(ctx, student.ID)
	require.NoError(t, err)

	var found bool
	for _, r := range results {
		if r.ID == sg.ID {
			found = true
			break
		}
	}
	assert.True(t, found)
}

// ============================================================================
// SetCanPickup Tests
// ============================================================================

func TestStudentGuardianRepository_SetCanPickup_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "SetPickup", "Student", "8a", ogsID)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "set-pickup-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// Create relationship with CanPickup = true initially
	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", false)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// ACT - Set to false
	err := repo.SetCanPickup(ctx, sg.ID, false)

	// ASSERT
	require.NoError(t, err)

	// Verify it was updated (should NOT be in pickup authorities now)
	results, err := repo.FindPickupAuthoritiesByStudentID(ctx, student.ID)
	require.NoError(t, err)

	for _, r := range results {
		assert.NotEqual(t, sg.ID, r.ID) // Should not find our record
	}
}

// ============================================================================
// UpdatePermissions Tests
// ============================================================================

func TestStudentGuardianRepository_UpdatePermissions_ValidJSON(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "Permissions", "Student", "9a", ogsID)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "permissions-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// Create relationship
	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", true)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// ACT
	permissions := `{"view_grades": true, "view_attendance": true, "communicate_teacher": false}`
	err := repo.UpdatePermissions(ctx, sg.ID, permissions)

	// ASSERT
	require.NoError(t, err)

	// Verify permissions were updated
	results, err := repo.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)

	var found bool
	for _, r := range results {
		if r.ID == sg.ID {
			found = true
			// Check permissions map
			assert.NotNil(t, r.Permissions)
			break
		}
	}
	assert.True(t, found)
}

func TestStudentGuardianRepository_UpdatePermissions_InvalidJSON(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "InvalidJSON", "Student", "9b", ogsID)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "invalid-json-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// Create relationship
	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", true)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// ACT
	invalidJSON := `{not valid json}`
	err := repo.UpdatePermissions(ctx, sg.ID, invalidJSON)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

// ============================================================================
// Create Tests
// ============================================================================

func TestStudentGuardianRepository_Create_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "CreateSG", "Student", "10a", ogsID)
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

// ============================================================================
// FindByRelationshipType Tests
// ============================================================================

func TestStudentGuardianRepository_FindByRelationshipType_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "RelType", "Student", "12a", ogsID)
	parentGuardian := testpkg.CreateTestGuardianProfile(t, db, "parent-type")
	relativeGuardian := testpkg.CreateTestGuardianProfile(t, db, "relative-type")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, parentGuardian.ID, relativeGuardian.ID)

	// Create relationships with different types
	sgParent := createTestStudentGuardian(t, db, student.ID, parentGuardian.ID, "parent", true)
	sgRelative := createTestStudentGuardian(t, db, student.ID, relativeGuardian.ID, "relative", false)
	defer cleanupStudentGuardians(t, db, sgParent.ID, sgRelative.ID)

	// ACT
	results, err := repo.FindByRelationshipType(ctx, student.ID, "parent")

	// ASSERT
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	for _, sg := range results {
		assert.Equal(t, "parent", sg.RelationshipType)
	}
}

// ============================================================================
// List Tests
// ============================================================================

func TestStudentGuardianRepository_List_WithFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := context.Background()

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "ListTest", "Student", "13a", ogsID)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "list-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// Create primary guardian
	sg := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  guardian.ID,
		RelationshipType:   "parent",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
		Permissions:        map[string]interface{}{},
	}
	_, err := db.NewInsert().Model(sg).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(t, err)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// ACT
	results, err := repo.List(ctx, map[string]interface{}{
		"is_primary": true,
	})

	// ASSERT
	require.NoError(t, err)
	for _, r := range results {
		assert.True(t, r.IsPrimary)
	}
}
