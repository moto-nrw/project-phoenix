package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
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
		},
	}
}

func buildInstance(tenantID, roomID int64, activityID *int64, date timezone.Date, start, end time.Time, title string) *scheduleModels.ActivityInstance {
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	t.Run("creates template-backed instance", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "create-tpl")
		defer fx.cleanup()

		inst := buildInstance(
			testpkg.Tenant(t), fx.roomID, &fx.activityID,
			timezone.NewDate(2026, 9, 15),
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
			testpkg.Tenant(t), fx.roomID, nil,
			timezone.NewDate(2026, 9, 16),
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

		inst := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID,
			timezone.NewDate(2026, 9, 17),
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

		date := timezone.NewDate(2026, 9, 18)
		start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

		a := buildInstance(testpkg.Tenant(t), fx.roomID, nil, date, start, end, "Spontan A")
		a.IsSpontaneous = true
		b := buildInstance(testpkg.Tenant(t), fx.roomID, nil, date, start, end, "Spontan B")
		b.IsSpontaneous = true

		require.NoError(t, repo.Create(ctx, a))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", a.ID)
		require.NoError(t, repo.Create(ctx, b))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", b.ID)
	})

	t.Run("partial unique index rejects duplicate template slot", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "create-dup")
		defer fx.cleanup()

		date := timezone.NewDate(2026, 9, 19)
		start := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)

		a := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date, start, end, "Lernzeit A")
		require.NoError(t, repo.Create(ctx, a))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", a.ID)

		b := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date, start, end, "Lernzeit B")
		err := repo.Create(ctx, b)
		require.Error(t, err, "partial UNIQUE must block same (tenant,date,group,start)")
	})
}

func TestActivityInstanceRepository_CreateTemplateBackedIfAbsent_DuplicateDoesNotAbortTransaction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)
	fx := newActivityInstanceFixtures(t, db, "create-if-absent")
	defer fx.cleanup()

	date := timezone.NewDate(2026, 9, 21)
	start := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)
	var createdIDs []int64

	err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		first := buildInstance(0, fx.roomID, &fx.activityID, date, start, end, "Lernzeit A")
		inserted, err := repo.CreateTemplateBackedIfAbsent(ctx, first)
		require.NoError(t, err)
		require.True(t, inserted)
		require.Greater(t, first.ID, int64(0), "inserted instance must keep its generated ID for materialization children")

		duplicate := buildInstance(0, fx.roomID, &fx.activityID, date, start, end, "Lernzeit B")
		inserted, err = repo.CreateTemplateBackedIfAbsent(ctx, duplicate)
		require.NoError(t, err)
		require.False(t, inserted)

		rows, err := repo.FindByActivityGroupAndDate(ctx, fx.activityID, date)
		require.NoError(t, err, "duplicate DO NOTHING must leave the transaction usable")
		require.Len(t, rows, 1)
		createdIDs = append(createdIDs, rows[0].ID)
		return nil
	})
	require.NoError(t, err)
}

