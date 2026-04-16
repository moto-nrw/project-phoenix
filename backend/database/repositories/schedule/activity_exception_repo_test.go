package schedule_test

import (
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityExceptionRepository_Create_and_FindByActivityGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityExceptionRepository(db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-Tpl-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID)

	cancelled := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   time.Date(2026, 12, 23, 0, 0, 0, 0, time.UTC),
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	cancelled.SetTenantID(1)

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("OverrideRoom-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, room.ID)

	modifiedRoom := room.ID
	modified := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC),
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		RoomID:          &modifiedRoom,
	}
	modified.SetTenantID(1)

	require.NoError(t, repo.Create(ctx, cancelled))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_exceptions", cancelled.ID)
	require.NoError(t, repo.Create(ctx, modified))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_exceptions", modified.ID)

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
		dup.SetTenantID(1)
		err := repo.Create(ctx, dup)
		require.Error(t, err)
	})

	t.Run("Create rejects cancelled with overrides", func(t *testing.T) {
		bad := &scheduleModels.ActivityException{
			ActivityGroupID: activity.ID,
			ExceptionDate:   time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC),
			ExceptionType:   scheduleModels.ActivityExceptionCancelled,
			RoomID:          &modifiedRoom,
		}
		bad.SetTenantID(1)
		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled exception must not set")
	})

	t.Run("Create rejects modified without overrides", func(t *testing.T) {
		bad := &scheduleModels.ActivityException{
			ActivityGroupID: activity.ID,
			ExceptionDate:   time.Date(2027, 1, 11, 0, 0, 0, 0, time.UTC),
			ExceptionType:   scheduleModels.ActivityExceptionModified,
		}
		bad.SetTenantID(1)
		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must override at least one")
	})
}

func TestActivityExceptionRepository_FindByActivityGroupAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityExceptionRepository(db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-By-Date-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID)

	date := time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC)

	exc := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   date,
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exc.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, exc))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_exceptions", exc.ID)

	t.Run("returns the row when exists", func(t *testing.T) {
		got, err := repo.FindByActivityGroupAndDate(ctx, activity.ID, date)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, exc.ID, got.ID)
	})

	t.Run("returns nil when missing", func(t *testing.T) {
		other := time.Date(2027, 3, 15, 0, 0, 0, 0, time.UTC)
		got, err := repo.FindByActivityGroupAndDate(ctx, activity.ID, other)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestActivityExceptionRepository_FindByDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityExceptionRepository(db)

	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exc-Range-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID)

	from := time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2027, 4, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2027, 4, 30, 0, 0, 0, 0, time.UTC)
	outside := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)

	mk := func(date time.Time) *scheduleModels.ActivityException {
		e := &scheduleModels.ActivityException{
			ActivityGroupID: activity.ID,
			ExceptionDate:   date,
			ExceptionType:   scheduleModels.ActivityExceptionCancelled,
		}
		e.SetTenantID(1)
		return e
	}
	eStart, eMid, eEnd, eOut := mk(from), mk(mid), mk(to), mk(outside)

	for _, e := range []*scheduleModels.ActivityException{eStart, eMid, eEnd, eOut} {
		require.NoError(t, repo.Create(ctx, e))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_exceptions", e.ID)
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

// TestActivityExceptionFKOnDelete verifies:
//   - activity_group_id → CASCADE (template delete removes its exceptions)
//   - room_id           → SET NULL (deleting an override room clears the hint, not the exception)
//
// Catalog code: 'c' = CASCADE, 'n' = SET NULL.
func TestActivityExceptionFKOnDelete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

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
