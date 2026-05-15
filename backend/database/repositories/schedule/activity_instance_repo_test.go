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
	"github.com/uptrace/bun"
)

// activityInstanceFixtures holds the shared dependencies a test instance row
// needs: a room (primary, always NOT NULL) and optionally a template (activity group).
type activityInstanceFixtures struct {
	roomID     int64
	activityID int64
	cleanup    func()
}

func newActivityInstanceFixtures(t *testing.T, db *bun.DB, prefix string) *activityInstanceFixtures {
	t.Helper()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s-%d", prefix, time.Now().UnixNano()))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Template-%s-%d", prefix, time.Now().UnixNano()))
	return &activityInstanceFixtures{
		roomID:     room.ID,
		activityID: activity.ID,
		cleanup: func() {
			testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID)
		},
	}
}

func buildInstance(tenantID, roomID int64, activityID *int64, date time.Time, start, end time.Time, title string) *scheduleModels.ActivityInstance {
	inst := &scheduleModels.ActivityInstance{
		Date:            date,
		ActivityGroupID: activityID,
		Title:           title,
		StartTime:       start,
		EndTime:         end,
		RoomID:          roomID,
		Status:          scheduleModels.InstanceStatusPlanned,
	}
	inst.SetTenantID(tenantID)
	return inst
}

func TestActivityInstanceRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	t.Run("creates template-backed instance", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "create-tpl")
		defer fx.cleanup()

		inst := buildInstance(
			1, fx.roomID, &fx.activityID,
			time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
			"Lernzeit",
		)

		err := repo.Create(ctx, inst)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)

		assert.Greater(t, inst.ID, int64(0))
	})

	t.Run("creates spontaneous instance (no activity_group_id)", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "create-spon")
		defer fx.cleanup()

		inst := buildInstance(
			1, fx.roomID, nil,
			time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
			"Spontanes Kochen",
		)
		inst.IsSpontaneous = true

		err := repo.Create(ctx, inst)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)

		assert.Greater(t, inst.ID, int64(0))
		assert.True(t, inst.IsSpontaneous)
		assert.Nil(t, inst.ActivityGroupID)
	})

	t.Run("rejects nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("rejects validation failure", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "create-bad")
		defer fx.cleanup()

		inst := buildInstance(1, fx.roomID, &fx.activityID,
			time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			"Broken",
		)

		err := repo.Create(ctx, inst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "end_time must be after start_time")
	})

	t.Run("partial unique index allows multiple spontaneous rows on same slot", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "create-spon-dup")
		defer fx.cleanup()

		date := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
		start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

		a := buildInstance(1, fx.roomID, nil, date, start, end, "Spontan A")
		a.IsSpontaneous = true
		b := buildInstance(1, fx.roomID, nil, date, start, end, "Spontan B")
		b.IsSpontaneous = true

		require.NoError(t, repo.Create(ctx, a))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", a.ID)
		require.NoError(t, repo.Create(ctx, b))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", b.ID)
	})

	t.Run("partial unique index rejects duplicate template slot", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "create-dup")
		defer fx.cleanup()

		date := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
		start := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)

		a := buildInstance(1, fx.roomID, &fx.activityID, date, start, end, "Lernzeit A")
		require.NoError(t, repo.Create(ctx, a))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", a.ID)

		b := buildInstance(1, fx.roomID, &fx.activityID, date, start, end, "Lernzeit B")
		err := repo.Create(ctx, b)
		require.Error(t, err, "partial UNIQUE must block same (tenant,date,group,start)")
	})
}

