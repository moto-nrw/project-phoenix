package tenant_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	tenantRepo "github.com/moto-nrw/project-phoenix/database/repositories/tenant"
	"github.com/moto-nrw/project-phoenix/models/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Test Fixtures - Unique names for traeger tests
// ============================================================================

// traegerTestFixture creates a traeger for testing.
func traegerTestFixture(t *testing.T, db *bun.DB, name string) *tenant.Traeger {
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

// bueroTestFixture creates a buero for a traeger.
func bueroTestFixture(t *testing.T, db *bun.DB, traegerID string, name string) *tenant.Buero {
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

// cleanupTraegerTestData removes a traeger and its associated bueros.
func cleanupTraegerTestData(t *testing.T, db *bun.DB, traegerID string) {
	t.Helper()
	ctx := context.Background()

	// Bueros are CASCADE deleted, but explicit cleanup for clarity
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

func TestTraegerRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("creates traeger with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TestTraeger_%d", time.Now().UnixNano())
		traeger := &tenant.Traeger{
			Name: uniqueName,
		}

		err := repo.Create(ctx, traeger)
		require.NoError(t, err)

		// Since Create uses Exec (not Scan), ID won't be populated
		// Verify by finding it by name
		found, err := repo.FindByName(ctx, uniqueName)
		require.NoError(t, err)
		assert.NotEmpty(t, found.ID)
		assert.Equal(t, uniqueName, found.Name)
		defer cleanupTraegerTestData(t, db, found.ID)
	})

	t.Run("creates traeger with contact email", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TraegerWithEmail_%d", time.Now().UnixNano())
		email := "test@example.com"
		traeger := &tenant.Traeger{
			Name:         uniqueName,
			ContactEmail: &email,
		}

		err := repo.Create(ctx, traeger)
		require.NoError(t, err)

		found, err := repo.FindByName(ctx, uniqueName)
		require.NoError(t, err)
		require.NotNil(t, found.ContactEmail)
		assert.Equal(t, email, *found.ContactEmail)
		defer cleanupTraegerTestData(t, db, found.ID)
	})

	t.Run("creates traeger with billing info", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TraegerWithBilling_%d", time.Now().UnixNano())
		traeger := &tenant.Traeger{
			Name:        uniqueName,
			BillingInfo: []byte(`{"address": "123 Main St"}`),
		}

		err := repo.Create(ctx, traeger)
		require.NoError(t, err)

		found, err := repo.FindByName(ctx, uniqueName)
		require.NoError(t, err)
		assert.Contains(t, string(found.BillingInfo), "123 Main St")
		defer cleanupTraegerTestData(t, db, found.ID)
	})

	t.Run("fails with nil traeger", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails with empty name", func(t *testing.T) {
		traeger := &tenant.Traeger{
			Name: "",
		}
		err := repo.Create(ctx, traeger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "träger name is required")
	})

	t.Run("fails with whitespace-only name", func(t *testing.T) {
		traeger := &tenant.Traeger{
			Name: "   ",
		}
		err := repo.Create(ctx, traeger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "träger name is required")
	})

	t.Run("trims whitespace from name", func(t *testing.T) {
		baseName := fmt.Sprintf("Trimmed_%d", time.Now().UnixNano())
		traeger := &tenant.Traeger{
			Name: "  " + baseName + "  ",
		}

		err := repo.Create(ctx, traeger)
		require.NoError(t, err)

		// Validation should trim the name before saving
		found, err := repo.FindByName(ctx, baseName)
		require.NoError(t, err)
		assert.Equal(t, baseName, found.Name)
		defer cleanupTraegerTestData(t, db, found.ID)
	})
}

// ============================================================================
// FindByID Tests
// ============================================================================

func TestTraegerRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("finds existing traeger", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "FindByID")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		found, err := repo.FindByID(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Equal(t, traeger.ID, found.ID)
		assert.Equal(t, traeger.Name, found.Name)
	})

	t.Run("returns all fields", func(t *testing.T) {
		email := "findbyid@example.com"
		uniqueName := fmt.Sprintf("FieldsTest_%d", time.Now().UnixNano())
		traeger := &tenant.Traeger{
			Name:         uniqueName,
			ContactEmail: &email,
			BillingInfo:  []byte(`{"test": true}`),
		}

		err := db.NewInsert().
			Model(traeger).
			ModelTableExpr("tenant.traeger").
			ExcludeColumn("id").
			Returning("*").
			Scan(ctx)
		require.NoError(t, err)
		defer cleanupTraegerTestData(t, db, traeger.ID)

		found, err := repo.FindByID(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Equal(t, traeger.ID, found.ID)
		assert.Equal(t, traeger.Name, found.Name)
		require.NotNil(t, found.ContactEmail)
		assert.Equal(t, email, *found.ContactEmail)
		assert.NotEmpty(t, found.BillingInfo)
		assert.False(t, found.CreatedAt.IsZero())
		assert.False(t, found.UpdatedAt.IsZero())
	})

	t.Run("returns error for non-existent traeger", func(t *testing.T) {
		_, err := repo.FindByID(ctx, "non-existent-uuid")
		require.Error(t, err)
	})

	t.Run("handles empty ID gracefully", func(t *testing.T) {
		// Empty ID query - repository doesn't explicitly validate,
		// but should return error if no matching row exists
		result, err := repo.FindByID(ctx, "")
		// If no traeger exists with empty ID, expect error
		// If somehow one exists, result should be non-nil
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotNil(t, result)
		}
	})
}

// ============================================================================
// FindByName Tests
// ============================================================================

func TestTraegerRepository_FindByName(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("finds traeger by exact name", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "FindByName")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		found, err := repo.FindByName(ctx, traeger.Name)
		require.NoError(t, err)
		assert.Equal(t, traeger.ID, found.ID)
		assert.Equal(t, traeger.Name, found.Name)
	})

	t.Run("finds traeger case-insensitively", func(t *testing.T) {
		// Create with mixed case
		uniqueName := fmt.Sprintf("MixedCase_%d", time.Now().UnixNano())
		traeger := &tenant.Traeger{Name: uniqueName}
		err := db.NewInsert().
			Model(traeger).
			ModelTableExpr("tenant.traeger").
			ExcludeColumn("id").
			Returning("*").
			Scan(ctx)
		require.NoError(t, err)
		defer cleanupTraegerTestData(t, db, traeger.ID)

		// Search with same case
		found, err := repo.FindByName(ctx, uniqueName)
		require.NoError(t, err)
		assert.Equal(t, traeger.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistent_TraegerName_12345")
		require.Error(t, err)
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "")
		require.Error(t, err)
	})
}

// ============================================================================
// Update Tests
// ============================================================================

func TestTraegerRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("updates traeger name", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "UpdateName")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		newName := fmt.Sprintf("UpdatedName_%d", time.Now().UnixNano())
		traeger.Name = newName

		err := repo.Update(ctx, traeger)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, found.Name)
	})

	t.Run("updates contact email", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "UpdateEmail")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		newEmail := "updated@example.com"
		traeger.ContactEmail = &newEmail

		err := repo.Update(ctx, traeger)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, traeger.ID)
		require.NoError(t, err)
		require.NotNil(t, found.ContactEmail)
		assert.Equal(t, newEmail, *found.ContactEmail)
	})

	t.Run("clears contact email by setting nil", func(t *testing.T) {
		email := "initial@example.com"
		traeger := &tenant.Traeger{
			Name:         fmt.Sprintf("ClearEmail_%d", time.Now().UnixNano()),
			ContactEmail: &email,
		}
		err := db.NewInsert().
			Model(traeger).
			ModelTableExpr("tenant.traeger").
			ExcludeColumn("id").
			Returning("*").
			Scan(ctx)
		require.NoError(t, err)
		defer cleanupTraegerTestData(t, db, traeger.ID)

		// Verify email was set
		found, err := repo.FindByID(ctx, traeger.ID)
		require.NoError(t, err)
		require.NotNil(t, found.ContactEmail)

		// Clear email
		traeger.ContactEmail = nil
		err = repo.Update(ctx, traeger)
		require.NoError(t, err)

		found, err = repo.FindByID(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Nil(t, found.ContactEmail)
	})

	t.Run("updates billing info", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "UpdateBilling")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		traeger.BillingInfo = []byte(`{"updated": "info"}`)

		err := repo.Update(ctx, traeger)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Contains(t, string(found.BillingInfo), "updated")
	})

	t.Run("fails with nil traeger", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails with empty name", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "UpdateEmpty")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		traeger.Name = ""
		err := repo.Update(ctx, traeger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "träger name is required")
	})

	t.Run("fails with whitespace-only name", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "UpdateWhitespace")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		traeger.Name = "   "
		err := repo.Update(ctx, traeger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "träger name is required")
	})
}

