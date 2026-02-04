package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// UpdateDuration Tests (uncovered method)
// ============================================================================

func TestWorkSessionBreakRepository_UpdateDuration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("updates duration and ended_at of break", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		// Create a break
		startedAt := time.Now().Add(-30 * time.Minute)
		endedAt := time.Now()
		brk := &active.WorkSessionBreak{
			SessionID:       session.ID,
			StartedAt:       startedAt,
			EndedAt:         &endedAt,
			DurationMinutes: 30,
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk.ID)

		// Update the duration
		newEndedAt := time.Now().Add(5 * time.Minute)
		err = repo.UpdateDuration(ctx, brk.ID, 45, newEndedAt)
		require.NoError(t, err)

		// Verify the update
		updated, err := repo.FindByID(ctx, brk.ID)
		require.NoError(t, err)
		assert.Equal(t, 45, updated.DurationMinutes)
		assert.NotNil(t, updated.EndedAt)
		assert.WithinDuration(t, newEndedAt, *updated.EndedAt, time.Second)
	})

	t.Run("updates duration for break without ended_at", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		// Create an active break (no ended_at)
		startedAt := time.Now().Add(-15 * time.Minute)
		brk := &active.WorkSessionBreak{
			SessionID:       session.ID,
			StartedAt:       startedAt,
			DurationMinutes: 0,
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk.ID)

		// Update the duration and set ended_at
		endedAt := time.Now()
		err = repo.UpdateDuration(ctx, brk.ID, 15, endedAt)
		require.NoError(t, err)

		// Verify the update
		updated, err := repo.FindByID(ctx, brk.ID)
		require.NoError(t, err)
		assert.Equal(t, 15, updated.DurationMinutes)
		assert.NotNil(t, updated.EndedAt)
	})

	t.Run("handles non-existent break gracefully", func(t *testing.T) {
		err := repo.UpdateDuration(ctx, 999999, 30, time.Now())
		require.NoError(t, err) // Should not error, just won't update anything
	})
}

// ============================================================================
// Additional Coverage Tests
// ============================================================================

func TestWorkSessionBreakRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("creates break with valid data", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now(),
		}

		err = repo.Create(ctx, brk)
		require.NoError(t, err)
		assert.NotZero(t, brk.ID)

		testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk.ID)
	})

	t.Run("create with nil break should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid break should fail validation", func(t *testing.T) {
		// Break with missing session ID
		brk := &active.WorkSessionBreak{
			SessionID: 0, // Invalid
			StartedAt: time.Now(),
		}
		err := repo.Create(ctx, brk)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session ID is required")
	})

	t.Run("create with negative duration should fail", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		brk := &active.WorkSessionBreak{
			SessionID:       session.ID,
			StartedAt:       time.Now(),
			DurationMinutes: -10, // Invalid
		}
		err = repo.Create(ctx, brk)
		assert.Error(t, err)
	})

	t.Run("create with zero started_at should fail validation", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		brk := &active.WorkSessionBreak{
			SessionID:       session.ID,
			StartedAt:       time.Time{}, // Zero value - invalid
			DurationMinutes: 0,
		}
		err = repo.Create(ctx, brk)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "started_at is required")
	})

	t.Run("create with started_at after ended_at should fail validation", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		startedAt := time.Now()
		endedAt := startedAt.Add(-30 * time.Minute) // Ended before started - invalid
		brk := &active.WorkSessionBreak{
			SessionID:       session.ID,
			StartedAt:       startedAt,
			EndedAt:         &endedAt,
			DurationMinutes: 30,
		}
		err = repo.Create(ctx, brk)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "started_at must be before ended_at")
	})
}

func TestWorkSessionBreakRepository_GetBySessionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("returns all breaks for session", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		// Create multiple breaks
		brk1 := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now().Add(-2 * time.Hour),
		}
		brk2 := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now().Add(-1 * time.Hour),
		}

		err = repo.Create(ctx, brk1)
		require.NoError(t, err)
		err = repo.Create(ctx, brk2)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk1.ID)
			testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk2.ID)
		}()

		breaks, err := repo.GetBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(breaks), 2)
	})
}

func TestWorkSessionBreakRepository_GetActiveBySessionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("returns active break without ended_at", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		// Create an active break
		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now(),
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk.ID)

		active, err := repo.GetActiveBySessionID(ctx, session.ID)
		require.NoError(t, err)
		require.NotNil(t, active)
		assert.Equal(t, brk.ID, active.ID)
	})

	t.Run("returns nil when no active break", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		active, err := repo.GetActiveBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.Nil(t, active)
	})
}

func TestWorkSessionBreakRepository_EndBreak(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("ends active break", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		// Create an active break
		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now().Add(-30 * time.Minute),
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk.ID)

		// End the break
		endedAt := time.Now()
		err = repo.EndBreak(ctx, brk.ID, endedAt, 30)
		require.NoError(t, err)

		// Verify it's ended
		ended, err := repo.FindByID(ctx, brk.ID)
		require.NoError(t, err)
		assert.NotNil(t, ended.EndedAt)
		assert.Equal(t, 30, ended.DurationMinutes)
	})
}

func TestWorkSessionBreakRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("lists all breaks", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		// Create breaks
		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now(),
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk.ID)

		breaks, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, breaks)
	})

	t.Run("lists with query options", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		// Create breaks
		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now(),
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_session_breaks", brk.ID)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)

		breaks, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(breaks), 10)
	})
}
