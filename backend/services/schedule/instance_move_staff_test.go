// Hermetic tests for the atomic staff move between blocks (#1884):
// MoveStaffBetweenBlocks relocates (or pool-assigns) one instance_staff row in
// one save, syncs live supervision, reconciles the target acknowledgement, and
// appends a single staff_moved Änderungsprotokoll entry.
package schedule_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// moveSetup bundles the per-test fixtures: one tenant, one room, two staff,
// two same-day blocks (source 14:00–15:00, target 14:30–15:30, overlapping).
type moveSetup struct {
	db       *bun.DB
	ctx      context.Context
	tenantID int64
	factory  *services.Factory
	roomID   int64
	staffID  int64 // assigned to source in makeMoveSetup
	otherID  int64 // free second staff member
	source   *scheduleModels.ActivityInstance
	target   *scheduleModels.ActivityInstance
	rowIDs   []int64
}

// createMoveInstance inserts one activity instance for the setup's tenant.
func createMoveInstance(t *testing.T, s *moveSetup, title, startHHMM, endHHMM, status string) *scheduleModels.ActivityInstance {
	t.Helper()
	row := &scheduleModels.ActivityInstance{
		Date:      moveTestDate(),
		Title:     title,
		StartTime: parseMoveClock(t, startHHMM),
		EndTime:   parseMoveClock(t, endHHMM),
		RoomID:    s.roomID,
		Status:    status,
	}
	row.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	return row
}

func createMoveStaffRow(t *testing.T, s *moveSetup, instanceID, staffID int64, mutate func(*scheduleModels.InstanceStaff)) *scheduleModels.InstanceStaff {
	t.Helper()
	row := &scheduleModels.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
	if mutate != nil {
		mutate(row)
	}
	row.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)
	s.rowIDs = append(s.rowIDs, row.ID)
	return row
}

func moveTestDate() timezone.Date {
	return timezone.TodayDate().AddDays(7)
}

func parseMoveClock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+hhmm)
	require.NoError(t, err)
	return parsed
}

func makeMoveSetup(t *testing.T) *moveSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	suffix := time.Now().UnixNano()

	s := &moveSetup{
		db:       db,
		ctx:      testpkg.TenantContext(tenantID),
		tenantID: tenantID,
		factory:  serviceFactory,
	}
	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("Move-Room-%d", suffix))
	s.roomID = room.ID
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Mona", fmt.Sprintf("Move-%d", suffix))
	other := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Otto", fmt.Sprintf("Other-%d", suffix))
	s.staffID = staff.ID
	s.otherID = other.ID

	s.source = createMoveInstance(t, s, "Schulhof", "14:00", "15:00", scheduleModels.InstanceStatusPlanned)
	s.target = createMoveInstance(t, s, "Mensa", "14:30", "15:30", scheduleModels.InstanceStatusPlanned)
	return s
}

func requireDeviationErr(t *testing.T, err error) *scheduleSvc.DeviationError {
	t.Helper()
	var de *scheduleSvc.DeviationError
	require.ErrorAs(t, err, &de)
	return de
}