func TestActivityInstanceRepository_CreateTemplateBackedIfAbsent_ValidationBranches(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)
	fx := newActivityInstanceFixtures(t, db, "create-if-absent-invalid")
	defer fx.cleanup()

	date := timezone.NewDate(2026, 9, 22)
	start := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)

	t.Run("rejects nil", func(t *testing.T) {
		inserted, err := repo.CreateTemplateBackedIfAbsent(ctx, nil)
		require.Error(t, err)
		assert.False(t, inserted)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("rejects spontaneous row", func(t *testing.T) {
		inst := buildInstance(testpkg.Tenant(t), fx.roomID, nil, date, start, end, "Spontan")

		inserted, err := repo.CreateTemplateBackedIfAbsent(ctx, inst)
		require.Error(t, err)
		assert.False(t, inserted)
		assert.Contains(t, err.Error(), "activity_group_id is required")
	})

	t.Run("rejects model validation failure", func(t *testing.T) {
		inst := buildInstance(
			testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
			start, time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			"Broken",
		)

		inserted, err := repo.CreateTemplateBackedIfAbsent(ctx, inst)
		require.Error(t, err)
		assert.False(t, inserted)
		assert.Contains(t, err.Error(), "end_time must be after start_time")
	})

	t.Run("wraps database insert failure", func(t *testing.T) {
		missingRoomID := int64(987654321)
		inst := buildInstance(testpkg.Tenant(t), missingRoomID, &fx.activityID, date, start, end, "Missing Room")

		inserted, err := repo.CreateTemplateBackedIfAbsent(ctx, inst)
		require.Error(t, err)
		assert.False(t, inserted)
		assert.Contains(t, err.Error(), "create template-backed if absent")
	})
}

func TestActivityInstanceRepository_FindByID_and_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "crud")
	defer fx.cleanup()

	inst := buildInstance(
		testpkg.Tenant(t), fx.roomID, &fx.activityID,
		timezone.NewDate(2026, 9, 15),
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
		"Lernzeit",
	)
	require.NoError(t, repo.Create(ctx, inst))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "mark-completed")
	defer fx.cleanup()

	inst := buildInstance(
		testpkg.Tenant(t), fx.roomID, &fx.activityID,
		timezone.NewDate(2026, 9, 16),
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
		"Lernzeit Lifecycle",
	)
	inst.Status = scheduleModels.InstanceStatusActive
	require.NoError(t, repo.Create(ctx, inst))

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

func TestActivityInstanceRepository_CompleteActiveByActiveGroupIDsOmitsRecoveryState(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "session-end")
	defer fx.cleanup()

	group := testpkg.CreateTestActiveGroup(t, db, fx.activityID, fx.roomID)

	inst := buildInstance(
		testpkg.Tenant(t), fx.roomID, &fx.activityID,
		timezone.NewDate(2026, 9, 17),
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
		"Session-End",
	)
	inst.Status = scheduleModels.InstanceStatusActive
	inst.ActiveGroupID = &group.ID
	require.NoError(t, repo.Create(ctx, inst))

	completedAt := time.Date(2026, 9, 17, 16, 0, 0, 0, time.UTC)
	rows, err := repo.CompleteActiveByActiveGroupIDs(ctx, []int64{group.ID}, completedAt)
	require.NoError(t, err)
	assert.Equal(t, 1, int(rows))

	found, err := repo.FindByID(ctx, inst.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, scheduleModels.InstanceStatusCompleted, found.Status)
	require.NotNil(t, found.CompletedAt)
	assert.Nil(t, found.CompletedBy)
	assert.Nil(t, found.ReopenUntil)
	assert.False(t, scheduleSvc.CanReopenInstance(found, 42, true, completedAt.Add(time.Minute)))
}

func TestActivityInstanceRepository_FindByTenantAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "by-date")
	defer fx.cleanup()

	date := timezone.NewDate(2026, 9, 20)
	other := timezone.NewDate(2026, 9, 21)

	a := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), "A")
	b := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC), "B")
	c := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, other,
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), "C")

	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.Create(ctx, b))
	require.NoError(t, repo.Create(ctx, c))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "range")
	defer fx.cleanup()

	start := timezone.NewDate(2026, 9, 22)
	mid := timezone.NewDate(2026, 9, 24)
	end := timezone.NewDate(2026, 9, 26)
	outside := timezone.NewDate(2026, 10, 5)

	s1 := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, start,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), "start")
	s2 := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, mid,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), "mid")
	s3 := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, end,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), "end")
	s4 := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, outside,
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "grp-date")
	defer fx.cleanup()

	date := timezone.NewDate(2026, 10, 1)

	a := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), "morning")
	b := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC), "afternoon")

	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.Create(ctx, b))

	got, err := repo.FindByActivityGroupAndDate(ctx, fx.activityID, date)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(got), 2)
}

