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

func TestDataDeletionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataDeletion
	ctx := context.Background()

	// Create a test student for FK reference
	student := testpkg.CreateTestStudent(t, db, "Deletion", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("creates data deletion with valid data", func(t *testing.T) {
		deletion := audit.NewDataDeletion(
			student.ID,
			audit.DeletionTypeVisitRetention,
			10,
			"system",
		)

		err := repo.Create(ctx, deletion)
		require.NoError(t, err)
		assert.NotZero(t, deletion.ID)

		testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion.ID)
	})

	t.Run("creates manual deletion record", func(t *testing.T) {
		deletion := &audit.DataDeletion{
			StudentID:      student.ID,
			DeletionType:   audit.DeletionTypeManual,
			RecordsDeleted: 5,
			DeletionReason: "Parent requested data deletion",
			DeletedBy:      "admin@example.com",
			DeletedAt:      time.Now(),
		}

		err := repo.Create(ctx, deletion)
		require.NoError(t, err)
		assert.NotZero(t, deletion.ID)

		testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion.ID)
	})

	t.Run("creates GDPR request deletion", func(t *testing.T) {
		deletion := audit.NewDataDeletion(
			student.ID,
			audit.DeletionTypeGDPRRequest,
			25,
			"admin@example.com",
		)
		deletion.DeletionReason = "GDPR Article 17 request"
		deletion.SetMetadata("request_id", "GDPR-2024-001")

		err := repo.Create(ctx, deletion)
		require.NoError(t, err)
		assert.NotZero(t, deletion.ID)

		testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion.ID)
	})

	t.Run("create with nil deletion should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestDataDeletionRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataDeletion
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Find", "Student", "2a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("finds existing deletion", func(t *testing.T) {
		deletion := audit.NewDataDeletion(
			student.ID,
			audit.DeletionTypeVisitRetention,
			15,
			"system",
		)
		err := repo.Create(ctx, deletion)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion.ID)

		found, err := repo.FindByID(ctx, deletion.ID)
		require.NoError(t, err)
		assert.Equal(t, deletion.ID, found.ID)
		assert.Equal(t, student.ID, found.StudentID)
	})

	t.Run("returns error for non-existent deletion", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestDataDeletionRepository_FindByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataDeletion
	ctx := context.Background()

	student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "3a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "3b")
	defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

	t.Run("finds deletions by student ID", func(t *testing.T) {
		deletion1 := audit.NewDataDeletion(student1.ID, audit.DeletionTypeVisitRetention, 5, "system")
		deletion2 := audit.NewDataDeletion(student1.ID, audit.DeletionTypeManual, 3, "admin")
		deletion3 := audit.NewDataDeletion(student2.ID, audit.DeletionTypeGDPRRequest, 10, "admin")

		err := repo.Create(ctx, deletion1)
		require.NoError(t, err)
		err = repo.Create(ctx, deletion2)
		require.NoError(t, err)
		err = repo.Create(ctx, deletion3)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion1.ID, deletion2.ID, deletion3.ID)

		deletions, err := repo.FindByStudentID(ctx, student1.ID)
		require.NoError(t, err)
		assert.Len(t, deletions, 2)

		for _, d := range deletions {
			assert.Equal(t, student1.ID, d.StudentID)
		}
	})
}

func TestDataDeletionRepository_FindByDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataDeletion
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Range", "Student", "4a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("finds deletions in date range", func(t *testing.T) {
		now := time.Now()
		weekAgo := now.AddDate(0, 0, -7)

		deletion := audit.NewDataDeletion(student.ID, audit.DeletionTypeVisitRetention, 5, "system")
		err := repo.Create(ctx, deletion)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion.ID)

		deletions, err := repo.FindByDateRange(ctx, weekAgo, now.Add(time.Hour))
		require.NoError(t, err)

		var found bool
		for _, d := range deletions {
			if d.ID == deletion.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestDataDeletionRepository_FindByType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataDeletion
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Type", "Student", "5a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("finds deletions by type", func(t *testing.T) {
		visitRetention := audit.NewDataDeletion(student.ID, audit.DeletionTypeVisitRetention, 5, "system")
		manual := audit.NewDataDeletion(student.ID, audit.DeletionTypeManual, 3, "admin")

		err := repo.Create(ctx, visitRetention)
		require.NoError(t, err)
		err = repo.Create(ctx, manual)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_deletions", visitRetention.ID, manual.ID)

		deletions, err := repo.FindByType(ctx, audit.DeletionTypeVisitRetention)
		require.NoError(t, err)

		for _, d := range deletions {
			assert.Equal(t, audit.DeletionTypeVisitRetention, d.DeletionType)
		}

		var found bool
		for _, d := range deletions {
			if d.ID == visitRetention.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestDataDeletionRepository_GetDeletionStats(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataDeletion
	ctx := context.Background()

	student1 := testpkg.CreateTestStudent(t, db, "Stats", "One", "6a")
	student2 := testpkg.CreateTestStudent(t, db, "Stats", "Two", "6b")
	defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

	t.Run("gets deletion statistics", func(t *testing.T) {
		weekAgo := time.Now().AddDate(0, 0, -7)

		deletion1 := audit.NewDataDeletion(student1.ID, audit.DeletionTypeVisitRetention, 10, "system")
		deletion2 := audit.NewDataDeletion(student1.ID, audit.DeletionTypeManual, 5, "admin")
		deletion3 := audit.NewDataDeletion(student2.ID, audit.DeletionTypeGDPRRequest, 20, "admin")

		err := repo.Create(ctx, deletion1)
		require.NoError(t, err)
		err = repo.Create(ctx, deletion2)
		require.NoError(t, err)
		err = repo.Create(ctx, deletion3)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion1.ID, deletion2.ID, deletion3.ID)

		stats, err := repo.GetDeletionStats(ctx, weekAgo)
		require.NoError(t, err)

		assert.NotNil(t, stats)
		assert.Contains(t, stats, "total_deletions")
		assert.Contains(t, stats, "total_records_deleted")
		assert.Contains(t, stats, "unique_students")
		assert.Contains(t, stats, "by_type")
	})
}

func TestDataDeletionRepository_CountByType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataDeletion
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Count", "Student", "7a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("counts deletions by type", func(t *testing.T) {
		weekAgo := time.Now().AddDate(0, 0, -7)

		deletion1 := audit.NewDataDeletion(student.ID, audit.DeletionTypeVisitRetention, 5, "system")
		deletion2 := audit.NewDataDeletion(student.ID, audit.DeletionTypeVisitRetention, 3, "system")

		err := repo.Create(ctx, deletion1)
		require.NoError(t, err)
		err = repo.Create(ctx, deletion2)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion1.ID, deletion2.ID)

		count, err := repo.CountByType(ctx, audit.DeletionTypeVisitRetention, weekAgo)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(2))
	})
}

func TestDataDeletionRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).DataDeletion
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "List", "Student", "8a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("lists all deletions", func(t *testing.T) {
		deletion := audit.NewDataDeletion(student.ID, audit.DeletionTypeVisitRetention, 5, "system")
		err := repo.Create(ctx, deletion)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion.ID)

		deletions, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, deletions)
	})

	t.Run("lists with filters", func(t *testing.T) {
		deletion := audit.NewDataDeletion(student.ID, audit.DeletionTypeManual, 8, "admin@test.com")
		err := repo.Create(ctx, deletion)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.data_deletions", deletion.ID)

		filters := map[string]interface{}{
			"deletion_type": audit.DeletionTypeManual,
		}
		deletions, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, d := range deletions {
			assert.Equal(t, audit.DeletionTypeManual, d.DeletionType)
		}
	})
}
