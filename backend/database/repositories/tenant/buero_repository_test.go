package tenant_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupBueroRepo(_ *testing.T, db *bun.DB) tenant.BueroRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.Buero
}

// createTestTraegerForBuero creates a traeger for testing buero functionality
func createTestTraegerForBuero(t *testing.T, db *bun.DB, name string) *tenant.Traeger {
	t.Helper()
	ctx := context.Background()

	uniqueName := fmt.Sprintf("%s_%d", name, time.Now().UnixNano())
	traeger := &tenant.Traeger{
		Name: uniqueName,
	}

	// Use ExcludeColumn to let database generate the ID via DEFAULT
	err := db.NewInsert().
		Model(traeger).
		ModelTableExpr("tenant.traeger").
		ExcludeColumn("id").
		Returning("*").
		Scan(ctx)
	require.NoError(t, err, "Failed to create test traeger")

	return traeger
}

// createTestBueroForTest creates a buero for a traeger
func createTestBueroForTest(t *testing.T, db *bun.DB, traegerID string, name string) *tenant.Buero {
	t.Helper()
	ctx := context.Background()

	uniqueName := fmt.Sprintf("%s_%d", name, time.Now().UnixNano())
	buero := &tenant.Buero{
		TraegerID: traegerID,
		Name:      uniqueName,
	}

	// Use ExcludeColumn to let database generate the ID via DEFAULT
	err := db.NewInsert().
		Model(buero).
		ModelTableExpr("tenant.buero").
		ExcludeColumn("id").
		Returning("*").
		Scan(ctx)
	require.NoError(t, err, "Failed to create test buero")

	return buero
}

// cleanupTraegerForBuero removes a traeger and its associated bueros
func cleanupTraegerForBuero(t *testing.T, db *bun.DB, traegerID string) {
	t.Helper()
	ctx := context.Background()

	// Bueros are CASCADE deleted, but explicit cleanup for clarity
	// Use TableExpr (not ModelTableExpr) to avoid BeforeAppendModel hook conflicts
	_, _ = db.NewDelete().
		TableExpr("tenant.buero").
		Where("traeger_id = ?", traegerID).
		Exec(ctx)

	_, _ = db.NewDelete().
		TableExpr("tenant.traeger").
		Where("id = ?", traegerID).
		Exec(ctx)
}

// ============================================================================
// Create Tests
// ============================================================================

func TestBueroRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupBueroRepo(t, db)
	ctx := context.Background()

	t.Run("creates buero with valid data", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroCreateTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		uniqueName := fmt.Sprintf("Test Büro %d", time.Now().UnixNano())
		buero := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      uniqueName,
		}

		err := repo.Create(ctx, buero)
		require.NoError(t, err)

		// Since Create uses Exec (not Returning), verify creation via FindByTraegerID
		bueros, err := repo.FindByTraegerID(ctx, traeger.ID)
		require.NoError(t, err)
		found := false
		for _, b := range bueros {
			if b.Name == uniqueName {
				found = true
				assert.NotEmpty(t, b.ID)
				break
			}
		}
		assert.True(t, found, "Created buero should be found")
	})

	t.Run("creates buero with contact email", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroCreateEmailTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		email := "contact@test.local"
		uniqueName := fmt.Sprintf("Büro with Email %d", time.Now().UnixNano())
		buero := &tenant.Buero{
			TraegerID:    traeger.ID,
			Name:         uniqueName,
			ContactEmail: &email,
		}

		err := repo.Create(ctx, buero)
		require.NoError(t, err)

		// Since Create uses Exec (not Returning), verify creation via FindByTraegerID
		bueros, err := repo.FindByTraegerID(ctx, traeger.ID)
		require.NoError(t, err)
		for _, b := range bueros {
			if b.Name == uniqueName {
				assert.NotEmpty(t, b.ID)
				require.NotNil(t, b.ContactEmail)
				assert.Equal(t, email, *b.ContactEmail)
				return
			}
		}
		t.Fatal("Created buero with email should be found")
	})

	t.Run("create with nil buero should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with empty name should fail validation", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroEmptyNameTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      "",
		}

		err := repo.Create(ctx, buero)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("create with whitespace only name should fail validation", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroWhitespaceNameTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      "   ",
		}

		err := repo.Create(ctx, buero)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("create with empty traeger_id should fail validation", func(t *testing.T) {
		buero := &tenant.Buero{
			TraegerID: "",
			Name:      "Test Büro",
		}

		err := repo.Create(ctx, buero)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "träger ID is required")
	})

	t.Run("create with whitespace only traeger_id should fail validation", func(t *testing.T) {
		buero := &tenant.Buero{
			TraegerID: "   ",
			Name:      "Test Büro",
		}

		err := repo.Create(ctx, buero)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "träger ID is required")
	})

	t.Run("validation trims whitespace from name", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroTrimTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      "  Trimmed Büro Name  ",
		}

		err := repo.Create(ctx, buero)
		require.NoError(t, err)
		assert.Equal(t, "Trimmed Büro Name", buero.Name)
	})
}