func TestActivityInstanceRepository_FindByActiveGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
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

		inst := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID,
			timezone.NewDate(2026, 10, 14),
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "bridge-uniq")
	defer fx.cleanup()

	ag := testpkg.CreateTestActiveGroup(t, db, fx.activityID, fx.roomID)

	date := timezone.NewDate(2026, 10, 13)

	first := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		"first")
	first.ActiveGroupID = &ag.ID
	require.NoError(t, repo.Create(ctx, first))

	second := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "list")
	defer fx.cleanup()

	inst := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID,
		timezone.NewDate(2026, 10, 7),
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
		"List-entry")
	require.NoError(t, repo.Create(ctx, inst))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

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

// #1840: re-plan / template split / template end route destructive deletes
// through DeletePlannedNonSpontaneousInWindow. A planned instance carrying a
// Vertretungsplan deviation (an acknowledged shortfall, or any instance_staff
// row marked absent / substitute / with an absence reason) is an explicit
// override of the base plan and must be PRESERVED; only clean planned instances
// are deleted and re-materialized.
func TestActivityInstanceRepository_DeletePlannedNonSpontaneousInWindow_PreservesDeviations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "dev-preserve")
	defer fx.cleanup()

	date := timezone.NewDate(2026, 9, 21)

	// plain: no deviation → deleted.
	plain := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC), "Plain")
	require.NoError(t, repo.Create(ctx, plain))

	// legacyManual: migration 1.15.303 reclassified legacy planned spontaneous
	// rows as non-spontaneous, but they remain unlinked to a template and must
	// not be removed by a whole-grid re-plan.
	legacyManual := buildInstance(testpkg.Tenant(t), fx.roomID, nil, date,
		time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC), "LegacyManual")
	require.NoError(t, repo.Create(ctx, legacyManual))

	// absentDev: carries an absent instance_staff row → preserved.
	absentDev := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC), "AbsentDev")
	require.NoError(t, repo.Create(ctx, absentDev))
	staff := testpkg.CreateTestStaff(t, db, "DevP", fmt.Sprintf("%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, db, absentDev.ID, staff.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	// ackDev: understaffed_ack=true → preserved.
	ackDev := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC), "AckDev")
	require.NoError(t, repo.Create(ctx, ackDev))
	_, err := db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances`).
		Set("understaffed_ack = ?", true).
		Where("id = ?", ackDev.ID).
		Exec(ctx)
	require.NoError(t, err)

	to := date
	// preserveDeviations=true (re-plan semantics): deviated instances survive.
	deleted, err := repo.DeletePlannedNonSpontaneousInWindow(ctx, date, &to, nil, true)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted, "only the template-backed non-deviated instance is deleted")

	_, err = repo.FindByID(ctx, plain.ID)
	assert.True(t, modelBase.IsNoRows(err), "plain instance must be deleted")

	gotLegacyManual, err := repo.FindByID(ctx, legacyManual.ID)
	require.NoError(t, err, "legacy manual instance must be preserved")
	assert.Equal(t, legacyManual.ID, gotLegacyManual.ID)

	gotAbsent, err := repo.FindByID(ctx, absentDev.ID)
	require.NoError(t, err, "instance with an absent staff row must be preserved")
	assert.Equal(t, absentDev.ID, gotAbsent.ID)

	gotAck, err := repo.FindByID(ctx, ackDev.ID)
	require.NoError(t, err, "acknowledged-shortfall instance must be preserved")
	assert.True(t, gotAck.UnderstaffedAck)
}

// #1840: the template split/end series operation passes preserveDeviations=false
// and must HARD-DELETE deviated rows too — otherwise "end this and all
// following" leaves the series partly alive, and a split leaves a duplicate old
// block next to the materialized successor.
func TestActivityInstanceRepository_DeletePlannedNonSpontaneousInWindow_HardDeleteIgnoresDeviations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "dev-harddelete")
	defer fx.cleanup()

	date := timezone.NewDate(2026, 9, 28)

	// A deviated instance: acknowledged shortfall AND an absent staff row.
	dev := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC), "DevHard")
	require.NoError(t, repo.Create(ctx, dev))
	staff := testpkg.CreateTestStaff(t, db, "DevH", fmt.Sprintf("%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, db, dev.ID, staff.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	_, err := db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances`).
		Set("understaffed_ack = ?", true).
		Where("id = ?", dev.ID).
		Exec(ctx)
	require.NoError(t, err)

	to := date
	// preserveDeviations=false (split/end semantics): deviated rows are deleted.
	deleted, err := repo.DeletePlannedNonSpontaneousInWindow(ctx, date, &to, nil, false)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted, "the deviated instance is hard-deleted")

	_, err = repo.FindByID(ctx, dev.ID)
	assert.True(t, modelBase.IsNoRows(err),
		"deviated instance must be deleted by the destructive series operation")
}

