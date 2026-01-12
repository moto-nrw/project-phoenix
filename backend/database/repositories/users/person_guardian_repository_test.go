package users_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// createTestParentAccount creates a parent account in auth.accounts_parents for testing
func createTestParentAccount(t *testing.T, db *bun.DB, emailPrefix string) int64 {
	t.Helper()
	ctx := context.Background()

	uniqueEmail := fmt.Sprintf("%s-%d@test.local", emailPrefix, time.Now().UnixNano())

	var id int64
	err := db.NewRaw(
		`INSERT INTO auth.accounts_parents (email, active, created_at, updated_at)
		 VALUES (?, ?, NOW(), NOW()) RETURNING id`,
		uniqueEmail, true).Scan(ctx, &id)
	require.NoError(t, err, "Failed to create test parent account")

	return id
}

// createTestPersonGuardian creates a person guardian relationship for testing
func createTestPersonGuardian(t *testing.T, db *bun.DB, personID, guardianAccountID int64, relType users.RelationshipType, isPrimary bool) *users.PersonGuardian {
	t.Helper()
	ctx := context.Background()

	pg := &users.PersonGuardian{
		PersonID:          personID,
		GuardianAccountID: guardianAccountID,
		RelationshipType:  relType,
		IsPrimary:         isPrimary,
		Permissions:       "{}",
	}

	err := db.NewInsert().
		Model(pg).
		ModelTableExpr(`users.persons_guardians`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test person guardian")

	return pg
}

// cleanupPersonGuardianRecords removes person guardian records and related data
func cleanupPersonGuardianRecords(t *testing.T, db *bun.DB, pgIDs ...int64) {
	t.Helper()
	if len(pgIDs) == 0 {
		return
	}

	ctx := context.Background()

	_, err := db.NewDelete().
		TableExpr("users.persons_guardians").
		Where("id IN (?)", bun.In(pgIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup person guardians: %v", err)
	}
}

// cleanupParentAccounts removes parent accounts
func cleanupParentAccounts(t *testing.T, db *bun.DB, accountIDs ...int64) {
	t.Helper()
	if len(accountIDs) == 0 {
		return
	}

	ctx := context.Background()

	_, err := db.NewDelete().
		TableExpr("auth.accounts_parents").
		Where("id IN (?)", bun.In(accountIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup parent accounts: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestPersonGuardianRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PersonGuardian
	ctx := context.Background()

	t.Run("creates person guardian with valid data", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "Guardian", "Create")
		accountID := createTestParentAccount(t, db, "guardian-create")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := &users.PersonGuardian{
			PersonID:          person.ID,
			GuardianAccountID: accountID,
			RelationshipType:  users.RelationshipParent,
			IsPrimary:         true,
			Permissions:       "{}",
		}

		err := repo.Create(ctx, pg)
		require.NoError(t, err)
		assert.NotZero(t, pg.ID)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		// Verify in DB
		found, err := repo.FindByID(ctx, pg.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, found.PersonID)
		assert.Equal(t, accountID, found.GuardianAccountID)
		assert.Equal(t, users.RelationshipParent, found.RelationshipType)
	})

	t.Run("fails with nil person guardian", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("fails with missing person ID", func(t *testing.T) {
		accountID := createTestParentAccount(t, db, "guardian-no-person")
		defer cleanupParentAccounts(t, db, accountID)

		pg := &users.PersonGuardian{
			PersonID:          0,
			GuardianAccountID: accountID,
			RelationshipType:  users.RelationshipParent,
		}

		err := repo.Create(ctx, pg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "person ID")
	})
}

// ============================================================================
// Query Tests - FindByPersonID
// ============================================================================

func TestPersonGuardianRepository_FindByPersonID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PersonGuardian
	ctx := context.Background()

	t.Run("finds guardians by person ID", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "FindByPerson", "Test")
		accountID := createTestParentAccount(t, db, "guardian-findperson")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := createTestPersonGuardian(t, db, person.ID, accountID, users.RelationshipParent, true)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		found, err := repo.FindByPersonID(ctx, person.ID)
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, pg.ID, found[0].ID)
	})

	t.Run("returns empty slice for non-existent person ID", func(t *testing.T) {
		found, err := repo.FindByPersonID(ctx, int64(999999))
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

// ============================================================================
// Query Tests - FindByGuardianID
// ============================================================================

func TestPersonGuardianRepository_FindByGuardianID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PersonGuardian
	ctx := context.Background()

	t.Run("finds relationships by guardian account ID", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "FindByGuardian", "Test")
		accountID := createTestParentAccount(t, db, "guardian-findguardian")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := createTestPersonGuardian(t, db, person.ID, accountID, users.RelationshipParent, true)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		found, err := repo.FindByGuardianID(ctx, accountID)
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, pg.ID, found[0].ID)
	})

	t.Run("returns empty slice for non-existent guardian ID", func(t *testing.T) {
		found, err := repo.FindByGuardianID(ctx, int64(999999))
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

// ============================================================================
// Query Tests - FindPrimaryByPersonID
// ============================================================================

func TestPersonGuardianRepository_FindPrimaryByPersonID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PersonGuardian
	ctx := context.Background()

	t.Run("finds primary guardian for person", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "FindPrimary", "Test")
		accountID := createTestParentAccount(t, db, "guardian-primary")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := createTestPersonGuardian(t, db, person.ID, accountID, users.RelationshipParent, true)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		found, err := repo.FindPrimaryByPersonID(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, pg.ID, found.ID)
		assert.True(t, found.IsPrimary)
	})

	t.Run("returns error for non-existent primary guardian", func(t *testing.T) {
		_, err := repo.FindPrimaryByPersonID(ctx, int64(999999))
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests - FindByRelationshipType
// ============================================================================

func TestPersonGuardianRepository_FindByRelationshipType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PersonGuardian
	ctx := context.Background()

	t.Run("finds guardians by relationship type", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "FindRelType", "Test")
		accountID1 := createTestParentAccount(t, db, "guardian-reltype1")
		accountID2 := createTestParentAccount(t, db, "guardian-reltype2")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID1, accountID2)

		// Create parent relationship
		pgParent := createTestPersonGuardian(t, db, person.ID, accountID1, users.RelationshipParent, true)
		// Create guardian relationship
		pgGuardian := createTestPersonGuardian(t, db, person.ID, accountID2, users.RelationshipGuardian, false)
		defer cleanupPersonGuardianRecords(t, db, pgParent.ID, pgGuardian.ID)

		// Find by parent type
		found, err := repo.FindByRelationshipType(ctx, person.ID, users.RelationshipParent)
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, pgParent.ID, found[0].ID)

		// Find by guardian type
		found, err = repo.FindByRelationshipType(ctx, person.ID, users.RelationshipGuardian)
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, pgGuardian.ID, found[0].ID)
	})
}