func TestMoveStaffBetweenBlocks_RelocatesRowAndLogsEvent(t *testing.T) {
	t.Parallel()

	s := makeMoveSetup(t)
	roomOverride := s.roomID
	original := createMoveStaffRow(t, s, s.source.ID, s.staffID, func(row *scheduleModels.InstanceStaff) {
		row.IsPrimary = true
		row.RoomID = &roomOverride
	})

	result, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
		StaffID:          s.staffID,
		SourceInstanceID: &s.source.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleSvc.MoveStaffActionMoved, result.Action)
	require.NotNil(t, result.Source)
	assert.Equal(t, s.source.ID, result.Source.ID)

	assert.Empty(t, loadInstanceStaffRows(t, s.db, s.ctx, s.source.ID), "source lost the row")
	targetRows := loadInstanceStaffRows(t, s.db, s.ctx, s.target.ID)
	require.Len(t, targetRows, 1)
	moved := targetRows[0]
	assert.Equal(t, original.ID, moved.ID, "row identity is preserved (relocate, not delete+create)")
	assert.Equal(t, s.staffID, moved.StaffID)
	assert.False(t, moved.IsPrimary, "primary flag does not travel")
	assert.Nil(t, moved.RoomID, "source room split does not travel")
	assert.False(t, moved.IsSubstitute)
	assert.False(t, moved.IsAbsent)

	events := loadDeviationEvents(t, s.db, s.ctx, s.tenantID)
	require.Len(t, events, 1, "exactly one connected protocol entry")
	ev := events[0]
	assert.Equal(t, auditModels.DeviationEventStaffMoved, ev.EventType)
	require.NotNil(t, ev.InstanceID)
	assert.Equal(t, s.target.ID, *ev.InstanceID, "anchored on the target block")
	require.NotNil(t, ev.SubjectStaffID)
	assert.Equal(t, s.staffID, *ev.SubjectStaffID)
	assert.Nil(t, ev.ActorAccountID, "no acting account in this scenario")
	assert.Contains(t, string(ev.OldValue), fmt.Sprintf(`"from_instance_id": %d`, s.source.ID))
	assert.Contains(t, string(ev.OldValue), `"from_title": "Schulhof"`)
	assert.Contains(t, string(ev.NewValue), fmt.Sprintf(`"to_instance_id": %d`, s.target.ID))
	assert.Contains(t, string(ev.NewValue), `"to_title": "Mensa"`)

	// The person still overlaps nothing else that day — no advisory conflicts.
	assert.Empty(t, result.Warnings)
}

func TestMoveStaffBetweenBlocks_AssignFromPoolCreatesRow(t *testing.T) {
	t.Parallel()

	s := makeMoveSetup(t)

	result, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
		StaffID: s.otherID,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleSvc.MoveStaffActionAssigned, result.Action)
	assert.Nil(t, result.Source)

	targetRows := loadInstanceStaffRows(t, s.db, s.ctx, s.target.ID)
	require.Len(t, targetRows, 1)
	assert.Equal(t, s.otherID, targetRows[0].StaffID)
	assert.False(t, targetRows[0].IsSubstitute, "a pool assign is a planned assignment, not a Vertretung")

	events := loadDeviationEvents(t, s.db, s.ctx, s.tenantID)
	require.Len(t, events, 1)
	assert.Equal(t, auditModels.DeviationEventStaffMoved, events[0].EventType)
	assert.NotContains(t, string(events[0].OldValue), "from_instance_id", "no source block for a pool assign")
	assert.Contains(t, string(events[0].NewValue), fmt.Sprintf(`"to_instance_id": %d`, s.target.ID))
}

func TestMoveStaffBetweenBlocks_RetryIsIdempotent(t *testing.T) {
	t.Parallel()

	s := makeMoveSetup(t)
	// Already-moved state: on target, gone from source.
	createMoveStaffRow(t, s, s.target.ID, s.staffID, nil)

	result, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
		StaffID:          s.staffID,
		SourceInstanceID: &s.source.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleSvc.MoveStaffActionAlreadyApplied, result.Action)
	assert.Empty(t, loadDeviationEvents(t, s.db, s.ctx, s.tenantID), "a no-op retry logs nothing")
	require.Len(t, loadInstanceStaffRows(t, s.db, s.ctx, s.target.ID), 1)
}