func TestActivityInstanceRepository_DeletePlannedMaterializedWeekendInstances(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db, func() time.Time {
		return timezone.NewDate(2026, 8, 24).BerlinMidnight()
	})
	legacyWeekendRepo, ok := any(repo).(interface {
		DeletePlannedMaterializedWeekendInstances(context.Context, int64, []int) (int64, error)
	})
	require.True(t, ok, "activity-instance repository must support legacy weekend cleanup")
	fx := newActivityInstanceFixtures(t, db, "legacy-weekend-delete")
	defer fx.cleanup()

	saturday := timezone.NewDate(2026, 8, 24).AddDays(1)
	for saturday.Weekday() != time.Saturday {
		saturday = saturday.AddDays(1)
	}
	period := testpkg.CreateTestCalendarPeriod(t, db, fmt.Sprintf("Legacy weekend %d", time.Now().UnixNano()), saturday, saturday.AddDays(1))

	materialized := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, saturday,
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC), "legacy saturday")
	materialized.CalendarPeriodID = &period.ID
	require.NoError(t, repo.Create(ctx, materialized))

	manual := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, saturday,
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC), "manual saturday")
	require.NoError(t, repo.Create(ctx, manual))

	sunday := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, saturday.AddDays(1),
		time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC), "legacy sunday")
	sunday.CalendarPeriodID = &period.ID
	require.NoError(t, repo.Create(ctx, sunday))

	deleted, err := legacyWeekendRepo.DeletePlannedMaterializedWeekendInstances(ctx, fx.activityID, []int{6})
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)

	_, err = repo.FindByID(ctx, materialized.ID)
	assert.True(t, modelBase.IsNoRows(err), "the removed Saturday slot must be deleted")
	_, err = repo.FindByID(ctx, manual.ID)
	require.NoError(t, err, "manual weekend instances must remain")
	_, err = repo.FindByID(ctx, sunday.ID)
	require.NoError(t, err, "weekend days retained by the template must remain")

	deleted, err = legacyWeekendRepo.DeletePlannedMaterializedWeekendInstances(ctx, fx.activityID, nil)
	require.NoError(t, err)
	assert.Zero(t, deleted, "an empty weekday set must be a no-op")
}