func TestActivityInstanceRepository_FindByID_and_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "crud")
	defer fx.cleanup()

	inst := buildInstance(
		1, fx.roomID, &fx.activityID,
		time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
		"Lernzeit",
	)
	require.NoError(t, repo.Create(ctx, inst))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)

	t.Run("FindByID returns the row", func(t *testing.T) {
		found, err := repo.FindByID(ctx, inst.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, inst.ID, found.ID)
		assert.Equal(t, "Lernzeit", found.Title)
		assert.Equal(t, scheduleModels.InstanceStatusPlanned, found.Status)
	})

	t.Run("Update mutates persisted row", func(t *testing.T) {
		inst.Title = "Lernzeit (angepasst)"
		inst.Status = scheduleModels.InstanceStatusActive
		require.NoError(t, repo.Update(ctx, inst))

		found, err := repo.FindByID(ctx, inst.ID)
		require.NoError(t, err)
		assert.Equal(t, "Lernzeit (angepasst)", found.Title)
		assert.Equal(t, scheduleModels.InstanceStatusActive, found.Status)
	})

	t.Run("Update rejects nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestActivityInstanceRepository_MarkCompletedUpdatesOnlyLifecycleColumns(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "mark-completed")
	defer fx.cleanup()

	inst := buildInstance(
		1, fx.roomID, &fx.activityID,
		time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
		"Lernzeit Lifecycle",
	)
	inst.Status = scheduleModels.InstanceStatusActive
	require.NoError(t, repo.Create(ctx, inst))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)

	completedAt := time.Date(2026, 9, 16, 15, 31, 0, 0, time.UTC)
	require.NoError(t, repo.MarkCompleted(ctx, inst.ID, completedAt))

	found, err := repo.FindByID(ctx, inst.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, scheduleModels.InstanceStatusCompleted, found.Status)
	require.NotNil(t, found.CompletedAt)
	assert.WithinDuration(t, completedAt, *found.CompletedAt, time.Second)
	assert.Equal(t, "Lernzeit Lifecycle", found.Title)
	assert.Equal(t, 14, found.StartTime.Hour())
	assert.Equal(t, 30, found.EndTime.Minute())
}

func TestActivityInstanceRepository_FindByTenantAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "by-date")
	defer fx.cleanup()

	date := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	other := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)

	a := buildInstance(1, fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), "A")
	b := buildInstance(1, fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC), "B")
	c := buildInstance(1, fx.roomID, &fx.activityID, other,
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), "C")

	require.NoError(t, repo.Create(ctx, a))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", a.ID)
	require.NoError(t, repo.Create(ctx, b))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", b.ID)
	require.NoError(t, repo.Create(ctx, c))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", c.ID)

	got, err := repo.FindByTenantAndDate(ctx, date)
	require.NoError(t, err)

	foundA, foundB, foundC := false, false, false
	for _, r := range got {
		switch r.ID {
		case a.ID:
			foundA = true
		case b.ID:
			foundB = true
		case c.ID:
			foundC = true
		}
	}
	assert.True(t, foundA)
	assert.True(t, foundB)
	assert.False(t, foundC, "other-date instance must not appear")
}

func TestActivityInstanceRepository_FindByTenantAndDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "range")
	defer fx.cleanup()

	start := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	outside := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

	s1 := buildInstance(1, fx.roomID, &fx.activityID, start,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), "start")
	s2 := buildInstance(1, fx.roomID, &fx.activityID, mid,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), "mid")
	s3 := buildInstance(1, fx.roomID, &fx.activityID, end,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), "end")
	s4 := buildInstance(1, fx.roomID, &fx.activityID, outside,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), "outside")

	for _, inst := range []*scheduleModels.ActivityInstance{s1, s2, s3, s4} {
		require.NoError(t, repo.Create(ctx, inst))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)
	}

	got, err := repo.FindByTenantAndDateRange(ctx, start, end)
	require.NoError(t, err)

	ids := map[int64]bool{}
	for _, r := range got {
		ids[r.ID] = true
	}
	assert.True(t, ids[s1.ID], "start date inclusive")
	assert.True(t, ids[s2.ID])
	assert.True(t, ids[s3.ID], "end date inclusive")
	assert.False(t, ids[s4.ID], "outside range must not appear")
}

func TestActivityInstanceRepository_FindByActivityGroupAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "grp-date")
	defer fx.cleanup()

	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	a := buildInstance(1, fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), "morning")
	b := buildInstance(1, fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC), "afternoon")

	require.NoError(t, repo.Create(ctx, a))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", a.ID)
	require.NoError(t, repo.Create(ctx, b))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", b.ID)

	got, err := repo.FindByActivityGroupAndDate(ctx, fx.activityID, date)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(got), 2)
}

func TestActivityInstanceRepository_FindByActiveGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	t.Run("returns nil when no link exists", func(t *testing.T) {
		got, err := repo.FindByActiveGroupID(ctx, int64(999999999))
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("returns the bridged instance when linked", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "active-lookup")
		defer fx.cleanup()

		ag := testpkg.CreateTestActiveGroup(t, db, fx.activityID, fx.roomID)

		inst := buildInstance(1, fx.roomID, &fx.activityID,
			time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			"linked")
		inst.ActiveGroupID = &ag.ID
		require.NoError(t, repo.Create(ctx, inst))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)

		got, err := repo.FindByActiveGroupID(ctx, ag.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, inst.ID, got.ID)
	})
}