func TestMoveStaffBetweenBlocks_ValidationFailures(t *testing.T) {
	t.Parallel()

	// One setup per subtest: each one puts the staff member on a block, and
	// schedule.instance_staff is unique on (instance, staff), so a shared
	// setup only worked while a per-row teardown ran between them (#2419).
	setup := func(t *testing.T) *moveSetup {
		t.Helper()
		s := makeMoveSetup(t)
		createMoveStaffRow(t, s, s.source.ID, s.staffID, nil)
		// Every case here is a rejected move, so none of them may log a
		// deviation event.
		t.Cleanup(func() {
			assert.Empty(t, loadDeviationEvents(t, s.db, s.ctx, s.tenantID))
		})
		return s
	}

	t.Run("not assigned to source", func(t *testing.T) {
		s := setup(t)
		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
			StaffID:          s.otherID,
			SourceInstanceID: &s.source.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusBadRequest, de.Status)
	})

	t.Run("on both blocks", func(t *testing.T) {
		s := setup(t)
		createMoveStaffRow(t, s, s.target.ID, s.staffID, nil)
		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
			StaffID:          s.staffID,
			SourceInstanceID: &s.source.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusConflict, de.Status)
		assert.Equal(t, "staff_already_on_target", de.Code)
	})

	t.Run("absent on target", func(t *testing.T) {
		s := setup(t)
		// Same state as the pool-assign path: the target row carries the
		// absence, so the conflict code must match (staff_absent_on_target,
		// not staff_already_on_target).
		createMoveStaffRow(t, s, s.target.ID, s.staffID, func(row *scheduleModels.InstanceStaff) {
			row.IsAbsent = true
		})
		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
			StaffID:          s.staffID,
			SourceInstanceID: &s.source.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusConflict, de.Status)
		assert.Equal(t, "staff_absent_on_target", de.Code)
	})

	t.Run("absent on source", func(t *testing.T) {
		s := setup(t)
		createMoveStaffRow(t, s, s.source.ID, s.otherID, func(row *scheduleModels.InstanceStaff) {
			row.IsAbsent = true
		})
		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
			StaffID:          s.otherID,
			SourceInstanceID: &s.source.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusBadRequest, de.Status)
	})

	t.Run("pool assign rejects day-wide absence from a terminal block", func(t *testing.T) {
		s := setup(t)
		terminal := createMoveInstance(t, s, "Abgeschlossene Historie", "09:00", "10:00", scheduleModels.InstanceStatusCompleted)
		createMoveStaffRow(t, s, terminal.ID, s.otherID, func(row *scheduleModels.InstanceStaff) {
			row.IsAbsent = true
		})

		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
			StaffID: s.otherID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusConflict, de.Status)
		assert.Equal(t, "staff_absent_on_date", de.Code)
		assert.Empty(t, loadInstanceStaffRows(t, s.db, s.ctx, s.target.ID))
	})

	t.Run("relocate rejects day-wide absence from another block", func(t *testing.T) {
		s := setup(t)
		third := createMoveInstance(t, s, "Dritter Block", "09:00", "10:00", scheduleModels.InstanceStatusPlanned)
		createMoveStaffRow(t, s, third.ID, s.staffID, func(row *scheduleModels.InstanceStaff) {
			row.IsAbsent = true
		})

		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
			StaffID:          s.staffID,
			SourceInstanceID: &s.source.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusConflict, de.Status)
		assert.Equal(t, "staff_absent_on_date", de.Code)
		assert.Empty(t, loadInstanceStaffRows(t, s.db, s.ctx, s.target.ID))
	})

	t.Run("cross-date move rejected", func(t *testing.T) {
		s := setup(t)
		otherDay := &scheduleModels.ActivityInstance{
			Date:      moveTestDate().AddDays(1),
			Title:     "Anderer Tag",
			StartTime: parseMoveClock(t, "14:00"),
			EndTime:   parseMoveClock(t, "15:00"),
			RoomID:    s.roomID,
			Status:    scheduleModels.InstanceStatusPlanned,
		}
		otherDay.SetTenantID(s.tenantID)
		_, err := s.db.NewInsert().Model(otherDay).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
		require.NoError(t, err)

		_, err = s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, otherDay.ID, scheduleSvc.MoveStaffInput{
			StaffID:          s.staffID,
			SourceInstanceID: &s.source.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusBadRequest, de.Status)
	})

	t.Run("source equals target", func(t *testing.T) {
		s := setup(t)
		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
			StaffID:          s.staffID,
			SourceInstanceID: &s.target.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusBadRequest, de.Status)
	})

	t.Run("unknown staff", func(t *testing.T) {
		s := setup(t)
		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
			StaffID:          999_999_999,
			SourceInstanceID: &s.source.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusNotFound, de.Status)
	})

	t.Run("cancelled target", func(t *testing.T) {
		s := setup(t)
		cancelled := createMoveInstance(t, s, "Abgesagt", "14:00", "15:00", scheduleModels.InstanceStatusCancelled)
		_, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, cancelled.ID, scheduleSvc.MoveStaffInput{
			StaffID:          s.staffID,
			SourceInstanceID: &s.source.ID,
		})
		de := requireDeviationErr(t, err)
		assert.Equal(t, http.StatusConflict, de.Status)
	})
}