// #1565 review: editing a series' Listenart must carry onto its already
// materialized FUTURE occurrences that still hold the old value, while leaving
// today/past rows, non-planned/spontaneous rows, and per-occurrence overrides
// untouched — otherwise the classified daily lists stay empty until a re-plan.
func TestActivityInstanceRepository_PropagateListKindToFutureInstances(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "listkind-prop")
	defer fx.cleanup()
	// A second template proves the rewrite is scoped to one series.
	other := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Other-listkind-%d", time.Now().UnixNano()))
	otherID := other.ID

	today := timezone.TodayDate()

	// mkInstance builds + persists an instance and registers cleanup. Each row
	// gets a distinct start hour so same-(template,date) rows do not collide on
	// the (tenant, date, activity_group_id, start_time) unique index.
	startHour := 8
	mkInstance := func(name string, activityID *int64, date timezone.Date, listKind *string, mutate func(*scheduleModels.ActivityInstance)) *scheduleModels.ActivityInstance {
		start := time.Date(2024, 1, 1, startHour, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 1, startHour+1, 0, 0, 0, time.UTC)
		startHour++
		inst := buildInstance(testpkg.Tenant(t), fx.roomID, activityID, date, start, end, name)
		inst.ListKind = listKind
		if mutate != nil {
			mutate(inst)
		}
		require.NoError(t, repo.Create(ctx, inst))
		return inst
	}

	future := today.AddDays(7)
	// futureNull: future occurrence still carrying the pre-edit NULL → rewritten.
	futureNull := mkInstance("FutureNull", &fx.activityID, future, nil, nil)
	// futureNull2: a second future date, proves the update is not single-row.
	futureNull2 := mkInstance("FutureNull2", &fx.activityID, today.AddDays(14), nil, nil)
	// todayRow: dated today (== after) → never rewritten (a printed list stands).
	todayRow := mkInstance("Today", &fx.activityID, today, nil, nil)
	// pastRow: already elapsed → never rewritten.
	pastRow := mkInstance("Past", &fx.activityID, today.AddDays(-7), nil, nil)
	// overridden: future, but individually re-classified → override preserved.
	overridden := mkInstance("Overridden", &fx.activityID, future, testpkg.StrPtr("mensa"), nil)
	// cancelledRow: future but not planned → skipped by the status predicate.
	cancelledRow := mkInstance("Cancelled", &fx.activityID, future, nil, func(i *scheduleModels.ActivityInstance) {
		i.Status = scheduleModels.InstanceStatusCancelled
	})
	// spontaneousRow: future planned but spontaneous (no template) → skipped.
	spontaneousRow := mkInstance("Spontaneous", nil, future, nil, func(i *scheduleModels.ActivityInstance) {
		i.IsSpontaneous = true
	})
	// otherSeries: future planned NULL but a different template → not this series.
	otherSeries := mkInstance("OtherSeries", &otherID, future, nil, nil)

	// Series edit: NULL → "learning_time".
	newKind := testpkg.StrPtr("learning_time")
	changed, err := repo.PropagateListKindToFutureInstances(ctx, fx.activityID, nil, newKind, today)
	require.NoError(t, err)
	assert.EqualValues(t, 2, changed, "only the two untouched future planned rows are rewritten")

	assertKind := func(id int64, want *string, msg string) {
		got, err := repo.FindByID(ctx, id)
		require.NoError(t, err)
		if want == nil {
			assert.Nil(t, got.ListKind, msg)
			return
		}
		require.NotNil(t, got.ListKind, msg)
		assert.Equal(t, *want, *got.ListKind, msg)
	}

	assertKind(futureNull.ID, newKind, "future NULL row must adopt the new series kind")
	assertKind(futureNull2.ID, newKind, "second future NULL row must adopt the new series kind")
	assertKind(todayRow.ID, nil, "today's row must be left untouched")
	assertKind(pastRow.ID, nil, "past row must be left untouched")
	assertKind(overridden.ID, testpkg.StrPtr("mensa"), "per-occurrence override must be preserved")
	assertKind(cancelledRow.ID, nil, "cancelled row must be skipped")
	assertKind(spontaneousRow.ID, nil, "spontaneous row must be skipped")
	assertKind(otherSeries.ID, nil, "another template's occurrence must not change")
}

func TestActivityInstanceRepository_FindByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)

	t.Run("empty input short-circuits without hitting the DB", func(t *testing.T) {
		instances, err := repo.FindByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, instances)
	})

	t.Run("returns matching instances ordered by date then start time", func(t *testing.T) {
		fx := newActivityInstanceFixtures(t, db, "findbyids")
		defer fx.cleanup()

		later := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID,
			timezone.NewDate(2026, 9, 20),
			time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			"Later")
		require.NoError(t, repo.Create(ctx, later))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", later.ID)

		earlier := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID,
			timezone.NewDate(2026, 9, 18),
			time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
			"Earlier")
		require.NoError(t, repo.Create(ctx, earlier))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", earlier.ID)

		instances, err := repo.FindByIDs(ctx, []int64{later.ID, earlier.ID, 9_999_999})
		require.NoError(t, err)
		require.Len(t, instances, 2, "the non-existent id is silently absent")
		assert.Equal(t, earlier.ID, instances[0].ID, "ordered by date ascending")
		assert.Equal(t, later.ID, instances[1].ID)
	})
}
