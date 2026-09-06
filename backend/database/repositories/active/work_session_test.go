package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestWorkSessionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("creates work session with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, session)
		require.NoError(t, err)
		assert.NotZero(t, session.ID)

	})

	t.Run("creates work session with home office status", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusHomeOffice,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, session)
		require.NoError(t, err)
		assert.NotZero(t, session.ID)
		assert.Equal(t, active.WorkSessionStatusHomeOffice, session.Status)

	})

	t.Run("create with nil session should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid status should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      "invalid_status",
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, session)
		assert.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestWorkSessionRepository_GetByStaffAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("finds existing session by staff and date", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("staff_id", staff.ID).Equal("date", today)
		found, err := repo.List(ctx, options)
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, session.ID, found[0].ID)
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("staff_id", staff.ID).Equal("date", today)
		found, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

func TestWorkSessionRepository_GetCurrentByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("finds active session for staff today", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		found, err := repo.GetCurrentByStaffID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, found.ID)
		assert.Nil(t, found.CheckOutTime)
	})

	t.Run("returns error when no active session exists", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		_, err := repo.GetCurrentByStaffID(ctx, staff.ID)
		require.Error(t, err)
	})

	t.Run("ignores checked-out sessions", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		checkOutTime := time.Now()
		session := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         today,
			Status:       active.WorkSessionStatusPresent,
			CheckInTime:  time.Now().Add(-2 * time.Hour),
			CheckOutTime: &checkOutTime,
			CreatedBy:    staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		_, err = repo.GetCurrentByStaffID(ctx, staff.ID)
		require.Error(t, err) // Should not find checked-out session
	})
}

func TestWorkSessionRepository_GetHistoryByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("returns sessions in date range", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.NewDate(2026, 8, 24)
		yesterday := today.AddDays(-1)
		twoDaysAgo := today.AddDays(-2)

		// Create sessions across multiple days
		session1Out := twoDaysAgo.BerlinMidnight().Add(16 * time.Hour)
		session1 := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         twoDaysAgo,
			Status:       active.WorkSessionStatusPresent,
			CheckInTime:  twoDaysAgo.BerlinMidnight().Add(8 * time.Hour),
			CheckOutTime: &session1Out,
			CreatedBy:    staff.ID,
		}
		session2Out := yesterday.BerlinMidnight().Add(16 * time.Hour)
		session2 := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         yesterday,
			Status:       active.WorkSessionStatusPresent,
			CheckInTime:  yesterday.BerlinMidnight().Add(8 * time.Hour),
			CheckOutTime: &session2Out,
			CreatedBy:    staff.ID,
		}

		err := repo.Create(ctx, session1)
		require.NoError(t, err)
		err = repo.Create(ctx, session2)
		require.NoError(t, err)

		history, err := repo.GetHistoryByStaffID(ctx, staff.ID, twoDaysAgo, today)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(history), 2)
	})

	t.Run("returns empty for date range with no sessions", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		futureDate := timezone.NewDate(2026, 8, 24).AddDays(365)
		history, err := repo.GetHistoryByStaffID(ctx, staff.ID, futureDate, futureDate)
		require.NoError(t, err)
		assert.Empty(t, history)
	})
}

func TestWorkSessionRepository_GetHistoryByStaffIDWrapsDatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)
	today := timezone.NewDate(2026, 8, 24)

	_, err := repo.GetHistoryByStaffID(ctx, 7, today, today)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get history by staff ID")
}

func TestWorkSessionRepository_GetOpenSessions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("finds sessions without check-out before date", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		yesterday := timezone.TodayDate().AddDays(-1)
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        yesterday,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now().AddDate(0, 0, -1),
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		openSessions, err := repo.GetOpenSessions(ctx, timezone.TodayDate())
		require.NoError(t, err)
		assert.NotEmpty(t, openSessions)

		var found bool
		for _, s := range openSessions {
			if s.ID == session.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("excludes sessions with check-out time", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		yesterday := timezone.TodayDate().AddDays(-1)
		checkOutTime := time.Now().AddDate(0, 0, -1).Add(4 * time.Hour)
		session := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         yesterday,
			Status:       active.WorkSessionStatusPresent,
			CheckInTime:  time.Now().AddDate(0, 0, -1),
			CheckOutTime: &checkOutTime,
			CreatedBy:    staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		openSessions, err := repo.GetOpenSessions(ctx, timezone.TodayDate())
		require.NoError(t, err)

		for _, s := range openSessions {
			assert.NotEqual(t, session.ID, s.ID)
		}
	})
}

func TestWorkSessionRepository_GetOpenSessionsWrapsDatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession

	_, err := repo.GetOpenSessions(testpkg.Ctx(t), timezone.TodayDate())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get open sessions")
}

