package feedback_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupFeedbackService creates a feedback service with real database connection.
func setupFeedbackService(t *testing.T, db *bun.DB) feedbackSvc.Service {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	return feedbackSvc.NewService(repoFactory.FeedbackEntry)
}

// ============================================================================
// Test Fixtures - Feedback Domain
// ============================================================================

// createTestFeedbackEntry creates a test feedback entry in the database
func createTestFeedbackEntry(t *testing.T, db *bun.DB, studentID int64, value string, day timezone.Date) *feedback.Entry {
	t.Helper()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	entry := &feedback.Entry{
		Value:           value,
		Day:             day,
		Time:            time.Now(),
		StudentID:       studentID,
		IsMensaFeedback: false,
	}
	entry.SetTenantID(testpkg.Tenant(t))

	err := db.NewInsert().
		Model(entry).
		ModelTableExpr(`feedback.entries`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test feedback entry")

	return entry
}

// ============================================================================
// Core Operations Tests
// ============================================================================

func TestFeedbackService_CreateEntry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student
	student := testpkg.CreateTestStudent(t, db, "Feedback", "TestStudent", "1a")

	t.Run("creates valid feedback entry", func(t *testing.T) {
		// ARRANGE
		entry := &feedback.Entry{
			Value:           feedback.ValuePositive,
			Day:             timezone.TodayDate(),
			Time:            time.Now(),
			StudentID:       student.ID,
			IsMensaFeedback: false,
		}

		// ACT
		err := service.CreateEntry(ctx, entry)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, entry.ID)

		// Cleanup
	})

	t.Run("rejects nil entry", func(t *testing.T) {
		// ACT
		err := service.CreateEntry(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects entry with invalid value", func(t *testing.T) {
		// ARRANGE
		entry := &feedback.Entry{
			Value:     "invalid-value",
			Day:       timezone.TodayDate(),
			Time:      time.Now(),
			StudentID: student.ID,
		}

		// ACT
		err := service.CreateEntry(ctx, entry)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects entry without student ID", func(t *testing.T) {
		// ARRANGE
		entry := &feedback.Entry{
			Value: feedback.ValuePositive,
			Day:   timezone.TodayDate(),
			Time:  time.Now(),
		}

		// ACT
		err := service.CreateEntry(ctx, entry)

		// ASSERT
		require.Error(t, err)
	})
}