// ============================================================================
// Update Tests - SetPrimary
// ============================================================================

func TestPersonGuardianRepository_SetPrimary(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PersonGuardian
	ctx := context.Background()

	t.Run("sets primary status", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "SetPrimary", "Test")
		accountID := createTestParentAccount(t, db, "guardian-setprimary")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := createTestPersonGuardian(t, db, person.ID, accountID, users.RelationshipParent, false)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		// Set as primary
		err := repo.SetPrimary(ctx, pg.ID, true)
		require.NoError(t, err)

		// Verify update
		found, err := repo.FindByID(ctx, pg.ID)
		require.NoError(t, err)
		assert.True(t, found.IsPrimary)
	})
}

// ============================================================================
// Update Tests - UpdatePermissions
// ============================================================================

func TestPersonGuardianRepository_UpdatePermissions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PersonGuardian
	ctx := context.Background()

	t.Run("updates permissions", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "UpdatePerms", "Test")
		accountID := createTestParentAccount(t, db, "guardian-updateperms")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := createTestPersonGuardian(t, db, person.ID, accountID, users.RelationshipParent, true)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		// Update permissions
		newPerms := `{"view_attendance":true,"view_grades":true}`
		err := repo.UpdatePermissions(ctx, pg.ID, newPerms)
		require.NoError(t, err)

		// Verify update - compare as JSON since PostgreSQL JSONB normalizes format
		found, err := repo.FindByID(ctx, pg.ID)
		require.NoError(t, err)

		var expectedPerms, actualPerms map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(newPerms), &expectedPerms))
		require.NoError(t, json.Unmarshal([]byte(found.Permissions), &actualPerms))
		assert.Equal(t, expectedPerms, actualPerms)
	})
}

// ============================================================================
// Query Tests - List
// ============================================================================

func TestPersonGuardianRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PersonGuardian
	ctx := context.Background()

	t.Run("lists with filter options", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "ListOpts", "Test")
		accountID := createTestParentAccount(t, db, "guardian-listopts")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := createTestPersonGuardian(t, db, person.ID, accountID, users.RelationshipParent, true)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		// List with person_id filter
		found, err := repo.List(ctx, map[string]interface{}{
			"person_id": person.ID,
		})
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, pg.ID, found[0].ID)
	})

	t.Run("lists with nil filters", func(t *testing.T) {
		// Should not panic with nil filters
		_, err := repo.List(ctx, nil)
		require.NoError(t, err)
	})

	t.Run("lists with is_primary filter", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "ListPrimary", "Test")
		accountID := createTestParentAccount(t, db, "guardian-listprimary")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := createTestPersonGuardian(t, db, person.ID, accountID, users.RelationshipParent, true)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		// List with is_primary filter
		found, err := repo.List(ctx, map[string]interface{}{
			"is_primary": true,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, found)
		// All returned should be primary
		for _, pg := range found {
			assert.True(t, pg.IsPrimary)
		}
	})

	t.Run("lists with relationship_type filter", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "ListRelType", "Test")
		accountID := createTestParentAccount(t, db, "guardian-listreltype")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer cleanupParentAccounts(t, db, accountID)

		pg := createTestPersonGuardian(t, db, person.ID, accountID, users.RelationshipGuardian, false)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		// List with relationship_type filter
		found, err := repo.List(ctx, map[string]interface{}{
			"relationship_type": "guardian",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, found)
		// All returned should be guardians
		for _, pg := range found {
			assert.Equal(t, users.RelationshipGuardian, pg.RelationshipType)
		}
	})
}