func TestMoveStaffBetweenBlocks_TargetAckClearedSourceAckKept(t *testing.T) {
	t.Parallel()

	s := makeMoveSetup(t)
	createMoveStaffRow(t, s, s.source.ID, s.staffID, nil)
	// Both blocks acknowledged as deliberately unstaffed.
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("understaffed_ack = TRUE").
		Where(`"activity_instance".id IN (?, ?)`, s.source.ID, s.target.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
		StaffID:          s.staffID,
		SourceInstanceID: &s.source.ID,
	})
	require.NoError(t, err)

	var target, source scheduleModels.ActivityInstance
	require.NoError(t, s.db.NewSelect().Model(&target).ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).Where("id = ?", s.target.ID).Scan(s.ctx))
	require.NoError(t, s.db.NewSelect().Model(&source).ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).Where("id = ?", s.source.ID).Scan(s.ctx))
	assert.False(t, target.UnderstaffedAck, "a now-staffed target drops its stale acknowledgement")
	assert.True(t, source.UnderstaffedAck, "the source stays deliberately understaffed")
}

func TestMoveStaffBetweenBlocks_ActiveBlocksSyncSupervisionAndAllowRoundTrip(t *testing.T) {
	t.Parallel()

	s := makeMoveSetup(t)
	sourceGroup := testpkg.CreateTestActiveGroupForTenant(t, s.db, s.tenantID)
	targetGroup := testpkg.CreateTestActiveGroupForTenant(t, s.db, s.tenantID)
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", scheduleModels.InstanceStatusActive).
		Set("active_group_id = ?", sourceGroup.ID).
		Where(`"activity_instance".id = ?`, s.source.ID).
		Exec(s.ctx)
	require.NoError(t, err)
	_, err = s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", scheduleModels.InstanceStatusActive).
		Set("active_group_id = ?", targetGroup.ID).
		Where(`"activity_instance".id = ?`, s.target.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	createMoveStaffRow(t, s, s.source.ID, s.staffID, nil)
	sup := &activeModels.GroupSupervisor{
		StaffID:   s.staffID,
		GroupID:   sourceGroup.ID,
		Role:      "supervisor",
		StartDate: timezone.TodayDate(),
	}
	sup.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(sup).ModelTableExpr(`active.group_supervisors`).Exec(s.ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		var supIDs []int64
		_ = s.db.NewSelect().
			Model((*activeModels.GroupSupervisor)(nil)).
			ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
			Column("id").
			Where("tenant_id = ?", s.tenantID).
			Scan(context.Background(), &supIDs)
	})

	result, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
		StaffID:          s.staffID,
		SourceInstanceID: &s.source.ID,
	})
	require.NoError(t, err)
	assert.Len(t, result.ActiveTouched, 2, "both live groups need an SSE refetch")

	var endedSup activeModels.GroupSupervisor
	require.NoError(t, s.db.NewSelect().
		Model(&endedSup).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".id = ?`, sup.ID).
		Scan(s.ctx))
	assert.NotNil(t, endedSup.EndDate, "source supervision ended")

	var targetSups []*activeModels.GroupSupervisor
	require.NoError(t, s.db.NewSelect().
		Model(&targetSups).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".group_id = ?`, targetGroup.ID).
		Where(`"group_supervisor".staff_id = ?`, s.staffID).
		Scan(s.ctx))
	require.Len(t, targetSups, 1, "target supervision started")
	assert.Nil(t, targetSups[0].EndDate)

	// Moving back ends the target supervision and creates a new active source
	// row. The partial unique index applies only to end_date IS NULL, so the
	// historical source row must not cause a uniqueness failure.
	reverse, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.source.ID, scheduleSvc.MoveStaffInput{
		StaffID:          s.staffID,
		SourceInstanceID: &s.target.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleSvc.MoveStaffActionMoved, reverse.Action)
	assert.Len(t, reverse.ActiveTouched, 2)

	var sourceSups []*activeModels.GroupSupervisor
	require.NoError(t, s.db.NewSelect().
		Model(&sourceSups).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".group_id = ?`, sourceGroup.ID).
		Where(`"group_supervisor".staff_id = ?`, s.staffID).
		OrderExpr(`"group_supervisor".id ASC`).
		Scan(s.ctx))
	require.Len(t, sourceSups, 2)
	assert.NotNil(t, sourceSups[0].EndDate)
	assert.Nil(t, sourceSups[1].EndDate)

	targetSups = nil
	require.NoError(t, s.db.NewSelect().
		Model(&targetSups).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".group_id = ?`, targetGroup.ID).
		Where(`"group_supervisor".staff_id = ?`, s.staffID).
		Scan(s.ctx))
	require.Len(t, targetSups, 1)
	assert.NotNil(t, targetSups[0].EndDate)
}

