package feedback_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupEntryRepo(t *testing.T, db *bun.DB) feedback.EntryRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.FeedbackEntry
}

// cleanupEntryRecords removes feedback entries directly
func cleanupEntryRecords(t *testing.T, db *bun.DB, entryIDs ...int64) {
	t.Helper()
	if len(entryIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("feedback.entries").
		Where("id IN (?)", bun.In(entryIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup feedback entries: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestEntryRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	// Create a test student for FK
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("creates entry with valid data", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		assert.NotZero(t, entry.ID)

		cleanupEntryRecords(t, db, entry.ID)
	})

	t.Run("creates mensa feedback entry", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)
		entry := &feedback.Entry{
			Value:           feedback.ValueNegative,
			Day:             now,
			Time:            time.Now(),
			StudentID:       student.ID,
			IsMensaFeedback: true,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		assert.NotZero(t, entry.ID)
		assert.True(t, entry.IsMensaFeedback)

		cleanupEntryRecords(t, db, entry.ID)
	})

	t.Run("create with nil entry should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid value should fail", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)
		entry := &feedback.Entry{
			Value:     "invalid_value",
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		assert.Error(t, err)
	})
}

func TestEntryRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Find", "Student", "2a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("finds existing entry", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)
		entry := &feedback.Entry{
			Value:     feedback.ValueNeutral,
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry.ID)

		found, err := repo.FindByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, entry.ID, found.ID)
		assert.Equal(t, feedback.ValueNeutral, found.Value)
	})

	t.Run("returns error for non-existent entry", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestEntryRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Update", "Student", "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("updates entry", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry.ID)

		entry.Value = feedback.ValueNegative
		err = repo.Update(ctx, entry)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, feedback.ValueNegative, found.Value)
	})
}

func TestEntryRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Delete", "Student", "4a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("deletes existing entry", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		err = repo.Delete(ctx, entry.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, entry.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestEntryRepository_FindByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "5a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "5b")
	defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

	t.Run("finds entries by student ID", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)
		entry1 := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       now,
			Time:      time.Now(),
			StudentID: student1.ID,
		}
		entry2 := &feedback.Entry{
			Value:     feedback.ValueNeutral,
			Day:       now,
			Time:      time.Now(),
			StudentID: student1.ID,
		}
		entry3 := &feedback.Entry{
			Value:     feedback.ValueNegative,
			Day:       now,
			Time:      time.Now(),
			StudentID: student2.ID,
		}

		err := repo.Create(ctx, entry1)
		require.NoError(t, err)
		err = repo.Create(ctx, entry2)
		require.NoError(t, err)
		err = repo.Create(ctx, entry3)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry1.ID, entry2.ID, entry3.ID)

		entries, err := repo.FindByStudentID(ctx, student1.ID)
		require.NoError(t, err)
		assert.Len(t, entries, 2)

		for _, e := range entries {
			assert.Equal(t, student1.ID, e.StudentID)
		}
	})
}

func TestEntryRepository_FindByDay(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Day", "Student", "6a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("finds entries by day", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		yesterday := today.AddDate(0, 0, -1)

		entry1 := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		entry2 := &feedback.Entry{
			Value:     feedback.ValueNeutral,
			Day:       yesterday,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry1)
		require.NoError(t, err)
		err = repo.Create(ctx, entry2)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry1.ID, entry2.ID)

		entries, err := repo.FindByDay(ctx, today)
		require.NoError(t, err)

		var found bool
		for _, e := range entries {
			if e.ID == entry1.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestEntryRepository_FindByDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Range", "Student", "7a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("finds entries in date range", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		yesterday := today.AddDate(0, 0, -1)
		weekAgo := today.AddDate(0, 0, -7)

		entry1 := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		entry2 := &feedback.Entry{
			Value:     feedback.ValueNeutral,
			Day:       yesterday,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry1)
		require.NoError(t, err)
		err = repo.Create(ctx, entry2)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry1.ID, entry2.ID)

		entries, err := repo.FindByDateRange(ctx, weekAgo, today)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2)
	})
}

func TestEntryRepository_FindMensaFeedback(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Mensa", "Student", "8a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("finds mensa feedback entries", func(t *testing.T) {
		now := time.Now().Truncate(24 * time.Hour)
		mensaEntry := &feedback.Entry{
			Value:           feedback.ValuePositive,
			Day:             now,
			Time:            time.Now(),
			StudentID:       student.ID,
			IsMensaFeedback: true,
		}
		regularEntry := &feedback.Entry{
			Value:           feedback.ValueNeutral,
			Day:             now,
			Time:            time.Now(),
			StudentID:       student.ID,
			IsMensaFeedback: false,
		}

		err := repo.Create(ctx, mensaEntry)
		require.NoError(t, err)
		err = repo.Create(ctx, regularEntry)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, mensaEntry.ID, regularEntry.ID)

		entries, err := repo.FindMensaFeedback(ctx, true)
		require.NoError(t, err)

		for _, e := range entries {
			assert.True(t, e.IsMensaFeedback)
		}

		var found bool
		for _, e := range entries {
			if e.ID == mensaEntry.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestEntryRepository_FindByStudentAndDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "StudentRange", "Test", "9a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("finds student entries in date range", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		yesterday := today.AddDate(0, 0, -1)
		weekAgo := today.AddDate(0, 0, -7)

		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       yesterday,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry.ID)

		entries, err := repo.FindByStudentAndDateRange(ctx, student.ID, weekAgo, today)
		require.NoError(t, err)

		var found bool
		for _, e := range entries {
			if e.ID == entry.ID {
				found = true
				assert.Equal(t, student.ID, e.StudentID)
				break
			}
		}
		assert.True(t, found)
	})
}

// ============================================================================
// Count Tests
// ============================================================================

func TestEntryRepository_CountByDay(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Count", "Day", "10a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("counts entries by day", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)

		entry1 := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		entry2 := &feedback.Entry{
			Value:     feedback.ValueNeutral,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry1)
		require.NoError(t, err)
		err = repo.Create(ctx, entry2)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry1.ID, entry2.ID)

		count, err := repo.CountByDay(ctx, today)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 2)
	})
}

func TestEntryRepository_CountByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Count", "Student", "11a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("counts entries by student", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)

		entry1 := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		entry2 := &feedback.Entry{
			Value:     feedback.ValueNegative,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry1)
		require.NoError(t, err)
		err = repo.Create(ctx, entry2)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry1.ID, entry2.ID)

		count, err := repo.CountByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 2)
	})
}

func TestEntryRepository_CountMensaFeedback(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Count", "Mensa", "12a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("counts mensa feedback entries", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)

		mensaEntry := &feedback.Entry{
			Value:           feedback.ValuePositive,
			Day:             today,
			Time:            time.Now(),
			StudentID:       student.ID,
			IsMensaFeedback: true,
		}

		err := repo.Create(ctx, mensaEntry)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, mensaEntry.ID)

		count, err := repo.CountMensaFeedback(ctx, true)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)
	})
}

func TestEntryRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupEntryRepo(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "List", "Student", "13a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("lists all entries", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry.ID)

		entries, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, entries)
	})

	t.Run("lists with filters", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		entry := &feedback.Entry{
			Value:           feedback.ValueNeutral,
			Day:             today,
			Time:            time.Now(),
			StudentID:       student.ID,
			IsMensaFeedback: true,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		defer cleanupEntryRecords(t, db, entry.ID)

		filters := map[string]interface{}{
			"is_mensa_feedback": true,
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, e := range entries {
			assert.True(t, e.IsMensaFeedback)
		}
	})
}
