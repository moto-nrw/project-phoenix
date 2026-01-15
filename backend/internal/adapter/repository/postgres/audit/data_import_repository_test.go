package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestDataImportRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataImport
	ctx := context.Background()

	// Create a test account for the imported_by FK
	account := testpkg.CreateTestAccount(t, db, "import_test@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("creates data import with valid data", func(t *testing.T) {
		now := time.Now()
		completedAt := now.Add(5 * time.Minute)
		dataImport := &audit.DataImport{
			EntityType:   "student",
			Filename:     "students_2024.csv",
			TotalRows:    100,
			CreatedCount: 80,
			UpdatedCount: 15,
			SkippedCount: 3,
			ErrorCount:   2,
			DryRun:       false,
			ImportedBy:   account.ID,
			StartedAt:    now,
			CompletedAt:  &completedAt,
		}

		err := repo.Create(ctx, dataImport)
		require.NoError(t, err)
		assert.NotZero(t, dataImport.ID)

		testpkg.CleanupTableRecords(t, db, "audit.data_imports", dataImport.ID)
	})

	t.Run("creates dry run import", func(t *testing.T) {
		now := time.Now()
		dataImport := &audit.DataImport{
			EntityType:   "teacher",
			Filename:     "teachers_preview.xlsx",
			TotalRows:    50,
			CreatedCount: 0,
			UpdatedCount: 0,
			SkippedCount: 50,
			ErrorCount:   0,
			DryRun:       true,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}

		err := repo.Create(ctx, dataImport)
		require.NoError(t, err)
		assert.NotZero(t, dataImport.ID)
		assert.True(t, dataImport.DryRun)

		testpkg.CleanupTableRecords(t, db, "audit.data_imports", dataImport.ID)
	})

	t.Run("creates import with metadata", func(t *testing.T) {
		now := time.Now()
		dataImport := &audit.DataImport{
			EntityType:   "room",
			Filename:     "rooms.csv",
			TotalRows:    30,
			CreatedCount: 25,
			UpdatedCount: 5,
			WarningCount: 2,
			DryRun:       false,
			ImportedBy:   account.ID,
			StartedAt:    now,
			Metadata: audit.JSONBMap{
				"source":   "admin_panel",
				"warnings": []string{"Row 5: Room name truncated", "Row 12: Invalid capacity"},
			},
		}

		err := repo.Create(ctx, dataImport)
		require.NoError(t, err)
		assert.NotZero(t, dataImport.ID)

		testpkg.CleanupTableRecords(t, db, "audit.data_imports", dataImport.ID)
	})
}

func TestDataImportRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataImport
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "find_import@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds existing data import", func(t *testing.T) {
		now := time.Now()
		dataImport := &audit.DataImport{
			EntityType:   "student",
			Filename:     "find_test.csv",
			TotalRows:    10,
			CreatedCount: 10,
			DryRun:       false,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}
		err := repo.Create(ctx, dataImport)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", dataImport.ID)

		found, err := repo.FindByID(ctx, dataImport.ID)
		require.NoError(t, err)
		assert.Equal(t, dataImport.ID, found.ID)
		assert.Equal(t, "find_test.csv", found.Filename)
	})

	t.Run("returns error for non-existent import", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestDataImportRepository_FindByImportedBy(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataImport
	ctx := context.Background()

	account1 := testpkg.CreateTestAccount(t, db, "importer1@example.com")
	account2 := testpkg.CreateTestAccount(t, db, "importer2@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account1.ID, account2.ID)

	t.Run("finds imports by account", func(t *testing.T) {
		now := time.Now()
		import1 := &audit.DataImport{
			EntityType:   "student",
			Filename:     "students1.csv",
			TotalRows:    50,
			CreatedCount: 50,
			ImportedBy:   account1.ID,
			StartedAt:    now,
		}
		import2 := &audit.DataImport{
			EntityType:   "student",
			Filename:     "students2.csv",
			TotalRows:    30,
			CreatedCount: 30,
			ImportedBy:   account1.ID,
			StartedAt:    now,
		}
		import3 := &audit.DataImport{
			EntityType:   "teacher",
			Filename:     "teachers.csv",
			TotalRows:    20,
			CreatedCount: 20,
			ImportedBy:   account2.ID,
			StartedAt:    now,
		}

		err := repo.Create(ctx, import1)
		require.NoError(t, err)
		err = repo.Create(ctx, import2)
		require.NoError(t, err)
		err = repo.Create(ctx, import3)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", import1.ID, import2.ID, import3.ID)

		imports, err := repo.FindByImportedBy(ctx, account1.ID, 10)
		require.NoError(t, err)
		assert.Len(t, imports, 2)

		for _, imp := range imports {
			assert.Equal(t, account1.ID, imp.ImportedBy)
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		now := time.Now()
		import1 := &audit.DataImport{
			EntityType: "student", Filename: "limit1.csv", TotalRows: 10, ImportedBy: account1.ID, StartedAt: now,
		}
		import2 := &audit.DataImport{
			EntityType: "student", Filename: "limit2.csv", TotalRows: 10, ImportedBy: account1.ID, StartedAt: now,
		}
		import3 := &audit.DataImport{
			EntityType: "student", Filename: "limit3.csv", TotalRows: 10, ImportedBy: account1.ID, StartedAt: now,
		}

		err := repo.Create(ctx, import1)
		require.NoError(t, err)
		err = repo.Create(ctx, import2)
		require.NoError(t, err)
		err = repo.Create(ctx, import3)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", import1.ID, import2.ID, import3.ID)

		imports, err := repo.FindByImportedBy(ctx, account1.ID, 2)
		require.NoError(t, err)
		assert.Len(t, imports, 2)
	})
}

func TestDataImportRepository_FindByEntityType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataImport
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "entity_type@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds imports by entity type", func(t *testing.T) {
		now := time.Now()
		studentImport := &audit.DataImport{
			EntityType:   "student",
			Filename:     "students.csv",
			TotalRows:    100,
			CreatedCount: 100,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}
		teacherImport := &audit.DataImport{
			EntityType:   "teacher",
			Filename:     "teachers.csv",
			TotalRows:    20,
			CreatedCount: 20,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}

		err := repo.Create(ctx, studentImport)
		require.NoError(t, err)
		err = repo.Create(ctx, teacherImport)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", studentImport.ID, teacherImport.ID)

		imports, err := repo.FindByEntityType(ctx, "student", 10)
		require.NoError(t, err)

		for _, imp := range imports {
			assert.Equal(t, "student", imp.EntityType)
		}

		var found bool
		for _, imp := range imports {
			if imp.ID == studentImport.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestDataImportRepository_FindRecent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataImport
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "recent@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds recent imports", func(t *testing.T) {
		now := time.Now()
		import1 := &audit.DataImport{
			EntityType:   "student",
			Filename:     "recent1.csv",
			TotalRows:    50,
			CreatedCount: 50,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}
		import2 := &audit.DataImport{
			EntityType:   "teacher",
			Filename:     "recent2.csv",
			TotalRows:    30,
			CreatedCount: 30,
			ImportedBy:   account.ID,
			StartedAt:    now.Add(time.Second), // Slightly later
		}

		err := repo.Create(ctx, import1)
		require.NoError(t, err)
		err = repo.Create(ctx, import2)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", import1.ID, import2.ID)

		imports, err := repo.FindRecent(ctx, 5)
		require.NoError(t, err)
		assert.NotEmpty(t, imports)

		// Verify both imports are in results
		var foundIDs []int64
		for _, imp := range imports {
			foundIDs = append(foundIDs, imp.ID)
		}
		assert.Contains(t, foundIDs, import1.ID)
		assert.Contains(t, foundIDs, import2.ID)
	})
}

func TestDataImportRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataImport
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "list_import@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("lists all imports", func(t *testing.T) {
		now := time.Now()
		dataImport := &audit.DataImport{
			EntityType:   "student",
			Filename:     "list_test.csv",
			TotalRows:    25,
			CreatedCount: 25,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}

		err := repo.Create(ctx, dataImport)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", dataImport.ID)

		imports, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, imports)
	})

	t.Run("lists with entity type filter", func(t *testing.T) {
		now := time.Now()
		dataImport := &audit.DataImport{
			EntityType:   "room",
			Filename:     "rooms_filter.csv",
			TotalRows:    10,
			CreatedCount: 10,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}

		err := repo.Create(ctx, dataImport)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", dataImport.ID)

		filters := map[string]interface{}{
			"entity_type": "room",
		}
		imports, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, imp := range imports {
			assert.Equal(t, "room", imp.EntityType)
		}
	})

	t.Run("lists with dry run filter", func(t *testing.T) {
		now := time.Now()
		dryRunImport := &audit.DataImport{
			EntityType:   "student",
			Filename:     "dry_run.csv",
			TotalRows:    50,
			SkippedCount: 50,
			DryRun:       true,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}

		err := repo.Create(ctx, dryRunImport)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", dryRunImport.ID)

		filters := map[string]interface{}{
			"dry_run": true,
		}
		imports, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, imp := range imports {
			assert.True(t, imp.DryRun)
		}
	})

	t.Run("lists with imported by filter", func(t *testing.T) {
		now := time.Now()
		dataImport := &audit.DataImport{
			EntityType:   "student",
			Filename:     "by_account.csv",
			TotalRows:    30,
			CreatedCount: 30,
			ImportedBy:   account.ID,
			StartedAt:    now,
		}

		err := repo.Create(ctx, dataImport)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_imports", dataImport.ID)

		filters := map[string]interface{}{
			"imported_by": account.ID,
		}
		imports, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, imp := range imports {
			assert.Equal(t, account.ID, imp.ImportedBy)
		}
	})
}