// ============================================================================
// FindByID Tests
// ============================================================================

func TestBueroRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupBueroRepo(t, db)
	ctx := context.Background()

	t.Run("finds existing buero", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroFindByIDTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "FindByID Büro")

		found, err := repo.FindByID(ctx, buero.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, buero.ID, found.ID)
		assert.Equal(t, buero.Name, found.Name)
		assert.Equal(t, buero.TraegerID, found.TraegerID)
	})

	t.Run("returns error for non-existent buero", func(t *testing.T) {
		found, err := repo.FindByID(ctx, "non-existent-id-12345")
		// BUN returns sql.ErrNoRows wrapped in DatabaseError
		assert.Error(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns error for empty id", func(t *testing.T) {
		found, err := repo.FindByID(ctx, "")
		// Empty ID won't match any record
		assert.Error(t, err)
		assert.Nil(t, found)
	})
}

// ============================================================================
// FindByTraegerID Tests
// ============================================================================

func TestBueroRepository_FindByTraegerID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupBueroRepo(t, db)
	ctx := context.Background()

	t.Run("finds all bueros for traeger", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroFindByTraegerTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		// Create multiple bueros for the same traeger
		buero1 := createTestBueroForTest(t, db, traeger.ID, "A-Büro")
		buero2 := createTestBueroForTest(t, db, traeger.ID, "B-Büro")

		bueros, err := repo.FindByTraegerID(ctx, traeger.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(bueros), 2)

		// Verify all returned bueros belong to this traeger
		for _, b := range bueros {
			assert.Equal(t, traeger.ID, b.TraegerID)
		}

		// Verify our bueros are in the list
		foundIDs := make(map[string]bool)
		for _, b := range bueros {
			foundIDs[b.ID] = true
		}
		assert.True(t, foundIDs[buero1.ID], "Büro 1 should be in list")
		assert.True(t, foundIDs[buero2.ID], "Büro 2 should be in list")
	})

	t.Run("returns bueros sorted by name", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroSortTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		// Create bueros with names that will sort predictably
		timestamp := time.Now().UnixNano()
		alphaName := fmt.Sprintf("Alpha Büro %d", timestamp)
		zebraName := fmt.Sprintf("Zebra Büro %d", timestamp)
		bueroZ := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      zebraName,
		}
		bueroA := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      alphaName,
		}

		err := repo.Create(ctx, bueroZ)
		require.NoError(t, err)
		err = repo.Create(ctx, bueroA)
		require.NoError(t, err)

		bueros, err := repo.FindByTraegerID(ctx, traeger.ID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(bueros), 2)

		// Find the positions of our test bueros by name (since Create doesn't return ID)
		var alphaIdx, zebraIdx = -1, -1
		for i, b := range bueros {
			if b.Name == alphaName {
				alphaIdx = i
			}
			if b.Name == zebraName {
				zebraIdx = i
			}
		}

		// Alpha should come before Zebra
		require.NotEqual(t, -1, alphaIdx, "Alpha Büro should be found")
		require.NotEqual(t, -1, zebraIdx, "Zebra Büro should be found")
		assert.True(t, alphaIdx < zebraIdx, "Expected Alpha Büro to come before Zebra Büro")
	})

	t.Run("returns empty slice for non-existent traeger", func(t *testing.T) {
		bueros, err := repo.FindByTraegerID(ctx, "non-existent-traeger-id")
		require.NoError(t, err)
		assert.Empty(t, bueros)
	})

	t.Run("returns empty slice for traeger with no bueros", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "EmptyTraegerTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		bueros, err := repo.FindByTraegerID(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Empty(t, bueros)
	})
}

// ============================================================================
// Update Tests
// ============================================================================

func TestBueroRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupBueroRepo(t, db)
	ctx := context.Background()

	t.Run("updates buero name", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroUpdateNameTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "Original Name")

		buero.Name = "Updated Name"
		err := repo.Update(ctx, buero)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, buero.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", found.Name)
	})

	t.Run("updates buero contact email", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroUpdateEmailTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "Email Update Test")

		newEmail := "updated@test.local"
		buero.ContactEmail = &newEmail
		err := repo.Update(ctx, buero)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, buero.ID)
		require.NoError(t, err)
		require.NotNil(t, found.ContactEmail)
		assert.Equal(t, newEmail, *found.ContactEmail)
	})

	t.Run("update with nil buero should fail", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("update with empty name should fail validation", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroUpdateEmptyNameTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "Original Name")

		buero.Name = ""
		err := repo.Update(ctx, buero)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("update with empty traeger_id should fail validation", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroUpdateEmptyTraegerTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "Original Name")

		buero.TraegerID = ""
		err := repo.Update(ctx, buero)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "träger ID is required")
	})

	t.Run("update trims whitespace from name", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroUpdateTrimTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "Original Name")

		buero.Name = "  Trimmed Update Name  "
		err := repo.Update(ctx, buero)
		require.NoError(t, err)
		assert.Equal(t, "Trimmed Update Name", buero.Name)
	})
}

