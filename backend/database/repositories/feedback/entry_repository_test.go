package feedback_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestEntryRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	// Create a test student for FK
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Student", "1a")

	t.Run("creates entry with valid data", func(t *testing.T) {
		now := timezone.TodayDate()
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)
		assert.NotZero(t, entry.ID)

	})

	t.Run("creates mensa feedback entry", func(t *testing.T) {
		now := timezone.TodayDate()
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

	})

	t.Run("create with nil entry should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid value should fail", func(t *testing.T) {
		now := timezone.TodayDate()
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Find", "Student", "2a")

	t.Run("finds existing entry", func(t *testing.T) {
		now := timezone.TodayDate()
		entry := &feedback.Entry{
			Value:     feedback.ValueNeutral,
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, entry.ID, found.ID)
		assert.Equal(t, feedback.ValueNeutral, found.Value)
	})

	t.Run("returns nil for non-existent entry", func(t *testing.T) {
		found, err := repo.FindByID(ctx, int64(999999))
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestEntryRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Update", "Student", "3a")

	t.Run("updates entry", func(t *testing.T) {
		now := timezone.TodayDate()
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		entry.Value = feedback.ValueNegative
		err = repo.Update(ctx, entry)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, feedback.ValueNegative, found.Value)
	})
}

func TestEntryRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Delete", "Student", "4a")

	t.Run("deletes existing entry", func(t *testing.T) {
		now := timezone.TodayDate()
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

		// After delete, FindByID should return nil for not found
		found, err := repo.FindByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestEntryRepository_FindByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "5a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "5b")

	t.Run("finds entries by student ID", func(t *testing.T) {
		now := timezone.TodayDate()
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

		entries, err := repo.FindByStudentID(ctx, student1.ID)
		require.NoError(t, err)
		assert.Len(t, entries, 2)

		for _, e := range entries {
			assert.Equal(t, student1.ID, e.StudentID)
		}
	})
}

func TestEntryRepository_FindByDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Day", "Student", "6a")

	t.Run("finds entries by day", func(t *testing.T) {
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Range", "Student", "7a")

	t.Run("finds entries in date range", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		yesterday := today.AddDays(-1)
		weekAgo := today.AddDays(-7)

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

		entries, err := repo.FindByDateRange(ctx, weekAgo, today)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2)
	})
}

func TestEntryRepository_FindMensaFeedback(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Mensa", "Student", "8a")

	t.Run("finds mensa feedback entries", func(t *testing.T) {
		now := timezone.TodayDate()
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "StudentRange", "Test", "9a")

	t.Run("finds student entries in date range", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		yesterday := today.AddDays(-1)
		weekAgo := today.AddDays(-7)

		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       yesterday,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)

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
// Cleanup Tests
// ============================================================================

func TestEntryRepository_DeleteOlderThan(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Cleanup", "Student", "14a")

	t.Run("deletes entries older than N days", func(t *testing.T) {
		// ARRANGE: old entry (100 days ago) + recent entry (today)
		oldDay := timezone.TodayDate().AddDays(-100)
		today := timezone.TodayDate()

		oldEntry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       oldDay,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		recentEntry := &feedback.Entry{
			Value:     feedback.ValueNeutral,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, oldEntry)
		require.NoError(t, err)
		err = repo.Create(ctx, recentEntry)
		require.NoError(t, err)

		// Count entries for this student before delete
		entriesBefore, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		countBefore := len(entriesBefore)

		// ACT: delete entries older than 30 days
		deleted, err := repo.DeleteOlderThan(ctx, 30)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, 1)

		// Count entries for this student after delete — should have fewer
		entriesAfter, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Less(t, len(entriesAfter), countBefore, "should have fewer entries after cleanup")
	})

	t.Run("returns zero when nothing to delete", func(t *testing.T) {
		// ARRANGE: only a recent entry
		recentEntry := &feedback.Entry{
			Value:     feedback.ValueNegative,
			Day:       timezone.TodayDate(),
			Time:      time.Now(),
			StudentID: student.ID,
		}
		err := repo.Create(ctx, recentEntry)
		require.NoError(t, err)

		// ACT
		deleted, err := repo.DeleteOlderThan(ctx, 365)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 0, deleted)
	})
}

// ============================================================================
// Count Tests
// ============================================================================

func TestEntryRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "List", "Student", "13a")

	t.Run("lists all entries", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		entries, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, entries)
	})

	t.Run("lists with is_mensa_feedback filter", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		entry := &feedback.Entry{
			Value:           feedback.ValueNeutral,
			Day:             today,
			Time:            time.Now(),
			StudentID:       student.ID,
			IsMensaFeedback: true,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"is_mensa_feedback": true,
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, e := range entries {
			assert.True(t, e.IsMensaFeedback)
		}
	})

	t.Run("lists with day_from filter", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		yesterday := today.AddDays(-1)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"day_from": yesterday,
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, e := range entries {
			assert.True(t, !e.Day.Before(yesterday))
		}
	})

	t.Run("lists with day_to filter", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"day_to": today.AddDays(1),
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, entries)
	})

	t.Run("lists with value_like filter", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"value_like": "pos",
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, e := range entries {
			assert.Contains(t, string(e.Value), "pos")
		}
	})

	t.Run("lists with student_id filter (default case)", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"student_id": student.ID,
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, e := range entries {
			assert.Equal(t, student.ID, e.StudentID)
		}
	})

	t.Run("lists with nil value in filters", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       today,
			Time:      time.Now(),
			StudentID: student.ID,
		}

		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"is_mensa_feedback": nil,
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, entries)
	})
}

func TestEntryRepository_Update_EdgeCases(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "UpdateEdge", "Student", "14a")

	t.Run("update with nil entry should fail", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("update with invalid value should fail", func(t *testing.T) {
		now := timezone.TodayDate()
		entry := &feedback.Entry{
			Value:     feedback.ValuePositive,
			Day:       now,
			Time:      time.Now(),
			StudentID: student.ID,
		}
		err := repo.Create(ctx, entry)
		require.NoError(t, err)

		entry.Value = "invalid_value"
		err = repo.Update(ctx, entry)
		assert.Error(t, err)
	})
}

// ============================================================================
// Filter Type Assertion Edge Cases
// ============================================================================

func TestEntryRepository_List_InvalidFilterTypes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).FeedbackEntry
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "FilterType", "Student", "15a")

	// Create entry for testing
	now := timezone.TodayDate()
	entry := &feedback.Entry{
		Value:     feedback.ValuePositive,
		Day:       now,
		Time:      time.Now(),
		StudentID: student.ID,
	}
	err := repo.Create(ctx, entry)
	require.NoError(t, err)

	t.Run("day_from with non-time value is ignored", func(t *testing.T) {
		filters := map[string]interface{}{
			"day_from": "not-a-time", // string instead of time.Time
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)
		// Should return results because filter is ignored
		assert.NotEmpty(t, entries)
	})

	t.Run("day_to with non-time value is ignored", func(t *testing.T) {
		filters := map[string]interface{}{
			"day_to": 12345, // int instead of time.Time
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, entries)
	})

	t.Run("value_like with non-string value is ignored", func(t *testing.T) {
		filters := map[string]interface{}{
			"value_like": 999, // int instead of string
		}
		entries, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, entries)
	})
}
