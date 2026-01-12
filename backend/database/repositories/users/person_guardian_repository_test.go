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
// Setup Helpers
// ============================================================================

// createTestPersonGuardian creates a person guardian relationship for testing
func createTestPersonGuardian(t *testing.T, db *bun.DB, personID, guardianAccountID int64, relType users.RelationshipType, isPrimary bool) *users.PersonGuardian {
	t.Helper()
	ctx := context.Background()

	pg := &users.PersonGuardian{
		PersonID:          personID,
		GuardianAccountID: guardianAccountID,
		RelationshipType:  relType,
		IsPrimary:         isPrimary,
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
		account := testpkg.CreateTestAccount(t, db, "guardian-create")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := &users.PersonGuardian{
			PersonID:          person.ID,
			GuardianAccountID: account.ID,
			RelationshipType:  users.RelationshipParent,
			IsPrimary:         true,
		}

		err := repo.Create(ctx, pg)
		require.NoError(t, err)
		assert.NotZero(t, pg.ID)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		// Verify in DB
		found, err := repo.FindByID(ctx, pg.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, found.PersonID)
		assert.Equal(t, account.ID, found.GuardianAccountID)
		assert.Equal(t, users.RelationshipParent, found.RelationshipType)
	})

	t.Run("fails with nil person guardian", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("fails with missing person ID", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "guardian-no-person")
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := &users.PersonGuardian{
			PersonID:          0,
			GuardianAccountID: account.ID,
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
		account := testpkg.CreateTestAccount(t, db, "guardian-findperson")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := createTestPersonGuardian(t, db, person.ID, account.ID, users.RelationshipParent, true)
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
		account := testpkg.CreateTestAccount(t, db, "guardian-findguardian")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := createTestPersonGuardian(t, db, person.ID, account.ID, users.RelationshipParent, true)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		found, err := repo.FindByGuardianID(ctx, account.ID)
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
		account := testpkg.CreateTestAccount(t, db, "guardian-primary")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := createTestPersonGuardian(t, db, person.ID, account.ID, users.RelationshipParent, true)
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
		account1 := testpkg.CreateTestAccount(t, db, "guardian-reltype1")
		account2 := testpkg.CreateTestAccount(t, db, "guardian-reltype2")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id IN (?)", bun.In([]int64{account1.ID, account2.ID})).Exec(ctx)
		}()

		// Create parent relationship
		pgParent := createTestPersonGuardian(t, db, person.ID, account1.ID, users.RelationshipParent, true)
		// Create guardian relationship
		pgGuardian := createTestPersonGuardian(t, db, person.ID, account2.ID, users.RelationshipGuardian, false)
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
		account := testpkg.CreateTestAccount(t, db, "guardian-setprimary")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := createTestPersonGuardian(t, db, person.ID, account.ID, users.RelationshipParent, false)
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
		account := testpkg.CreateTestAccount(t, db, "guardian-updateperms")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := createTestPersonGuardian(t, db, person.ID, account.ID, users.RelationshipParent, true)
		defer cleanupPersonGuardianRecords(t, db, pg.ID)

		// Update permissions
		newPerms := `{"view_attendance":true,"view_grades":true}`
		err := repo.UpdatePermissions(ctx, pg.ID, newPerms)
		require.NoError(t, err)

		// Verify update
		found, err := repo.FindByID(ctx, pg.ID)
		require.NoError(t, err)
		assert.Equal(t, newPerms, found.Permissions)
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
		account := testpkg.CreateTestAccount(t, db, "guardian-listopts")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := createTestPersonGuardian(t, db, person.ID, account.ID, users.RelationshipParent, true)
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
		account := testpkg.CreateTestAccount(t, db, "guardian-listprimary")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := createTestPersonGuardian(t, db, person.ID, account.ID, users.RelationshipParent, true)
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
		account := testpkg.CreateTestAccount(t, db, "guardian-listreltype")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)
		defer func() {
			_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
		}()

		pg := createTestPersonGuardian(t, db, person.ID, account.ID, users.RelationshipGuardian, false)
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