// ============================================================================
// Delete Tests
// ============================================================================

func TestBueroRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupBueroRepo(t, db)
	ctx := context.Background()

	t.Run("deletes existing buero", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroDeleteTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "To Be Deleted")
		bueroID := buero.ID

		err := repo.Delete(ctx, bueroID)
		require.NoError(t, err)

		// Verify deletion
		found, err := repo.FindByID(ctx, bueroID)
		assert.Error(t, err)
		assert.Nil(t, found)
	})

	t.Run("delete non-existent buero does not error", func(t *testing.T) {
		// Deleting a non-existent record should not return an error
		// This is standard SQL behavior - DELETE affects 0 rows
		err := repo.Delete(ctx, "non-existent-buero-id-12345")
		require.NoError(t, err)
	})

	t.Run("delete with empty id does not error", func(t *testing.T) {
		// Empty ID will match 0 rows, which is not an error
		err := repo.Delete(ctx, "")
		require.NoError(t, err)
	})
}

// ============================================================================
// List Tests
// ============================================================================

func TestBueroRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupBueroRepo(t, db)
	ctx := context.Background()

	t.Run("lists all bueros", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroListTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "List Test Büro")

		bueros, err := repo.List(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, bueros)

		// Verify our created buero is in the list
		found := false
		for _, b := range bueros {
			if b.ID == buero.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Created buero should be in the list")
	})

	t.Run("list returns bueros sorted by name", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroListSortTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		timestamp := time.Now().UnixNano()
		alphaName := fmt.Sprintf("AAA-Büro %d", timestamp)
		zebraName := fmt.Sprintf("ZZZ-Büro %d", timestamp)
		bueroZ := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      zebraName,
		}
		bueroA := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      alphaName,
		}

		err := repo.Create(ctx, bueroZ)
		require.NoError(t, err)
		err = repo.Create(ctx, bueroA)
		require.NoError(t, err)

		bueros, err := repo.List(ctx)
		require.NoError(t, err)

		// Find positions by name (since Create doesn't return ID)
		var alphaIdx, zebraIdx = -1, -1
		for i, b := range bueros {
			if b.Name == alphaName {
				alphaIdx = i
			}
			if b.Name == zebraName {
				zebraIdx = i
			}
		}

		// AAA should come before ZZZ
		require.NotEqual(t, -1, alphaIdx, "AAA-Büro should be found")
		require.NotEqual(t, -1, zebraIdx, "ZZZ-Büro should be found")
		assert.True(t, alphaIdx < zebraIdx, "AAA-Büro should come before ZZZ-Büro")
	})

	t.Run("list returns bueros from multiple traegers", func(t *testing.T) {
		traeger1 := createTestTraegerForBuero(t, db, "BueroListMultiTraeger1")
		traeger2 := createTestTraegerForBuero(t, db, "BueroListMultiTraeger2")
		defer cleanupTraegerForBuero(t, db, traeger1.ID)
		defer cleanupTraegerForBuero(t, db, traeger2.ID)

		buero1 := createTestBueroForTest(t, db, traeger1.ID, "Multi-Traeger Büro 1")
		buero2 := createTestBueroForTest(t, db, traeger2.ID, "Multi-Traeger Büro 2")

		bueros, err := repo.List(ctx)
		require.NoError(t, err)

		// Both bueros should be in the list
		foundIDs := make(map[string]bool)
		for _, b := range bueros {
			foundIDs[b.ID] = true
		}

		assert.True(t, foundIDs[buero1.ID], "Büro from Träger 1 should be in list")
		assert.True(t, foundIDs[buero2.ID], "Büro from Träger 2 should be in list")
	})
}

// ============================================================================
// Edge Cases and Error Handling
// ============================================================================