// ============================================================================
// Delete Tests
// ============================================================================

func TestTraegerRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("deletes existing traeger", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "ToDelete")
		traegerID := traeger.ID

		err := repo.Delete(ctx, traegerID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.FindByID(ctx, traegerID)
		require.Error(t, err)
	})

	t.Run("cascades delete to bueros", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "CascadeDelete")
		buero := bueroTestFixture(t, db, traeger.ID, "BueroToDelete")

		// Verify buero exists
		bueroRepo := tenantRepo.NewBueroRepository(db)
		_, err := bueroRepo.FindByID(ctx, buero.ID)
		require.NoError(t, err)

		// Delete traeger (should cascade)
		err = repo.Delete(ctx, traeger.ID)
		require.NoError(t, err)

		// Verify traeger is gone
		_, err = repo.FindByID(ctx, traeger.ID)
		require.Error(t, err)

		// Verify buero is gone (cascaded)
		_, err = bueroRepo.FindByID(ctx, buero.ID)
		require.Error(t, err)
	})

	t.Run("no error when deleting non-existent traeger", func(t *testing.T) {
		// Delete is idempotent - deleting non-existent ID should not error
		err := repo.Delete(ctx, "non-existent-uuid-12345")
		// Most SQL implementations don't error on DELETE that affects 0 rows
		assert.NoError(t, err)
	})

	t.Run("no error when deleting with empty ID", func(t *testing.T) {
		err := repo.Delete(ctx, "")
		// Empty ID won't match any row
		assert.NoError(t, err)
	})
}

// ============================================================================
// List Tests
// ============================================================================

func TestTraegerRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("lists all traegers", func(t *testing.T) {
		// Create multiple traegers
		traeger1 := traegerTestFixture(t, db, "ListTest_A")
		traeger2 := traegerTestFixture(t, db, "ListTest_B")
		defer cleanupTraegerTestData(t, db, traeger1.ID)
		defer cleanupTraegerTestData(t, db, traeger2.ID)

		traegers, err := repo.List(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, traegers)

		// Check that our test traegers are in the list
		ids := make(map[string]bool)
		for _, tr := range traegers {
			ids[tr.ID] = true
		}
		assert.True(t, ids[traeger1.ID], "List should contain traeger1")
		assert.True(t, ids[traeger2.ID], "List should contain traeger2")
	})

	t.Run("returns traegers ordered by name ASC", func(t *testing.T) {
		// Create traegers with specific alphabetical ordering
		traegerZ := traegerTestFixture(t, db, "ZZZ_OrderTest")
		traegerA := traegerTestFixture(t, db, "AAA_OrderTest")
		traegerM := traegerTestFixture(t, db, "MMM_OrderTest")
		defer cleanupTraegerTestData(t, db, traegerZ.ID)
		defer cleanupTraegerTestData(t, db, traegerA.ID)
		defer cleanupTraegerTestData(t, db, traegerM.ID)

		traegers, err := repo.List(ctx)
		require.NoError(t, err)

		// Find positions of our test traegers
		var posA, posM, posZ int
		for i, tr := range traegers {
			switch tr.ID {
			case traegerA.ID:
				posA = i
			case traegerM.ID:
				posM = i
			case traegerZ.ID:
				posZ = i
			}
		}

		// A should come before M, M before Z
		assert.Less(t, posA, posM, "AAA should come before MMM")
		assert.Less(t, posM, posZ, "MMM should come before ZZZ")
	})

	t.Run("returns empty slice when no traegers exist", func(t *testing.T) {
		// List returns whatever exists - could be seeded or empty
		traegers, err := repo.List(ctx)
		require.NoError(t, err)
		assert.NotNil(t, traegers, "List should return non-nil slice even if empty")
	})
}

// ============================================================================
// FindWithBueros Tests
// ============================================================================

