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
// CRUD Tests
// ============================================================================

func TestWorkSessionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("creates work session with valid data", func(t *testing.T) {
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

		testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)
	})

	t.Run("creates work session with home office status", func(t *testing.T) {
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

		testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)
	})

	t.Run("create with nil session should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid status should fail", func(t *testing.T) {
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

func TestWorkSessionRepository_ListByStaffAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("returns all blocks of the day ordered by check-in", func(t *testing.T) {
		today := timezone.TodayDate()
		firstOut := time.Now().Add(-2 * time.Hour)
		first := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         today,
			Status:       active.WorkSessionStatusHomeOffice,
			CheckInTime:  time.Now().Add(-6 * time.Hour),
			CheckOutTime: &firstOut,
			CreatedBy:    staff.ID,
		}
		require.NoError(t, repo.Create(ctx, first))
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", first.ID)

		second := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now().Add(-30 * time.Minute),
			CreatedBy:   staff.ID,
		}
		require.NoError(t, repo.Create(ctx, second),
			"a second block on the same day must be insertable since #2402")
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", second.ID)

		found, err := repo.ListByStaffAndDate(ctx, staff.ID, today)
		require.NoError(t, err)
		require.Len(t, found, 2)
		assert.Equal(t, first.ID, found[0].ID, "blocks come back in check-in order")
		assert.Equal(t, second.ID, found[1].ID)
		assert.Equal(t, active.WorkSessionStatusHomeOffice, found[0].Status)
		assert.Equal(t, active.WorkSessionStatusPresent, found[1].Status)
	})

	t.Run("returns empty list for a day without sessions", func(t *testing.T) {
		found, err := repo.ListByStaffAndDate(ctx, staff.ID, timezone.TodayDate().AddDays(7))
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

// TestWorkSessionRepository_SecondOpenBlockPerDayIsRejected pins the partial
// unique index from migration 1.15.305: several CLOSED blocks per day are
// fine, but at most one block per staff and day may be open.
func TestWorkSessionRepository_SecondOpenBlockPerDayIsRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	scope := testpkg.NewTenantScope(t, db)
	ctx := scope.Context()

	staff := testpkg.CreateTestStaffForTenant(t, db, scope.TenantID, "Open", "Blocks")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	today := timezone.TodayDate()
	open := &active.WorkSession{
		StaffID:     staff.ID,
		Date:        today,
		Status:      active.WorkSessionStatusPresent,
		CheckInTime: time.Now().Add(-1 * time.Hour),
		CreatedBy:   staff.ID,
	}
	require.NoError(t, repo.Create(ctx, open))
	defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", open.ID)

	secondOpen := &active.WorkSession{
		StaffID:     staff.ID,
		Date:        today,
		Status:      active.WorkSessionStatusPresent,
		CheckInTime: time.Now(),
		CreatedBy:   staff.ID,
	}
	err := repo.Create(ctx, secondOpen)
	require.Error(t, err, "two OPEN blocks on one day must hit uq_work_sessions_staff_date_open")
	if secondOpen.ID != 0 {
		testpkg.CleanupTableRecords(t, db, "active.work_sessions", secondOpen.ID)
	}
}

// TestWorkSessionRepository_ListOverlappingByStaffID pins the overlap lookup
// behind the #2402 block guard. It compares timestamps, not the date column,
// so a block filed days earlier that still runs into the candidate interval is
// part of the answer — a date window would miss exactly that case.
func TestWorkSessionRepository_ListOverlappingByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	scope := testpkg.NewTenantScope(t, db)
	ctx := scope.Context()

	staff := testpkg.CreateTestStaffForTenant(t, db, scope.TenantID, "Overlap", "Lookup")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	today := timezone.TodayDate()
	base := today.BerlinMidnight()
	at := func(dayOffset, hour int) time.Time {
		return base.AddDate(0, 0, dayOffset).Add(time.Duration(hour) * time.Hour)
	}
	create := func(date timezone.Date, checkIn time.Time, checkOut *time.Time) *active.WorkSession {
		s := &active.WorkSession{
			StaffID:      staff.ID,
			Date:         date,
			Status:       active.WorkSessionStatusPresent,
			CheckInTime:  checkIn,
			CheckOutTime: checkOut,
			CreatedBy:    staff.ID,
		}
		require.NoError(t, repo.Create(ctx, s))
		return s
	}

	morningEnd := at(0, 12)
	morning := create(today, at(0, 8), &morningEnd)
	defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", morning.ID)
	// Opened three days ago and never closed (a missed auto-checkout).
	stale := create(today.AddDays(-3), at(-3, 9), nil)
	defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", stale.ID)

	t.Run("finds the block a closed candidate runs into", func(t *testing.T) {
		to := at(0, 14)
		found, err := repo.ListOverlappingByStaffID(ctx, staff.ID, at(0, 11), &to)
		require.NoError(t, err)
		ids := make([]int64, 0, len(found))
		for _, s := range found {
			ids = append(ids, s.ID)
		}
		assert.Contains(t, ids, morning.ID)
		assert.Contains(t, ids, stale.ID, "an open block from days ago still overlaps")
	})

	t.Run("ignores a block that only touches the candidate", func(t *testing.T) {
		to := at(0, 16)
		found, err := repo.ListOverlappingByStaffID(ctx, staff.ID, at(0, 12), &to)
		require.NoError(t, err)
		for _, s := range found {
			assert.NotEqual(t, morning.ID, s.ID, "12:00 starts exactly where the morning block ends")
		}
	})

	t.Run("an open candidate reaches every later block", func(t *testing.T) {
		found, err := repo.ListOverlappingByStaffID(ctx, staff.ID, at(0, 13), nil)
		require.NoError(t, err)
		ids := make([]int64, 0, len(found))
		for _, s := range found {
			ids = append(ids, s.ID)
		}
		assert.NotContains(t, ids, morning.ID, "the morning block ended before 13:00")
		assert.Contains(t, ids, stale.ID)
	})
}