func TestFeedbackService_GetEntryByID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student and entry
	student := testpkg.CreateTestStudent(t, db, "Feedback", "GetStudent", "2a")

	entry := createTestFeedbackEntry(t, db, student.ID, feedback.ValueNeutral, timezone.TodayDate())

	t.Run("retrieves entry by ID", func(t *testing.T) {
		// ACT
		result, err := service.GetEntryByID(ctx, entry.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, entry.ID, result.ID)
		assert.Equal(t, feedback.ValueNeutral, result.Value)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		// ACT
		_, err := service.GetEntryByID(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		_, err := service.GetEntryByID(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})
}

func TestFeedbackService_DeleteEntry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student
	student := testpkg.CreateTestStudent(t, db, "Feedback", "DeleteStudent", "4a")

	t.Run("deletes entry successfully", func(t *testing.T) {
		// ARRANGE
		entry := createTestFeedbackEntry(t, db, student.ID, feedback.ValuePositive, timezone.TodayDate())

		// ACT
		err := service.DeleteEntry(ctx, entry.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetEntryByID(ctx, entry.ID)
		require.Error(t, err)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		err := service.DeleteEntry(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent entry", func(t *testing.T) {
		// ACT
		err := service.DeleteEntry(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestFeedbackService_ListEntries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student and entries
	student := testpkg.CreateTestStudent(t, db, "Feedback", "ListStudent", "5a")

	createTestFeedbackEntry(t, db, student.ID, feedback.ValuePositive, timezone.TodayDate())
	createTestFeedbackEntry(t, db, student.ID, feedback.ValueNeutral, timezone.TodayDate())

	t.Run("lists all entries", func(t *testing.T) {
		// ACT
		entries, err := service.ListEntries(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2)
	})

	t.Run("lists with filters", func(t *testing.T) {
		// ARRANGE
		filters := map[string]interface{}{
			"student_id": student.ID,
		}

		// ACT
		entries, err := service.ListEntries(ctx, filters)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2)
	})
}

// ============================================================================
// Query Operations Tests
// ============================================================================

func TestFeedbackService_GetEntriesByStudent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student and entries
	student := testpkg.CreateTestStudent(t, db, "Feedback", "StudentQuery", "6a")

	createTestFeedbackEntry(t, db, student.ID, feedback.ValuePositive, timezone.TodayDate())
	createTestFeedbackEntry(t, db, student.ID, feedback.ValueNegative, timezone.TodayDate())

	t.Run("retrieves entries for student", func(t *testing.T) {
		// ACT
		entries, err := service.GetEntriesByStudent(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2)
	})

	t.Run("returns error for invalid student ID", func(t *testing.T) {
		// ACT
		_, err := service.GetEntriesByStudent(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})
}

func TestFeedbackService_GetEntriesByDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student and entries
	student := testpkg.CreateTestStudent(t, db, "Feedback", "DayQuery", "7a")

	today := timezone.TodayDate()
	createTestFeedbackEntry(t, db, student.ID, feedback.ValuePositive, today)

	t.Run("retrieves entries for day", func(t *testing.T) {
		// ACT
		entries, err := service.GetEntriesByDay(ctx, today)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 1)
	})

	t.Run("returns error for zero time", func(t *testing.T) {
		// ACT
		_, err := service.GetEntriesByDay(ctx, timezone.Date{})

		// ASSERT
		require.Error(t, err)
	})
}

func TestFeedbackService_GetEntriesByDateRange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student and entries
	student := testpkg.CreateTestStudent(t, db, "Feedback", "RangeQuery", "8a")

	today := timezone.TodayDate()
	createTestFeedbackEntry(t, db, student.ID, feedback.ValueNeutral, today)

	t.Run("retrieves entries for date range", func(t *testing.T) {
		// ARRANGE
		startDate := today.AddDays(-1)
		endDate := today.AddDays(1)

		// ACT
		entries, err := service.GetEntriesByDateRange(ctx, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 1)
	})

	t.Run("returns error for zero start date", func(t *testing.T) {
		// ACT
		_, err := service.GetEntriesByDateRange(ctx, timezone.Date{}, timezone.TodayDate())

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for zero end date", func(t *testing.T) {
		// ACT
		_, err := service.GetEntriesByDateRange(ctx, timezone.TodayDate(), timezone.Date{})

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for inverted date range", func(t *testing.T) {
		// ACT
		_, err := service.GetEntriesByDateRange(ctx, timezone.TodayDate().AddDays(1), timezone.TodayDate())

		// ASSERT
		require.Error(t, err)
	})
}

func TestFeedbackService_GetMensaFeedback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("retrieves mensa feedback", func(t *testing.T) {
		// ACT
		_, err := service.GetMensaFeedback(ctx, true)

		// ASSERT
		require.NoError(t, err)
		// May or may not have entries, just verify no error
	})

	t.Run("retrieves non-mensa feedback", func(t *testing.T) {
		// ACT
		_, err := service.GetMensaFeedback(ctx, false)

		// ASSERT
		require.NoError(t, err)
	})
}

func TestFeedbackService_GetEntriesByStudentAndDateRange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student and entries
	student := testpkg.CreateTestStudent(t, db, "Feedback", "StudentRange", "9a")

	today := timezone.TodayDate()
	createTestFeedbackEntry(t, db, student.ID, feedback.ValuePositive, today)

	t.Run("retrieves entries for student and date range", func(t *testing.T) {
		// ARRANGE
		startDate := today.AddDays(-1)
		endDate := today.AddDays(1)

		// ACT
		entries, err := service.GetEntriesByStudentAndDateRange(ctx, student.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 1)
	})

	t.Run("returns error for invalid student ID", func(t *testing.T) {
		// ACT
		_, err := service.GetEntriesByStudentAndDateRange(ctx, 0, timezone.TodayDate(), timezone.TodayDate().AddDays(1))

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for zero dates", func(t *testing.T) {
		// ACT
		_, err := service.GetEntriesByStudentAndDateRange(ctx, student.ID, timezone.Date{}, timezone.TodayDate())

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for inverted date range", func(t *testing.T) {
		// ACT
		_, err := service.GetEntriesByStudentAndDateRange(ctx, student.ID, timezone.TodayDate().AddDays(1), timezone.TodayDate())

		// ASSERT
		require.Error(t, err)
	})
}

// ============================================================================
// Count Operations Tests
// ============================================================================

// ============================================================================
// Batch Operations Tests
// ============================================================================