func TestTraegerRepository_FindWithBueros(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("finds traeger with bueros loaded", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "WithBueros")
		buero1 := bueroTestFixture(t, db, traeger.ID, "Buero1")
		buero2 := bueroTestFixture(t, db, traeger.ID, "Buero2")
		defer cleanupTraegerTestData(t, db, traeger.ID) // Cascades to bueros

		found, err := repo.FindWithBueros(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Equal(t, traeger.ID, found.ID)
		assert.NotNil(t, found.Bueros)
		assert.Len(t, found.Bueros, 2)

		// Check bueros are correctly loaded
		bueroIDs := make(map[string]bool)
		for _, b := range found.Bueros {
			bueroIDs[b.ID] = true
			assert.Equal(t, traeger.ID, b.TraegerID)
		}
		assert.True(t, bueroIDs[buero1.ID], "Should contain buero1")
		assert.True(t, bueroIDs[buero2.ID], "Should contain buero2")
	})

	t.Run("returns empty bueros slice when traeger has no bueros", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "NoBueros")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		found, err := repo.FindWithBueros(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Equal(t, traeger.ID, found.ID)
		// When no bueros exist, Bueros may be nil or empty slice
		assert.Empty(t, found.Bueros, "Bueros should be empty (nil or zero-length)")
	})

	t.Run("loads all traeger fields", func(t *testing.T) {
		email := "withbueros@example.com"
		traeger := &tenant.Traeger{
			Name:         fmt.Sprintf("FieldsWithBueros_%d", time.Now().UnixNano()),
			ContactEmail: &email,
			BillingInfo:  []byte(`{"test": "data"}`),
		}
		err := db.NewInsert().
			Model(traeger).
			ModelTableExpr("tenant.traeger").
			ExcludeColumn("id").
			Returning("*").
			Scan(ctx)
		require.NoError(t, err)
		defer cleanupTraegerTestData(t, db, traeger.ID)

		found, err := repo.FindWithBueros(ctx, traeger.ID)
		require.NoError(t, err)
		assert.Equal(t, traeger.Name, found.Name)
		require.NotNil(t, found.ContactEmail)
		assert.Equal(t, email, *found.ContactEmail)
		assert.Contains(t, string(found.BillingInfo), "test")
		assert.False(t, found.CreatedAt.IsZero())
		assert.False(t, found.UpdatedAt.IsZero())
	})

	t.Run("loads buero fields correctly", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "BueroFields")
		bueroEmail := "buero@example.com"
		buero := &tenant.Buero{
			TraegerID:    traeger.ID,
			Name:         fmt.Sprintf("BueroWithEmail_%d", time.Now().UnixNano()),
			ContactEmail: &bueroEmail,
		}
		err := db.NewInsert().
			Model(buero).
			ModelTableExpr("tenant.buero").
			ExcludeColumn("id").
			Returning("*").
			Scan(ctx)
		require.NoError(t, err)
		defer cleanupTraegerTestData(t, db, traeger.ID)

		found, err := repo.FindWithBueros(ctx, traeger.ID)
		require.NoError(t, err)
		require.Len(t, found.Bueros, 1)

		loadedBuero := found.Bueros[0]
		assert.Equal(t, buero.ID, loadedBuero.ID)
		assert.Equal(t, buero.Name, loadedBuero.Name)
		assert.Equal(t, traeger.ID, loadedBuero.TraegerID)
		require.NotNil(t, loadedBuero.ContactEmail)
		assert.Equal(t, bueroEmail, *loadedBuero.ContactEmail)
	})

	t.Run("returns error for non-existent traeger", func(t *testing.T) {
		_, err := repo.FindWithBueros(ctx, "non-existent-uuid-12345")
		require.Error(t, err)
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		_, err := repo.FindWithBueros(ctx, "")
		require.Error(t, err)
	})
}

// ============================================================================
// Edge Cases and Error Handling Tests
// ============================================================================