// A person can already supervise the target's live group without holding an
// instance_staff row; the move must reuse that open supervision instead of
// tripping the end_date IS NULL partial unique index with a second Create.
func TestMoveStaffBetweenBlocks_ExistingOpenSupervisionIsReused(t *testing.T) {
	t.Parallel()

	s := makeMoveSetup(t)
	targetGroup := testpkg.CreateTestActiveGroupForTenant(t, s.db, s.tenantID)
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", scheduleModels.InstanceStatusActive).
		Set("active_group_id = ?", targetGroup.ID).
		Where(`"activity_instance".id = ?`, s.target.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	existing := &activeModels.GroupSupervisor{
		StaffID:   s.otherID,
		GroupID:   targetGroup.ID,
		Role:      "supervisor",
		StartDate: timezone.TodayDate(),
	}
	existing.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(existing).ModelTableExpr(`active.group_supervisors`).Exec(s.ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		var supIDs []int64
		_ = s.db.NewSelect().
			Model((*activeModels.GroupSupervisor)(nil)).
			ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
			Column("id").
			Where("tenant_id = ?", s.tenantID).
			Scan(context.Background(), &supIDs)
	})

	result, err := s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, s.target.ID, scheduleSvc.MoveStaffInput{
		StaffID: s.otherID,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleSvc.MoveStaffActionAssigned, result.Action)
	assert.Len(t, result.ActiveTouched, 1)

	var sups []*activeModels.GroupSupervisor
	require.NoError(t, s.db.NewSelect().
		Model(&sups).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".group_id = ?`, targetGroup.ID).
		Where(`"group_supervisor".staff_id = ?`, s.otherID).
		Scan(s.ctx))
	require.Len(t, sups, 1, "the open supervision is reused, not duplicated")
	assert.Nil(t, sups[0].EndDate)
}

// Guard: a DeviationError must always be extractable for the handler mapping.
func TestMoveStaffBetweenBlocks_PastDateRejected(t *testing.T) {
	t.Parallel()

	s := makeMoveSetup(t)
	past := &scheduleModels.ActivityInstance{
		Date:      timezone.TodayDate().AddDays(-1),
		Title:     "Gestern",
		StartTime: parseMoveClock(t, "14:00"),
		EndTime:   parseMoveClock(t, "15:00"),
		RoomID:    s.roomID,
		Status:    scheduleModels.InstanceStatusPlanned,
	}
	past.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(past).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.factory.Instance.MoveStaffBetweenBlocks(s.ctx, past.ID, scheduleSvc.MoveStaffInput{StaffID: s.staffID})
	var de *scheduleSvc.DeviationError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, http.StatusBadRequest, de.Status)
}
