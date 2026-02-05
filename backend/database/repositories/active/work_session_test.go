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
// CRUD Tests
// ============================================================================

func TestWorkSessionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("creates work session with valid data", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
		today := timezone.DateOfUTC(time.Now())
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
		today := timezone.DateOfUTC(time.Now())
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("finds existing session by staff and date", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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

		found, err := repo.GetByStaffAndDate(ctx, staff.ID, today)
		require.NoError(t, err)
		assert.Equal(t, session.ID, found.ID)
		assert.Equal(t, staff.ID, found.StaffID)
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		_, err := repo.GetByStaffAndDate(ctx, staff.ID, today)
		require.Error(t, err)
	})
}

func TestWorkSessionRepository_GetCurrentByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("finds active session for staff today", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
		today := timezone.DateOfUTC(time.Now())
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
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("returns sessions in date range", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		yesterday := today.AddDate(0, 0, -1)
		twoDaysAgo := today.AddDate(0, 0, -2)

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
		futureDate := timezone.DateOfUTC(time.Now().AddDate(1, 0, 0))
		history, err := repo.GetHistoryByStaffID(ctx, staff.ID, futureDate, futureDate)
		require.NoError(t, err)
		assert.Empty(t, history)
	})
}

func TestWorkSessionRepository_GetOpenSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("finds sessions without check-out before date", func(t *testing.T) {
		yesterday := timezone.DateOfUTC(time.Now().AddDate(0, 0, -1))
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

		openSessions, err := repo.GetOpenSessions(ctx, timezone.TodayUTC())
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
		yesterday := timezone.DateOfUTC(time.Now().AddDate(0, 0, -1))
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

		openSessions, err := repo.GetOpenSessions(ctx, timezone.TodayUTC())
		require.NoError(t, err)

		for _, s := range openSessions {
			assert.NotEqual(t, session.ID, s.ID)
		}
	})
}

func TestWorkSessionRepository_GetTodayPresenceMap(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
	staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
	defer func() {
		testpkg.CleanupActivityFixtures(t, db, 0, staff1.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, staff2.ID)
	}()

	t.Run("returns presence map for today", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())

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
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("lists all work sessions", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
		today := timezone.DateOfUTC(time.Now())
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

// ============================================================================
// Update Tests
// ============================================================================

func TestWorkSessionRepository_UpdateBreakMinutes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("updates break minutes", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
}

func TestWorkSessionRepository_CloseSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("closes session with check-out time", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
		err = repo.CloseSession(ctx, session.ID, checkOutTime, false)
		require.NoError(t, err)

		closed, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, closed.CheckOutTime)
		assert.False(t, closed.AutoCheckedOut)
	})

	t.Run("closes session with auto-checkout flag", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
		err = repo.CloseSession(ctx, session.ID, checkOutTime, true)
		require.NoError(t, err)

		closed, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, closed.CheckOutTime)
		assert.True(t, closed.AutoCheckedOut)
	})

	t.Run("does not close already closed session", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
		err = repo.CloseSession(ctx, session.ID, newCheckOut, false)
		require.NoError(t, err)

		// Original check-out time should remain
		closed, err := repo.FindByID(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, closed.CheckOutTime)
		assert.WithinDuration(t, firstCheckOut, *closed.CheckOutTime, time.Second)
	})
}
