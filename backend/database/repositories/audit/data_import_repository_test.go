package audit_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestDataImportRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).DataImport
	ctx := testpkg.Ctx(t)

	// Create a test account for the imported_by FK
	account := testpkg.CreateTestAccount(t, db, "import_test@example.com")

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

	})
}

func TestDataImportRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).DataImport
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "find_import@example.com")

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

func TestDataImportRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).DataImport
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "list_import@example.com")

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