func TestFeedbackService_CreateEntries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student
	student := testpkg.CreateTestStudent(t, db, "Feedback", "BatchStudent", "12a")

	t.Run("creates multiple entries", func(t *testing.T) {
		// ARRANGE
		entries := []*feedback.Entry{
			{Value: feedback.ValuePositive, Day: timezone.TodayDate(), Time: time.Now(), StudentID: student.ID},
			{Value: feedback.ValueNeutral, Day: timezone.TodayDate(), Time: time.Now(), StudentID: student.ID},
		}

		// ACT
		errors, err := service.CreateEntries(ctx, entries)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, errors)
	})

	t.Run("returns empty for empty slice", func(t *testing.T) {
		// ACT
		errors, err := service.CreateEntries(ctx, []*feedback.Entry{})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, errors)
	})

	t.Run("collects errors for invalid entries", func(t *testing.T) {
		// ARRANGE
		entries := []*feedback.Entry{
			{Value: feedback.ValuePositive, Day: timezone.TodayDate(), Time: time.Now(), StudentID: student.ID},
			{Value: "invalid", Day: timezone.TodayDate(), Time: time.Now(), StudentID: student.ID}, // Invalid value
		}

		// ACT
		errors, err := service.CreateEntries(ctx, entries)

		// ASSERT
		// Should have error for the invalid entry
		require.Error(t, err)
		assert.NotEmpty(t, errors)
	})
}

// ============================================================================
// Cleanup Operations Tests
// ============================================================================

func TestFeedbackService_DeleteEntriesOlderThan(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFeedbackService(t, db)
	ctx := testpkg.Ctx(t)

	// Create a test student
	student := testpkg.CreateTestStudent(t, db, "Feedback", "CleanupStudent", "13a")

	t.Run("deletes entries older than specified days", func(t *testing.T) {
		// ARRANGE: Create an old entry (100 days ago) and a recent entry (today)
		oldDay := timezone.TodayDate().AddDays(-100)
		recentDay := timezone.TodayDate()

		createTestFeedbackEntry(t, db, student.ID, feedback.ValuePositive, oldDay)
		recentEntry := createTestFeedbackEntry(t, db, student.ID, feedback.ValueNeutral, recentDay)

		// Count before
		entriesBefore, err := service.GetEntriesByStudent(ctx, student.ID)
		require.NoError(t, err)
		countBefore := len(entriesBefore)

		// ACT: Delete entries older than 30 days
		deleted, err := service.DeleteEntriesOlderThan(ctx, 30)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, 1, "should delete at least the old entry")

		// Verify count decreased
		entriesAfter, err := service.GetEntriesByStudent(ctx, student.ID)
		require.NoError(t, err)
		assert.Less(t, len(entriesAfter), countBefore, "should have fewer entries after cleanup")

		// Verify recent entry still exists
		recent, err := service.GetEntryByID(ctx, recentEntry.ID)
		require.NoError(t, err, "recent entry should still exist")
		assert.Equal(t, recentEntry.ID, recent.ID)
	})

	t.Run("returns zero when no old entries exist", func(t *testing.T) {
		// ARRANGE: Create only a recent entry
		createTestFeedbackEntry(t, db, student.ID, feedback.ValueNegative, timezone.TodayDate())

		// ACT: Delete entries older than 365 days — none should qualify
		deleted, err := service.DeleteEntriesOlderThan(ctx, 365)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 0, deleted)
	})

	t.Run("rejects zero days", func(t *testing.T) {
		// ACT
		_, err := service.DeleteEntriesOlderThan(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects negative days", func(t *testing.T) {
		// ACT
		_, err := service.DeleteEntriesOlderThan(ctx, -1)

		// ASSERT
		require.Error(t, err)
	})
}

// ============================================================================
// Error Type Tests
// ============================================================================

func TestFeedbackErrors(t *testing.T) {
	t.Parallel()
	t.Run("InvalidEntryDataError contains error details", func(t *testing.T) {
		err := &feedbackSvc.InvalidEntryDataError{Err: feedbackSvc.ErrInvalidParameters}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "invalid")
	})

	t.Run("EntryNotFoundError contains entry ID", func(t *testing.T) {
		err := &feedbackSvc.EntryNotFoundError{EntryID: 123}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "123")
	})

	t.Run("InvalidDateRangeError contains dates", func(t *testing.T) {
		start := timezone.TodayDate().AddDays(1)
		end := timezone.TodayDate()
		err := &feedbackSvc.InvalidDateRangeError{StartDate: start, EndDate: end}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "date range")
	})

	t.Run("BatchOperationError contains error count", func(t *testing.T) {
		err := &feedbackSvc.BatchOperationError{
			Errors: []error{feedbackSvc.ErrInvalidParameters, feedbackSvc.ErrInvalidParameters},
		}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "2")
	})
}