func TestWorkSessionRepository_GetCurrentByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("finds active session for staff today", func(t *testing.T) {
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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		found, err := repo.GetCurrentByStaffID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, found.ID)
		assert.Nil(t, found.CheckOutTime)
	})

	t.Run("returns error when no active session exists", func(t *testing.T) {
		_, err := repo.GetCurrentByStaffID(ctx, staff.ID)
		require.Error(t, err)
	})

	t.Run("ignores checked-out sessions", func(t *testing.T) {
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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		_, err = repo.GetCurrentByStaffID(ctx, staff.ID)
		require.Error(t, err) // Should not find checked-out session
	})
}

func TestWorkSessionRepository_GetHistoryByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("returns sessions in date range", func(t *testing.T) {
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)
		twoDaysAgo := today.AddDays(-2)

		// Create sessions across multiple days
		session1 := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        twoDaysAgo,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now().AddDate(0, 0, -2),
			CreatedBy:   staff.ID,
		}
		session2 := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        yesterday,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now().AddDate(0, 0, -1),
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, session1)
		require.NoError(t, err)
		err = repo.Create(ctx, session2)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.work_sessions", session1.ID)
			testpkg.CleanupTableRecords(t, db, "active.work_sessions", session2.ID)
		}()

		history, err := repo.GetHistoryByStaffID(ctx, staff.ID, twoDaysAgo, today)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(history), 2)
	})

	t.Run("returns empty for date range with no sessions", func(t *testing.T) {
		futureDate := timezone.TodayDate().AddDays(365)
		history, err := repo.GetHistoryByStaffID(ctx, staff.ID, futureDate, futureDate)
		require.NoError(t, err)
		assert.Empty(t, history)
	})
}

func TestWorkSessionRepository_GetHistoryByStaffIDWrapsDatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)
	today := timezone.TodayDate()

	_, err := repo.GetHistoryByStaffID(ctx, 7, today, today)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get history by staff ID")
}

func TestWorkSessionRepository_GetOpenSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("finds sessions without check-out before date", func(t *testing.T) {
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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		openSessions, err := repo.GetOpenSessions(ctx, timezone.TodayDate())
		require.NoError(t, err)

		for _, s := range openSessions {
			assert.NotEqual(t, session.ID, s.ID)
		}
	})
}

func TestWorkSessionRepository_GetOpenSessionsWrapsDatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db).WorkSession

	_, err := repo.GetOpenSessions(testpkg.TenantContext(1), timezone.TodayDate())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get open sessions")
}

func TestWorkSessionRepository_GetTodayPresenceMap(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
	staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
	defer func() {
		testpkg.CleanupActivityFixtures(t, db, 0, staff1.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, staff2.ID)
	}()

	t.Run("returns presence map for today", func(t *testing.T) {
		today := timezone.TodayDate()

		// Active session
		session1 := &active.WorkSession{
			StaffID:     staff1.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff1.ID,
		}

		// Checked-out session
		checkOutTime := time.Now()
		session2 := &active.WorkSession{
			StaffID:      staff2.ID,
			Date:         today,
			Status:       active.WorkSessionStatusHomeOffice,
			CheckInTime:  time.Now().Add(-2 * time.Hour),
			CheckOutTime: &checkOutTime,
			CreatedBy:    staff2.ID,
		}

		err := repo.Create(ctx, session1)
		require.NoError(t, err)
		err = repo.Create(ctx, session2)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.work_sessions", session1.ID)
			testpkg.CleanupTableRecords(t, db, "active.work_sessions", session2.ID)
		}()

		presenceMap, err := repo.GetTodayPresenceMap(ctx)
		require.NoError(t, err)
		assert.Equal(t, active.WorkSessionStatusPresent, presenceMap[staff1.ID])
		assert.Equal(t, "checked_out", presenceMap[staff2.ID])
	})
}

func TestWorkSessionRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("lists all work sessions", func(t *testing.T) {
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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		sessions, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, sessions)
	})

	t.Run("lists with query options", func(t *testing.T) {
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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)

		sessions, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(sessions), 10)
	})
}

func TestWorkSessionRepository_ListWrapsDatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db).WorkSession

	_, err := repo.List(testpkg.TenantContext(1), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list")
}

// ============================================================================
// Update Tests
// ============================================================================

func TestWorkSessionRepository_UpdateBreakMinutes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("updates break minutes", func(t *testing.T) {
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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		err = repo.UpdateBreakMinutes(ctx, session.ID, 30)
		require.NoError(t, err)

		updated, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, 30, updated.BreakMinutes)
	})

	t.Run("does not update sessions outside the tenant", func(t *testing.T) {
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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		err = repo.UpdateBreakMinutes(otherTenant.Context(), session.ID, 45)
		require.Error(t, err)

		unchanged, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, unchanged.BreakMinutes)
	})
}

func TestWorkSessionRepository_UpdateBreakMinutesWrapsDatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db).WorkSession

	err := repo.UpdateBreakMinutes(testpkg.TenantContext(1), 1, 30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update columns")
}

func TestWorkSessionRepository_CloseSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("closes session with check-out time", func(t *testing.T) {
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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

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
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

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
	db := testpkg.SetupTestDB(t)
	require.NoError(t, db.Close())

	repo := repositories.NewFactory(db).WorkSession

	_, err := repo.CloseSession(testpkg.TenantContext(1), 1, time.Now(), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close session")
}