func TestWorkSessionRepository_GetTodayPresenceMap(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := activeRepo.NewWorkSessionRepository(db, func() time.Time {
		return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	})
	ctx := testpkg.Ctx(t)

	t.Run("returns presence map for today", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
		staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
		today := timezone.NewDate(2026, 8, 24)

		// Active session
		session1 := &active.WorkSession{
			StaffID:     staff1.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
			CreatedBy:   staff1.ID,
		}

		// Checked-out session
		checkOutTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		session2 := &active.WorkSession{
			StaffID:      staff2.ID,
			Date:         today,
			Status:       active.WorkSessionStatusHomeOffice,
			CheckInTime:  time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC).Add(-2 * time.Hour),
			CheckOutTime: &checkOutTime,
			CreatedBy:    staff2.ID,
		}

		err := repo.Create(ctx, session1)
		require.NoError(t, err)
		err = repo.Create(ctx, session2)
		require.NoError(t, err)

		presenceMap, err := repo.GetTodayPresenceMap(ctx)
		require.NoError(t, err)
		assert.Equal(t, active.WorkSessionStatusPresent, presenceMap[staff1.ID])
		assert.Equal(t, "checked_out", presenceMap[staff2.ID])
	})

	t.Run("open blocks past the live limit stop reporting presence", func(t *testing.T) {
		running := testpkg.CreateTestStaff(t, db, "Night", "Block")
		forgotten := testpkg.CreateTestStaff(t, db, "Forgotten", "Checkout")
		now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		today := timezone.DateFromTime(now)

		// Filed yesterday, checked in two hours ago: a block that ran across
		// Berlin midnight. Its owner is at work right now.
		nightBlock := &active.WorkSession{
			StaffID:     running.ID,
			Date:        today.AddDays(-1),
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: now.Add(-2 * time.Hour),
			CreatedBy:   running.ID,
		}
		// A checkout that never happened three days ago. The balance stopped
		// crediting it at the end of its own day (BalanceSessionEnd); presence
		// has to stop with it instead of reporting the person present forever.
		staleBlock := &active.WorkSession{
			StaffID:     forgotten.ID,
			Date:        today.AddDays(-3),
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: now.Add(-72 * time.Hour),
			CreatedBy:   forgotten.ID,
		}

		require.NoError(t, repo.Create(ctx, nightBlock))
		require.NoError(t, repo.Create(ctx, staleBlock))

		presenceMap, err := repo.GetTodayPresenceMap(ctx)
		require.NoError(t, err)
		assert.Equal(t, active.WorkSessionStatusPresent, presenceMap[running.ID])
		_, listed := presenceMap[forgotten.ID]
		assert.False(t, listed, "a stale open block must not report presence")
	})
}

func TestWorkSessionRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("lists all work sessions", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		sessions, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, sessions)
	})

	t.Run("lists with query options", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)

		sessions, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(sessions), 10)
	})
}

func TestWorkSessionRepository_ListWrapsDatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession

	_, err := repo.List(testpkg.Ctx(t), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list")
}

// ============================================================================
// Update Tests
// ============================================================================

func TestWorkSessionRepository_UpdateBreakMinutes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("updates break minutes", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         today,
			Status:       active.WorkSessionStatusPresent,
			CheckInTime:  time.Now(),
			BreakMinutes: 0,
			CreatedBy:    staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		err = repo.UpdateBreakMinutes(ctx, session.ID, 30)
		require.NoError(t, err)

		updated, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, 30, updated.BreakMinutes)
	})

	t.Run("does not update sessions outside the tenant", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		otherTenant := testpkg.NewTenantScope(t, db)
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         today,
			Status:       active.WorkSessionStatusPresent,
			CheckInTime:  time.Now(),
			BreakMinutes: 0,
			CreatedBy:    staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		err = repo.UpdateBreakMinutes(otherTenant.Context(), session.ID, 45)
		require.Error(t, err)

		unchanged, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, unchanged.BreakMinutes)
	})
}

func TestWorkSessionRepository_UpdateBreakMinutesWrapsDatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession

	err := repo.UpdateBreakMinutes(testpkg.Ctx(t), 1, 30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update columns")
}

