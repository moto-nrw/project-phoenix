package active_test

import (
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("updates duration and ended_at of break", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

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
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

		// Create an active break (no ended_at)
		startedAt := time.Now().Add(-15 * time.Minute)
		brk := &active.WorkSessionBreak{
			SessionID:       session.ID,
			StartedAt:       startedAt,
			DurationMinutes: 0,
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("creates break with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now(),
		}

		err = repo.Create(ctx, brk)
		require.NoError(t, err)
		assert.NotZero(t, brk.ID)

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
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

		brk := &active.WorkSessionBreak{
			SessionID:       session.ID,
			StartedAt:       time.Now(),
			DurationMinutes: -10, // Invalid
		}
		err = repo.Create(ctx, brk)
		assert.Error(t, err)
	})

	t.Run("create with zero started_at should fail validation", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

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
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("returns all breaks for session", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

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

		breaks, err := repo.GetBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(breaks), 2)
	})
}

func TestWorkSessionBreakRepository_GetActiveBySessionID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("returns active break without ended_at", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

		// Create an active break
		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now(),
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)

		active, err := repo.GetActiveBySessionID(ctx, session.ID)
		require.NoError(t, err)
		require.NotNil(t, active)
		assert.Equal(t, brk.ID, active.ID)
	})

	t.Run("returns nil when no active break", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

		active, err := repo.GetActiveBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.Nil(t, active)
	})
}

func TestWorkSessionBreakRepository_EndBreak(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("ends active break", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

		// Create an active break
		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now().Add(-30 * time.Minute),
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)

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

func TestWorkSessionBreakRepository_EndBreakRejectsStaleState(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Break", "Guard")
	session := &active.WorkSession{
		StaffID:     staff.ID,
		Date:        timezone.TodayDate(),
		Status:      active.WorkSessionStatusPresent,
		CheckInTime: time.Now(),
		CreatedBy:   staff.ID,
	}
	require.NoError(t, repos.WorkSession.Create(ctx, session))
	brk := &active.WorkSessionBreak{
		SessionID: session.ID,
		StartedAt: time.Now().Add(-30 * time.Minute),
	}
	require.NoError(t, repos.WorkSessionBreak.Create(ctx, brk))

	endedAt := time.Now()
	require.NoError(t, repos.WorkSessionBreak.EndBreak(ctx, brk.ID, endedAt, 30))
	require.Error(t, repos.WorkSessionBreak.EndBreak(ctx, brk.ID, endedAt.Add(time.Minute), 31))

	unchanged, err := repos.WorkSessionBreak.FindByID(ctx, brk.ID)
	require.NoError(t, err)
	require.NotNil(t, unchanged.EndedAt)
	assert.WithinDuration(t, endedAt, *unchanged.EndedAt, time.Second)
	assert.Equal(t, 30, unchanged.DurationMinutes)
}

func TestWorkSessionBreakRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSessionBreak
	sessionRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("lists all breaks", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

		// Create breaks
		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now(),
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)

		breaks, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, breaks)
	})

	t.Run("lists with query options", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		// Create a work session
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)

		// Create breaks
		brk := &active.WorkSessionBreak{
			SessionID: session.ID,
			StartedAt: time.Now(),
		}
		err = repo.Create(ctx, brk)
		require.NoError(t, err)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)

		breaks, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(breaks), 10)
	})
}
