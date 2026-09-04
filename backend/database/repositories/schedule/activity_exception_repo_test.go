package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func activityExceptionRepository(t *testing.T, db *bun.DB) scheduleModels.ActivityExceptionRepository {
	t.Helper()
	factory := repositories.NewFactory(db)
	factory.BindTimetable(timetabletest.New(t, db))
	return factory.ActivityException
}

func TestActivityExceptionRepository_Create_and_FindByActivityGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := activityExceptionRepository(t, db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-Tpl-%d", time.Now().UnixNano()))

	cancelled := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   timezone.NewDate(2026, 12, 23),
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	cancelled.SetTenantID(testpkg.Tenant(t))

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("OverrideRoom-%d", time.Now().UnixNano()))

	modifiedRoom := room.ID
	modified := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   timezone.NewDate(2026, 12, 24),
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		RoomID:          &modifiedRoom,
	}
	modified.SetTenantID(testpkg.Tenant(t))

	require.NoError(t, repo.Create(ctx, cancelled))
	require.NoError(t, repo.Create(ctx, modified))

	t.Run("FindByActivityGroupID returns both", func(t *testing.T) {
		got, err := repo.FindByActivityGroupID(ctx, activity.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(got), 2)
	})

	t.Run("Create rejects nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("UNIQUE(activity_group_id, exception_date) blocks duplicate", func(t *testing.T) {
		dup := &scheduleModels.ActivityException{
			ActivityGroupID: activity.ID,
			ExceptionDate:   cancelled.ExceptionDate,
			ExceptionType:   scheduleModels.ActivityExceptionCancelled,
		}
		dup.SetTenantID(testpkg.Tenant(t))
		err := repo.Create(ctx, dup)
		require.Error(t, err)
	})

	t.Run("Create rejects cancelled with overrides", func(t *testing.T) {
		bad := &scheduleModels.ActivityException{
			ActivityGroupID: activity.ID,
			ExceptionDate:   timezone.NewDate(2027, 1, 10),
			ExceptionType:   scheduleModels.ActivityExceptionCancelled,
			RoomID:          &modifiedRoom,
		}
		bad.SetTenantID(testpkg.Tenant(t))
		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled exception must not set")
	})

	t.Run("Create rejects modified without overrides", func(t *testing.T) {
		bad := &scheduleModels.ActivityException{
			ActivityGroupID: activity.ID,
			ExceptionDate:   timezone.NewDate(2027, 1, 11),
			ExceptionType:   scheduleModels.ActivityExceptionModified,
		}
		bad.SetTenantID(testpkg.Tenant(t))
		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must override at least one")
	})
}

func TestActivityExceptionRepository_FindByActivityGroupAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := activityExceptionRepository(t, db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-By-Date-%d", time.Now().UnixNano()))

	date := timezone.NewDate(2027, 2, 3)

	exc := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   date,
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exc.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, exc))

	t.Run("returns the row when exists", func(t *testing.T) {
		got, err := repo.FindByActivityGroupAndDate(ctx, activity.ID, date)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, exc.ID, got.ID)
	})

	t.Run("returns nil when missing", func(t *testing.T) {
		other := timezone.NewDate(2027, 3, 15)
		got, err := repo.FindByActivityGroupAndDate(ctx, activity.ID, other)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestActivityExceptionRepository_FindByDateRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := activityExceptionRepository(t, db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-Range-%d", time.Now().UnixNano()))

	from := timezone.NewDate(2027, 4, 1)
	mid := timezone.NewDate(2027, 4, 10)
	to := timezone.NewDate(2027, 4, 30)
	outside := timezone.NewDate(2027, 6, 1)

	mk := func(date timezone.Date) *scheduleModels.ActivityException {
		e := &scheduleModels.ActivityException{
			ActivityGroupID: activity.ID,
			ExceptionDate:   date,
			ExceptionType:   scheduleModels.ActivityExceptionCancelled,
		}
		e.SetTenantID(testpkg.Tenant(t))
		return e
	}
	eStart, eMid, eEnd, eOut := mk(from), mk(mid), mk(to), mk(outside)

	for _, e := range []*scheduleModels.ActivityException{eStart, eMid, eEnd, eOut} {
		require.NoError(t, repo.Create(ctx, e))
	}

	got, err := repo.FindByDateRange(ctx, from, to)
	require.NoError(t, err)

	ids := map[int64]bool{}
	for _, r := range got {
		ids[r.ID] = true
	}
	assert.True(t, ids[eStart.ID], "from-date inclusive")
	assert.True(t, ids[eMid.ID])
	assert.True(t, ids[eEnd.ID], "to-date inclusive")
	assert.False(t, ids[eOut.ID], "outside range must not appear")
}

func TestActivityExceptionRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := activityExceptionRepository(t, db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-Upd-%d", time.Now().UnixNano()))

	exc := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   timezone.NewDate(2027, 5, 1),
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exc.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, exc))

	t.Run("updates fields in place", func(t *testing.T) {
		reason := "Ferien"
		exc.Reason = &reason

		require.NoError(t, repo.Update(ctx, exc))

		got, err := repo.FindByID(ctx, exc.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Reason)
		assert.Equal(t, "Ferien", *got.Reason)
	})

	t.Run("rejects nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("rejects invalid payload", func(t *testing.T) {
		bad := &scheduleModels.ActivityException{
			ActivityGroupID: 0,
			ExceptionDate:   timezone.NewDate(2027, 5, 2),
			ExceptionType:   scheduleModels.ActivityExceptionCancelled,
		}
		bad.SetTenantID(testpkg.Tenant(t))
		bad.ID = exc.ID
		err := repo.Update(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "activity_group_id is required")
	})
}

func TestActivityExceptionRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := activityExceptionRepository(t, db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-FID-%d", time.Now().UnixNano()))

	exc := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   timezone.NewDate(2027, 5, 3),
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exc.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, exc))

	t.Run("returns row for known id", func(t *testing.T) {
		got, err := repo.FindByID(ctx, exc.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, exc.ID, got.ID)
	})

	t.Run("wraps sql.ErrNoRows for missing id", func(t *testing.T) {
		got, err := repo.FindByID(ctx, int64(999999999))
		require.Error(t, err)
		assert.Nil(t, got)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by id", dbErr.Op)
	})
}

func TestActivityExceptionRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := activityExceptionRepository(t, db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-List-%d", time.Now().UnixNano()))

	exc := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   timezone.NewDate(2027, 5, 4),
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exc.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, exc))

	t.Run("nil options returns rows", func(t *testing.T) {
		rows, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rows), 1)
	})

	t.Run("with filter + pagination", func(t *testing.T) {
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("activity_group_id", activity.ID)
		options.WithPagination(1, 10)

		rows, err := repo.List(ctx, options)
		require.NoError(t, err)
		require.NotEmpty(t, rows)
		for _, r := range rows {
			assert.Equal(t, activity.ID, r.ActivityGroupID)
		}
	})

	t.Run("wraps driver errors in DatabaseError", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		rows, err := repo.List(cancelledCtx, nil)
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "list with options", dbErr.Op)
	})
}

func TestActivityExceptionRepository_ErrorBranches(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := activityExceptionRepository(t, db)

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	date := timezone.NewDate(2027, 5, 5)

	t.Run("FindByActivityGroupID wraps driver errors", func(t *testing.T) {
		rows, err := repo.FindByActivityGroupID(cancelledCtx, int64(999999))
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by activity group id", dbErr.Op)
	})

	t.Run("FindByActivityGroupAndDate wraps driver errors", func(t *testing.T) {
		row, err := repo.FindByActivityGroupAndDate(cancelledCtx, int64(999999), date)
		assert.Nil(t, row)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by activity group and date", dbErr.Op)
	})

	t.Run("FindByDateRange wraps driver errors", func(t *testing.T) {
		rows, err := repo.FindByDateRange(cancelledCtx, date, date)
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by date range", dbErr.Op)
	})
}

// TestActivityExceptionFKOnDelete verifies:
//   - activity_group_id → CASCADE (template delete removes its exceptions)
//   - room_id           → SET NULL (deleting an override room clears the hint, not the exception)
//
// Catalog code: 'c' = CASCADE, 'n' = SET NULL.
func TestActivityExceptionFKOnDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	cases := []struct {
		refTable string
		refCol   string
		wantCode string
	}{
		{"activities.groups", "activity_group_id", "c"},
		{"facilities.rooms", "room_id", "n"},
	}

	for _, tc := range cases {
		t.Run(tc.refTable+"/"+tc.refCol, func(t *testing.T) {
			var confdeltype string
			err := db.NewRaw(`
				SELECT c.confdeltype
				FROM pg_constraint c
				JOIN pg_class t       ON t.oid = c.conrelid
				JOIN pg_namespace tns ON tns.oid = t.relnamespace
				JOIN pg_class rt      ON rt.oid = c.confrelid
				JOIN pg_namespace rns ON rns.oid = rt.relnamespace
				JOIN pg_attribute a   ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
				WHERE c.contype = 'f'
				  AND tns.nspname || '.' || t.relname = 'schedule.activity_exceptions'
				  AND rns.nspname || '.' || rt.relname = ?
				  AND a.attname = ?
				LIMIT 1
			`, tc.refTable, tc.refCol).Scan(ctx, &confdeltype)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCode, confdeltype,
				"activity_exceptions.%s FK to %s must have confdeltype=%q (got %q)",
				tc.refCol, tc.refTable, tc.wantCode, confdeltype)
		})
	}
}