func TestWorkSessionRepository_CloseSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("closes session with check-out time", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		checkOutTime := time.Now()
		didClose, err := repo.CloseSession(ctx, session.ID, checkOutTime, false)
		require.NoError(t, err)
		assert.True(t, didClose)

		closed, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, closed.CheckOutTime)
		assert.False(t, closed.AutoCheckedOut)
	})

	t.Run("closes session with auto-checkout flag", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		checkOutTime := time.Now()
		didClose, err := repo.CloseSession(ctx, session.ID, checkOutTime, true)
		require.NoError(t, err)
		assert.True(t, didClose)

		closed, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, closed.CheckOutTime)
		assert.True(t, closed.AutoCheckedOut)
	})

	t.Run("does not close already closed session", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		firstCheckOut := time.Now()
		session := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         today,
			Status:       active.WorkSessionStatusPresent,
			CheckInTime:  time.Now().Add(-2 * time.Hour),
			CheckOutTime: &firstCheckOut,
			CreatedBy:    staff.ID,
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		// Try to close again - should be no-op due to WHERE clause
		newCheckOut := time.Now()
		didClose, err := repo.CloseSession(ctx, session.ID, newCheckOut, false)
		require.NoError(t, err)
		assert.False(t, didClose)

		// Original check-out time should remain
		closed, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, closed.CheckOutTime)
		assert.WithinDuration(t, firstCheckOut, *closed.CheckOutTime, time.Second)
	})
}

func TestWorkSessionRepository_CloseSessionWrapsDatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession

	_, err := repo.CloseSession(testpkg.Ctx(t), 1, time.Now(), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close session")
}

// GetLatestOpenByStaffID answers "is this person clocked in right now", so it
// applies the same live limit as the balance (#2402): a block that crossed
// Berlin midnight is still running, a checkout that never happened is not.
// Without the cutoff a single forgotten checkout would be reported as the
// current session forever and reject every later check-in.
func TestWorkSessionRepository_GetLatestOpenByStaffID_LiveWindow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	t.Run("returns a block that crossed midnight", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Night", "Staff")
		yesterday := timezone.TodayDate().AddDays(-1)
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        yesterday,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now().Add(-3 * time.Hour),
			CreatedBy:   staff.ID,
		}
		require.NoError(t, repo.Create(ctx, session))

		found, err := repo.GetLatestOpenByStaffID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, found.ID)
	})

	t.Run("ignores a block past the live limit", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Forgot", "Staff")
		threeDaysAgo := timezone.TodayDate().AddDays(-3)
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        threeDaysAgo,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: threeDaysAgo.BerlinMidnight().Add(8 * time.Hour),
			CreatedBy:   staff.ID,
		}
		require.NoError(t, repo.Create(ctx, session))

		_, err := repo.GetLatestOpenByStaffID(ctx, staff.ID)
		require.Error(t, err, "an expired block is not a running one")
	})

	t.Run("keeps a long block filed on today", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Long", "Staff")
		today := timezone.TodayDate()
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: today.BerlinMidnight(),
			CreatedBy:   staff.ID,
		}
		require.NoError(t, repo.Create(ctx, session))

		found, err := repo.GetLatestOpenByStaffID(ctx, staff.ID)
		require.NoError(t, err, "a long shift on today is not a mistake")
		assert.Equal(t, session.ID, found.ID)
	})
}

// The history range bounds `from` against check_out_time, never against the
// stored date or the check-in: a block that began days earlier and ends inside
// the range is part of the answer. This is the contract the history tables rely
// on — it is why they can ask for the visible range without knowing how far
// back a block might start (#2402).
func TestWorkSessionRepository_ListOverlappingByStaffID_KeepsEarlierStarts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).WorkSession
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Early", "Staff")
	from := timezone.NewDate(2026, 8, 24)
	to := from.AddDays(6)

	start := from.AddDays(-5)
	checkOut := from.BerlinMidnight().Add(10 * time.Hour)
	reaching := &active.WorkSession{
		StaffID:      staff.ID,
		Date:         start,
		Status:       active.WorkSessionStatusPresent,
		CheckInTime:  start.BerlinMidnight().Add(8 * time.Hour),
		CheckOutTime: &checkOut,
		CreatedBy:    staff.ID,
	}
	require.NoError(t, repo.Create(ctx, reaching))

	endedBefore := start.BerlinMidnight().Add(12 * time.Hour)
	past := &active.WorkSession{
		StaffID:      staff.ID,
		Date:         start,
		Status:       active.WorkSessionStatusPresent,
		CheckInTime:  start.BerlinMidnight().Add(9 * time.Hour),
		CheckOutTime: &endedBefore,
		CreatedBy:    staff.ID,
	}
	require.NoError(t, repo.Create(ctx, past))

	rangeEnd := to.AddDays(1).BerlinMidnight()
	found, err := repo.ListOverlappingByStaffID(ctx, staff.ID, from.BerlinMidnight(), &rangeEnd)
	require.NoError(t, err)

	ids := make([]int64, 0, len(found))
	for _, session := range found {
		ids = append(ids, session.ID)
	}
	assert.Contains(t, ids, reaching.ID, "a block ending inside the range belongs to it, however early it started")
	assert.NotContains(t, ids, past.ID, "a block that ended before the range does not")
}