func TestBueroRepository_EdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupBueroRepo(t, db)
	ctx := context.Background()

	t.Run("create with invalid traeger_id returns database error", func(t *testing.T) {
		// FK constraint should prevent creation with non-existent traeger
		buero := &tenant.Buero{
			TraegerID: "definitely-non-existent-traeger-id",
			Name:      "Should Fail FK",
		}

		err := repo.Create(ctx, buero)
		assert.Error(t, err)
		// The error should be wrapped as a DatabaseError
	})

	t.Run("buero with special characters in name", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroSpecialCharTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		specialName := "Büro mit Ümläuten & Sønderzeichen ß € @"
		buero := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      specialName,
		}

		err := repo.Create(ctx, buero)
		require.NoError(t, err)

		// Since Create doesn't return ID, verify via FindByTraegerID
		bueros, err := repo.FindByTraegerID(ctx, traeger.ID)
		require.NoError(t, err)
		var found *tenant.Buero
		for _, b := range bueros {
			if b.Name == specialName {
				found = b
				break
			}
		}
		require.NotNil(t, found, "Created buero should be found")
		assert.Equal(t, specialName, found.Name)
	})

	t.Run("buero with very long name", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroLongNameTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		// Create a 200 character name with unique suffix
		longName := ""
		for i := 0; i < 190; i++ {
			longName += "A"
		}
		longName += fmt.Sprintf("%d", time.Now().UnixNano()%1000000000)

		buero := &tenant.Buero{
			TraegerID: traeger.ID,
			Name:      longName,
		}

		err := repo.Create(ctx, buero)
		require.NoError(t, err)

		// Since Create doesn't return ID, verify via FindByTraegerID
		bueros, err := repo.FindByTraegerID(ctx, traeger.ID)
		require.NoError(t, err)
		var found *tenant.Buero
		for _, b := range bueros {
			if b.Name == longName {
				found = b
				break
			}
		}
		require.NotNil(t, found, "Created buero should be found")
		assert.GreaterOrEqual(t, len(found.Name), 190)
	})

	t.Run("timestamps are set on create", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroTimestampTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		beforeCreate := time.Now().Add(-time.Second)
		// Use fixture helper which returns ID via Returning("*").Scan()
		buero := createTestBueroForTest(t, db, traeger.ID, "Timestamp Test Büro")

		found, err := repo.FindByID(ctx, buero.ID)
		require.NoError(t, err)

		assert.True(t, found.CreatedAt.After(beforeCreate), "CreatedAt should be set")
		assert.True(t, found.UpdatedAt.After(beforeCreate), "UpdatedAt should be set")
	})

	t.Run("can clear contact email by setting to nil", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroClearEmailTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		// Use fixture helper which returns ID
		buero := createTestBueroForTest(t, db, traeger.ID, "Clear Email Test")

		// Set email via Update
		email := "initial@test.local"
		buero.ContactEmail = &email
		err := repo.Update(ctx, buero)
		require.NoError(t, err)

		// Verify email was set
		found, err := repo.FindByID(ctx, buero.ID)
		require.NoError(t, err)
		require.NotNil(t, found.ContactEmail)
		assert.Equal(t, email, *found.ContactEmail)

		// Clear email
		found.ContactEmail = nil
		err = repo.Update(ctx, found)
		require.NoError(t, err)

		final, err := repo.FindByID(ctx, buero.ID)
		require.NoError(t, err)
		assert.Nil(t, final.ContactEmail)
	})
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestBueroRepository_ConcurrentAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupBueroRepo(t, db)
	ctx := context.Background()

	t.Run("handles concurrent creates", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroConcurrentTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		const numCreates = 5
		errors := make(chan error, numCreates)
		bueros := make(chan *tenant.Buero, numCreates)

		for i := 0; i < numCreates; i++ {
			go func(idx int) {
				buero := &tenant.Buero{
					TraegerID: traeger.ID,
					Name:      fmt.Sprintf("Concurrent_%d_%d", idx, time.Now().UnixNano()),
				}
				err := repo.Create(ctx, buero)
				errors <- err
				if err == nil {
					bueros <- buero
				}
			}(i)
		}

		// Collect results
		var created []*tenant.Buero
		for i := 0; i < numCreates; i++ {
			err := <-errors
			assert.NoError(t, err)
		}
		close(bueros)
		for b := range bueros {
			created = append(created, b)
		}

		assert.Len(t, created, numCreates)
	})

	t.Run("handles concurrent reads", func(t *testing.T) {
		traeger := createTestTraegerForBuero(t, db, "BueroConcurrentReadTest")
		defer cleanupTraegerForBuero(t, db, traeger.ID)

		buero := createTestBueroForTest(t, db, traeger.ID, "ConcurrentRead")

		const numReads = 10
		errors := make(chan error, numReads)

		for i := 0; i < numReads; i++ {
			go func() {
				_, err := repo.FindByID(ctx, buero.ID)
				errors <- err
			}()
		}

		for i := 0; i < numReads; i++ {
			err := <-errors
			assert.NoError(t, err)
		}
	})
}