// TestActivityInstanceRepository_ActiveGroupBridgeUnique verifies the 1:1
// bridge invariant between schedule.activity_instances and active.groups:
// two instances must not be able to claim the same active.group.
func TestActivityInstanceRepository_ActiveGroupBridgeUnique(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "bridge-uniq")
	defer fx.cleanup()

	ag := testpkg.CreateTestActiveGroup(t, db, fx.activityID, fx.roomID)

	date := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)

	first := buildInstance(1, fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		"first")
	first.ActiveGroupID = &ag.ID
	require.NoError(t, repo.Create(ctx, first))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", first.ID)

	second := buildInstance(1, fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		"second")
	second.ActiveGroupID = &ag.ID

	err := repo.Create(ctx, second)
	require.Error(t, err, "second instance claiming the same active.group must be rejected by the unique partial index")
}

// TestActivityInstanceActiveGroupUniqueIndex verifies at the schema level that
// the partial unique index on active_group_id exists with the expected
// predicate. Mirrors the FK OnDelete pattern — guards against a future
// migration silently downgrading the invariant.
func TestActivityInstanceActiveGroupUniqueIndex(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

	var isUnique bool
	var predicate string
	err := db.NewRaw(`
		SELECT ix.indisunique, pg_get_expr(ix.indpred, ix.indrelid)
		FROM pg_index ix
		JOIN pg_class c     ON c.oid = ix.indexrelid
		JOIN pg_class t     ON t.oid = ix.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'schedule'
		  AND t.relname = 'activity_instances'
		  AND c.relname = 'idx_activity_instances_active_group_unique'
	`).Scan(ctx, &isUnique, &predicate)
	require.NoError(t, err, "unique partial index on active_group_id must exist")
	assert.True(t, isUnique, "index must be UNIQUE")
	assert.Contains(t, predicate, "active_group_id IS NOT NULL", "partial predicate must exclude NULLs")
}

func TestActivityInstanceRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "list")
	defer fx.cleanup()

	inst := buildInstance(1, fx.roomID, &fx.activityID,
		time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
		"List-entry")
	require.NoError(t, repo.Create(ctx, inst))
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)

	got, err := repo.List(ctx, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(got), 1)
}

// TestActivityInstanceFKOnDelete verifies FK cleanup semantics on the
// nullable parent references. Policy per plan / WP-B5 review:
//   - activity_group_id  → SET NULL (keep instance row after template delete)
//   - calendar_period_id → SET NULL (inherited pattern from 1.15.34)
//   - active_group_id    → SET NULL (live bridge is cleared on active_group delete)
//   - room_id            → NO ACTION / RESTRICT (primary room deletion blocked)
//
// Catalog code: 'n' = SET NULL, 'c' = CASCADE, 'r' = RESTRICT, 'a' = NO ACTION.
func TestActivityInstanceFKOnDelete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

	cases := []struct {
		refTable string
		refCol   string
		wantCode string
	}{
		{"activities.groups", "activity_group_id", "n"},
		{"schedule.calendar_periods", "calendar_period_id", "n"},
		{"active.groups", "active_group_id", "n"},
		{"facilities.rooms", "room_id", "r"},
	}

	for _, tc := range cases {
		t.Run(tc.refTable+"/"+tc.refCol, func(t *testing.T) {
			var confdeltype string
			err := db.NewRaw(`
				SELECT c.confdeltype
				FROM pg_constraint c
				JOIN pg_class t         ON t.oid = c.conrelid
				JOIN pg_namespace tns   ON tns.oid = t.relnamespace
				JOIN pg_class rt        ON rt.oid = c.confrelid
				JOIN pg_namespace rns   ON rns.oid = rt.relnamespace
				JOIN pg_attribute a     ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
				WHERE c.contype = 'f'
				  AND tns.nspname || '.' || t.relname = 'schedule.activity_instances'
				  AND rns.nspname || '.' || rt.relname = ?
				  AND a.attname = ?
				LIMIT 1
			`, tc.refTable, tc.refCol).Scan(ctx, &confdeltype)
			require.NoError(t, err, "FK for %s must exist", tc.refCol)
			assert.Equal(t, tc.wantCode, confdeltype,
				"activity_instances.%s FK to %s must have confdeltype=%q (got %q)",
				tc.refCol, tc.refTable, tc.wantCode, confdeltype)
		})
	}
}