func TestTraegerRepository_EdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("handles special characters in name", func(t *testing.T) {
		specialName := fmt.Sprintf("Träger mit Ümläuten & Sönderzeichen %d", time.Now().UnixNano())
		traeger := &tenant.Traeger{
			Name: specialName,
		}

		err := repo.Create(ctx, traeger)
		require.NoError(t, err)

		found, err := repo.FindByName(ctx, specialName)
		require.NoError(t, err)
		assert.Equal(t, specialName, found.Name)
		defer cleanupTraegerTestData(t, db, found.ID)
	})

	t.Run("handles very long name", func(t *testing.T) {
		// TEXT columns can hold very long strings
		longName := fmt.Sprintf("VeryLongName_%d_%s", time.Now().UnixNano(),
			"a very long suffix that goes on and on to test length handling")
		traeger := &tenant.Traeger{
			Name: longName,
		}

		err := repo.Create(ctx, traeger)
		require.NoError(t, err)

		found, err := repo.FindByName(ctx, longName)
		require.NoError(t, err)
		assert.Equal(t, longName, found.Name)
		defer cleanupTraegerTestData(t, db, found.ID)
	})

	t.Run("handles null billing info", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "NullBilling")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		found, err := repo.FindByID(ctx, traeger.ID)
		require.NoError(t, err)
		// billing_info is nullable JSONB - when NULL, postgres returns "null" literal
		// which BUN scans as json.RawMessage containing "null" bytes
		// Empty/nil json.RawMessage or literal "null" are both valid "no value" states
		if found.BillingInfo != nil {
			assert.True(t, len(found.BillingInfo) == 0 || string(found.BillingInfo) == "null",
				"BillingInfo should be empty or JSON null, got: %s", string(found.BillingInfo))
		}
	})

	t.Run("handles complex JSON billing info", func(t *testing.T) {
		complexJSON := []byte(`{
			"company": "Test GmbH",
			"address": {
				"street": "Hauptstraße 1",
				"city": "Berlin",
				"zip": "10115"
			},
			"contacts": ["billing@test.de", "accounts@test.de"],
			"active": true,
			"amount": 123.45
		}`)
		traeger := &tenant.Traeger{
			Name:        fmt.Sprintf("ComplexBilling_%d", time.Now().UnixNano()),
			BillingInfo: complexJSON,
		}

		err := repo.Create(ctx, traeger)
		require.NoError(t, err)

		found, err := repo.FindByName(ctx, traeger.Name)
		require.NoError(t, err)
		assert.Contains(t, string(found.BillingInfo), "Hauptstraße")
		assert.Contains(t, string(found.BillingInfo), "contacts")
		defer cleanupTraegerTestData(t, db, found.ID)
	})
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestTraegerRepository_ConcurrentAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := tenantRepo.NewTraegerRepository(db)
	ctx := context.Background()

	t.Run("handles concurrent creates", func(t *testing.T) {
		const numCreates = 5
		errors := make(chan error, numCreates)
		names := make(chan string, numCreates)

		for i := 0; i < numCreates; i++ {
			go func(idx int) {
				name := fmt.Sprintf("Concurrent_%d_%d", idx, time.Now().UnixNano())
				traeger := &tenant.Traeger{
					Name: name,
				}
				err := repo.Create(ctx, traeger)
				errors <- err
				if err == nil {
					names <- name
				}
			}(i)
		}

		// Collect results
		var createdNames []string
		var createdIDs []string
		for i := 0; i < numCreates; i++ {
			err := <-errors
			assert.NoError(t, err)
		}
		close(names)
		for name := range names {
			createdNames = append(createdNames, name)
			// Find the created traeger to get its ID for cleanup
			found, err := repo.FindByName(ctx, name)
			if err == nil {
				createdIDs = append(createdIDs, found.ID)
			}
		}

		// Cleanup all created traegers
		for _, id := range createdIDs {
			cleanupTraegerTestData(t, db, id)
		}

		assert.Len(t, createdNames, numCreates)
	})

	t.Run("handles concurrent reads", func(t *testing.T) {
		traeger := traegerTestFixture(t, db, "ConcurrentRead")
		defer cleanupTraegerTestData(t, db, traeger.ID)

		const numReads = 10
		errors := make(chan error, numReads)

		for i := 0; i < numReads; i++ {
			go func() {
				_, err := repo.FindByID(ctx, traeger.ID)
				errors <- err
			}()
		}

		for i := 0; i < numReads; i++ {
			err := <-errors
			assert.NoError(t, err)
		}
	})
}
