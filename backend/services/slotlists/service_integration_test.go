// Hermetic integration tests for the slot list service (issue #1565).
//
// Covers the three data modes over selected timetable slots:
//
//   - planned:        only schedule.instance_students rows
//   - actual:         visits that overlap the slot time window
//   - reconciliation: merge with planned/present/missing/unplanned counters
//
// plus the pickup-based Ganztag cohort split and a PDF render smoke test.
package slotlists_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	facilitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/slotlists"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// listDate is the date slot-based lists (Plan/Ist/Abgleich) are built for in
// this suite. It is a fixed, unique PAST Wednesday (Go weekday 3): a slot
// reconciliation is refused for a future date (ErrReconciliationFutureDate), so
// the far-future date this suite used before made every reconciliation test
// fail its own guard, and a far-past unique date keeps this suite's
// tenant-scoped FindByTenantAndDate reads isolated from any other suite seeding
// data for "today". The Wednesday weekday matches the hardcoded pickup weekdays
// in the care-day tests below.
var listDate = timezone.NewDate(2020, 9, 2)

// pickupNow pins the slot list service clock (injected via Dependencies.Now)
// so the pickup suite is deterministic regardless of the weekday CI runs on. A
// Ganztag pickup schedule only exists for ISO weekdays 1–5 and the
// effective-pickup service short-circuits weekends, so a wall-clock "today"
// made the whole pickup suite fail every Saturday and Sunday: the model
// rejected the weekend weekday, and no pickup time resolved. It is a fixed
// Wednesday (Go weekday 3 == ISO 3), after listDate so slot reconciliations
// still treat those 2020 occurrences as already started.
var pickupNow = time.Date(2025, 9, 3, 12, 0, 0, 0, timezone.Berlin)

// pickupDate is the calendar day of pickupNow — "today" from the injected
// clock's point of view. A pickup cohort is refused for a past date
// (ErrPickupCohortPastDate) and a reconciliation for a future date, so only
// today satisfies both a pickup list and its reconciliation together. Pickup
// weekdays are derived from it dynamically.
var pickupDate = timezone.DateFromTime(pickupNow)

// atOn returns the given Berlin wall-clock instant on calendar date d, so a
// seeded visit/attendance lands on the same day as the list being built.
func atOn(d timezone.Date, hour, minute int) time.Time {
	return time.Date(d.Year, d.Month, d.Day, hour, minute, 0, 0, timezone.Berlin)
}

func newTestService(db *bun.DB) slotlists.Service {
	return newTestServiceWithSettings(db, nil)
}

type stubSlotListSettings map[string]string

func (s stubSlotListSettings) ResolveString(_ context.Context, key string) (string, error) {
	if key == configModels.KeyStudentDataScope {
		return configModels.StudentDataScopeAllStaff, nil
	}
	return s[key], nil
}

func (s stubSlotListSettings) HasTenantOverride(_ context.Context, key string) (bool, error) {
	if key == configModels.KeyStudentDataScope {
		return true, nil
	}
	_, ok := s[key]
	return ok, nil
}

type failingSlotListSettings struct{}

func (failingSlotListSettings) ResolveString(context.Context, string) (string, error) {
	return "", errors.New("settings unavailable")
}

func (failingSlotListSettings) HasTenantOverride(context.Context, string) (bool, error) {
	return false, nil
}

type groupSupervisorOnlySlotListSettings struct{}

func (groupSupervisorOnlySlotListSettings) ResolveString(_ context.Context, key string) (string, error) {
	if key == configModels.KeyStudentDataScope {
		return configModels.StudentDataScopeGroupSupervisorsOnly, nil
	}
	return "", nil
}

func (groupSupervisorOnlySlotListSettings) HasTenantOverride(_ context.Context, key string) (bool, error) {
	return key == configModels.KeyStudentDataScope, nil
}

type slotListUserContext struct {
	currentStaff *userModels.Staff
	groups       []*educationModels.Group
}

func (u slotListUserContext) GetCurrentStaff(context.Context) (*userModels.Staff, error) {
	return u.currentStaff, nil
}

func (u slotListUserContext) GetMyGroups(context.Context) ([]*educationModels.Group, error) {
	return u.groups, nil
}

type failingRoomRepo struct {
	err error
}

func (r failingRoomRepo) FindByID(context.Context, any) (*facilitiesModels.Room, error) {
	return nil, r.err
}

func newTestServiceWithSettings(db *bun.DB, settings stubSlotListSettings) slotlists.Service {
	return newTestServiceWithSettingsReader(db, settings)
}

func newTestServiceWithSettingsReader(db *bun.DB, settings interface {
	ResolveString(context.Context, string) (string, error)
	HasTenantOverride(context.Context, string) (bool, error)
}) slotlists.Service {
	return newTestServiceWithCustomRoomRepo(db, facilitiesRepo.NewRoomRepository(db), settings)
}

func newTestServiceWithCustomRoomRepo(db *bun.DB, roomRepo interface {
	FindByID(context.Context, any) (*facilitiesModels.Room, error)
}, settings interface {
	HasTenantOverride(context.Context, string) (bool, error)
	ResolveString(context.Context, string) (string, error)
}) slotlists.Service {
	return newTestServiceWithCustomAccess(db, roomRepo, settings, slotListUserContext{currentStaff: &userModels.Staff{}})
}

func newTestServiceWithCustomAccess(db *bun.DB, roomRepo interface {
	FindByID(context.Context, any) (*facilitiesModels.Room, error)
}, settings interface {
	HasTenantOverride(context.Context, string) (bool, error)
	ResolveString(context.Context, string) (string, error)
}, userCtx slotListUserContext) slotlists.Service {
	return slotlists.NewService(slotlists.Dependencies{
		InstanceRepo:        scheduleRepo.NewActivityInstanceRepository(db),
		InstanceStudentRepo: scheduleRepo.NewInstanceStudentRepository(db),
		VisitRepo:           activeRepo.NewVisitRepository(db),
		AttendanceRepo:      activeRepo.NewAttendanceRepository(db),
		StatusDayRepo:       activeRepo.NewStudentStatusDayRepository(db),
		CareDayService: scheduleSvc.NewCareDayService(scheduleSvc.CareDayDependencies{
			ArrivalSchedules:  scheduleRepo.NewStudentArrivalScheduleRepository(db),
			ArrivalExceptions: scheduleRepo.NewStudentArrivalExceptionRepository(db),
			PickupSchedules:   scheduleRepo.NewStudentPickupScheduleRepository(db),
			PickupExceptions:  scheduleRepo.NewStudentPickupExceptionRepository(db),
		}),
		PickupScheduleRepo: scheduleRepo.NewStudentPickupScheduleRepository(db),
		StudentRepo:        usersRepo.NewStudentRepository(db),
		PersonRepo:         usersRepo.NewPersonRepository(db),
		EducationGroupRepo: educationRepo.NewGroupRepository(db),
		RoomRepo:           roomRepo,
		PickupService: scheduleSvc.NewPickupScheduleService(
			scheduleRepo.NewStudentPickupScheduleRepository(db),
			scheduleRepo.NewStudentPickupExceptionRepository(db),
			scheduleRepo.NewStudentPickupNoteRepository(db),
			db,
		),
		ArrivalService: scheduleSvc.NewArrivalScheduleService(
			scheduleRepo.NewStudentArrivalScheduleRepository(db),
			scheduleRepo.NewStudentArrivalExceptionRepository(db),
			scheduleRepo.NewStudentArrivalNoteRepository(db),
			usersRepo.NewStudentRepository(db),
			usersRepo.NewPersonRepository(db),
			db,
			nil,
		),
		ListExport:  listexport.NewService(),
		Settings:    settings,
		UserContext: userCtx,
		// Pin the clock to a fixed Wednesday so the pickup suite does not depend
		// on the weekday CI runs on (see pickupNow).
		Now: func() time.Time { return pickupNow },
	})
}

// buildMensaFixture seeds one active Mensa instance bridged to an active
// group with three students: planned+present, planned-only, unplanned visitor.
type mensaFixture struct {
	db         *bun.DB
	svc        slotlists.Service
	instanceID int64
	plannedID  int64 // planned + slot-overlapping visit → present
	missingID  int64 // planned, no visit → missing
	walkInID   int64 // slot-overlapping visit only → unplanned
}

func buildMensaFixture(t *testing.T) *mensaFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-Act-%d", suffix))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	listKind := activitiesModels.ListKindMensa

	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa %d", suffix),
		StartTime:       time.Date(1, 1, 1, 11, 30, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive,
		ActiveGroupID:   &activeGroup.ID,
		ListKind:        &listKind,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	planned := testpkg.CreateTestStudent(t, db, "SL-Planned", fmt.Sprintf("P-%d", suffix), "3a")
	missing := testpkg.CreateTestStudent(t, db, "SL-Missing", fmt.Sprintf("M-%d", suffix), "3a")
	walkIn := testpkg.CreateTestStudent(t, db, "SL-WalkIn", fmt.Sprintf("W-%d", suffix), "3b")

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	var instanceStudentIDs []int64
	for _, studentID := range []int64{planned.ID, missing.ID} {
		row := &scheduleModels.InstanceStudent{
			InstanceID: instance.ID,
			StudentID:  studentID,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(1)
		require.NoError(t, isRepo.Create(ctx, row))
		instanceStudentIDs = append(instanceStudentIDs, row.ID)
	}

	entry := atOn(listDate, 11, 35)
	exitA := atOn(listDate, 12, 15)
	exitB := atOn(listDate, 12, 20)
	visitA := testpkg.CreateTestVisit(t, db, planned.ID, activeGroup.ID, entry, &exitA)
	visitB := testpkg.CreateTestVisit(t, db, walkIn.ID, activeGroup.ID, entry, &exitB)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "active.visits", visitA.ID, visitB.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", instanceStudentIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, planned.ID)
		testpkg.CleanupActivityFixtures(t, db, missing.ID)
		testpkg.CleanupActivityFixtures(t, db, walkIn.ID)
		testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	return &mensaFixture{
		db:         db,
		svc:        newTestService(db),
		instanceID: instance.ID,
		plannedID:  planned.ID,
		missingID:  missing.ID,
		walkInID:   walkIn.ID,
	}
}

func rowByStudent(rows []slotlists.Row, studentID int64) *slotlists.Row {
	for i := range rows {
		if rows[i].StudentID == studentID {
			return &rows[i]
		}
	}
	return nil
}

func TestBuildList_MensaReconciliation(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	result, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.Counters.Planned)
	assert.Equal(t, 2, result.Counters.Present)
	assert.Equal(t, 1, result.Counters.Missing)
	assert.Equal(t, 1, result.Counters.Unplanned)
	require.Len(t, result.Rows, 3)
	require.Len(t, result.Slots, 1)

	plannedRow := rowByStudent(result.Rows, f.plannedID)
	require.NotNil(t, plannedRow)
	assert.Equal(t, "Anwesend", plannedRow.StatusLabel)
	assert.True(t, plannedRow.Planned)
	assert.True(t, plannedRow.Present, "closed historical visit overlapping the slot must count as present")

	missingRow := rowByStudent(result.Rows, f.missingID)
	require.NotNil(t, missingRow)
	assert.Equal(t, "Fehlt", missingRow.StatusLabel)

	walkInRow := rowByStudent(result.Rows, f.walkInID)
	require.NotNil(t, walkInRow)
	assert.Equal(t, "Ungeplant anwesend", walkInRow.StatusLabel)
	assert.True(t, walkInRow.Unplanned)
}

// TestBuildList_VisitOutsideNominalWindowCountsPresent proves that a visit
// recorded by an occurrence run outside its planned StartTime/EndTime still
// counts as attendance. InstanceService.Start permits starting an instance late
// (or starting and completing it early) regardless of the nominal times and
// stamps the bridged active.group with the real start instant; the bridge is
// 1:1, so every visit under it is this occurrence's own attendance. A
// nominal-time overlap filter would drop the visit here, omitting a present
// child from Ist and printing an attended child as "Fehlt" in the Abgleich
// (#1565 review pass 2). The instance window is 11:30–13:00 but the visit runs
// 13:30–14:00 — wholly after the planned end, as for a late start.
func TestBuildList_VisitOutsideNominalWindowCountsPresent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-LateRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-LateAct-%d", suffix))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	listKind := activitiesModels.ListKindMensa

	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa spät %d", suffix),
		StartTime:       time.Date(1, 1, 1, 11, 30, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive,
		ActiveGroupID:   &activeGroup.ID,
		ListKind:        &listKind,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	attended := testpkg.CreateTestStudent(t, db, "SL-LateAttended", fmt.Sprintf("LA-%d", suffix), "3a")
	absent := testpkg.CreateTestStudent(t, db, "SL-LateAbsent", fmt.Sprintf("LB-%d", suffix), "3a")

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	var instanceStudentIDs []int64
	for _, studentID := range []int64{attended.ID, absent.ID} {
		row := &scheduleModels.InstanceStudent{
			InstanceID: instance.ID,
			StudentID:  studentID,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(1)
		require.NoError(t, isRepo.Create(ctx, row))
		instanceStudentIDs = append(instanceStudentIDs, row.ID)
	}

	// Entry 13:30, exit 14:00 — the whole visit falls AFTER the nominal
	// 11:30–13:00 window, exactly what a late Start produces.
	entry := atOn(listDate, 13, 30)
	exit := atOn(listDate, 14, 0)
	visit := testpkg.CreateTestVisit(t, db, attended.ID, activeGroup.ID, entry, &exit)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", instanceStudentIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, attended.ID)
		testpkg.CleanupActivityFixtures(t, db, absent.ID)
		testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	attendedRow := rowByStudent(result.Rows, attended.ID)
	require.NotNil(t, attendedRow, "a child attending a late-started slot must stay on the list")
	assert.True(t, attendedRow.Present,
		"a visit recorded outside the planned window must still count as present")
	assert.Equal(t, "Anwesend", attendedRow.StatusLabel)

	absentRow := rowByStudent(result.Rows, absent.ID)
	require.NotNil(t, absentRow)
	assert.False(t, absentRow.Present)
	assert.Equal(t, "Fehlt", absentRow.StatusLabel)
}

// TestBuildList_CancelledFromActiveRetainsAttendance proves that a slot
// cancelled AFTER it ran keeps the visits recorded during its brief run as
// present rows, while its now-void plan produces no "Fehlt". InstanceService.Cancel
// on an already-active instance ends the active group but deliberately preserves
// ActiveGroupID and its historical visits, so dropping the slot wholesale would
// erase documented attendance from Ist and Abgleich (#1565 review).
func TestBuildList_CancelledFromActiveRetainsAttendance(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	// Flip the instance to cancelled while preserving its ActiveGroupID and
	// visits — exactly the state Cancel leaves behind for a slot that was active
	// when it was called off.
	_, err := f.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", scheduleModels.InstanceStatusCancelled).
		Where(`"activity_instance".id = ?`, f.instanceID).
		Where("tenant_id = ?", 1).
		Exec(ctx)
	require.NoError(t, err)

	result, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	// planned + walkIn each had a slot-overlapping visit → retained as present
	// evidence. The void plan is dropped, so both read as unplanned presence.
	plannedRow := rowByStudent(result.Rows, f.plannedID)
	require.NotNil(t, plannedRow, "a child present during the run before cancellation stays on the list")
	assert.True(t, plannedRow.Present)
	assert.False(t, plannedRow.Planned, "the void plan is dropped; presence is unplanned evidence")
	assert.True(t, plannedRow.Unplanned)

	walkInRow := rowByStudent(result.Rows, f.walkInID)
	require.NotNil(t, walkInRow, "a walk-in present during the run stays on the list")
	assert.True(t, walkInRow.Present)

	// missing had no visit → the void plan must NOT print it as "Fehlt".
	assert.Nil(t, rowByStudent(result.Rows, f.missingID),
		"a planned no-show on a cancelled slot must not print as Fehlt")

	assert.Equal(t, 2, result.Counters.Present)
	assert.Equal(t, 0, result.Counters.Missing, "a cancelled slot produces no missing rows")
	assert.Equal(t, 0, result.Counters.Planned, "a cancelled slot's void plan yields no planned rows")
}

func TestBuildList_ListKindRestrictsSlots(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, f.db, fmt.Sprintf("SL-Other-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, f.db, fmt.Sprintf("SL-Other-Act-%d", suffix))
	listKind := activitiesModels.ListKindLearningTime
	otherInstance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Lernzeit %d", suffix),
		StartTime:       time.Date(1, 1, 1, 13, 30, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 14, 30, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusPlanned,
		ListKind:        &listKind,
	}
	otherInstance.SetTenantID(1)
	_, err := f.db.NewInsert().Model(otherInstance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	student := testpkg.CreateTestStudent(t, f.db, "SL-Other", fmt.Sprintf("O-%d", suffix), "3c")
	row := &scheduleModels.InstanceStudent{
		InstanceID: otherInstance.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(1)
	require.NoError(t, scheduleRepo.NewInstanceStudentRepository(f.db).Create(ctx, row))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, f.db, "schedule.instance_students", row.ID)
		testpkg.CleanupTableRecords(t, f.db, "schedule.activity_instances", otherInstance.ID)
		testpkg.CleanupActivityFixtures(t, f.db, student.ID)
		testpkg.CleanupActivityFixtures(t, f.db, 0, 0, 0, activity.ID, room.ID)
	})

	result, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:     listDate,
		Target:   slotlists.TargetSlots,
		ListKind: slotlists.ListKindMensa,
		Source:   slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	require.Len(t, result.Slots, 1)
	assert.Equal(t, f.instanceID, result.Slots[0].InstanceID)
	require.Len(t, result.Rows, 2)
	assert.NotNil(t, rowByStudent(result.Rows, f.plannedID))
	assert.NotNil(t, rowByStudent(result.Rows, f.missingID))
	assert.Nil(t, rowByStudent(result.Rows, student.ID))

	options, err := f.svc.ListOptions(ctx, listDate)
	require.NoError(t, err)
	require.Len(t, options.ListKinds, 4)
	byKind := map[slotlists.ListKind]slotlists.ListKindOption{}
	for _, option := range options.ListKinds {
		byKind[option.Kind] = option
	}
	assert.Equal(t, 1, byKind[slotlists.ListKindMensa].SlotCount)
	assert.Equal(t, 2, byKind[slotlists.ListKindMensa].RowCount)
	assert.Equal(t, 1, byKind[slotlists.ListKindLearningTime].SlotCount)
	assert.Equal(t, 1, byKind[slotlists.ListKindLearningTime].RowCount)
}

func TestBuildList_GroupSupervisorScopeFiltersUnsupervisedStudentRows(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	allowedGroup := testpkg.CreateTestEducationGroup(t, f.db, "SL-Allowed")
	deniedGroup := testpkg.CreateTestEducationGroup(t, f.db, "SL-Denied")
	t.Cleanup(func() {
		_, err := f.db.NewUpdate().
			TableExpr(`users.students`).
			Set(`group_id = NULL`).
			Where(`id IN (?)`, bun.List([]int64{f.plannedID, f.missingID, f.walkInID})).
			Exec(ctx)
		require.NoError(t, err)
		testpkg.CleanupTableRecords(t, f.db, "education.groups", allowedGroup.ID, deniedGroup.ID)
	})

	for studentID, groupID := range map[int64]int64{
		f.plannedID: allowedGroup.ID,
		f.missingID: deniedGroup.ID,
		f.walkInID:  deniedGroup.ID,
	} {
		_, err := f.db.NewUpdate().
			TableExpr(`users.students`).
			Set(`group_id = ?`, groupID).
			Where(`id = ?`, studentID).
			Exec(ctx)
		require.NoError(t, err)
	}

	svc := newTestServiceWithCustomAccess(
		f.db,
		facilitiesRepo.NewRoomRepository(f.db),
		groupSupervisorOnlySlotListSettings{},
		slotListUserContext{
			currentStaff: &userModels.Staff{},
			groups:       []*educationModels.Group{allowedGroup},
		},
	)

	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	require.Len(t, result.Rows, 1)
	assert.Equal(t, f.plannedID, result.Rows[0].StudentID)
	assert.Equal(t, 1, result.Counters.Planned)
	assert.Equal(t, 1, result.Counters.Present)
	assert.Equal(t, 0, result.Counters.Missing)
	assert.Equal(t, 0, result.Counters.Unplanned)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, allowedGroup.ID, result.Groups[0].ID)
}

func TestBuildList_SlotFilter(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	full, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	require.Len(t, full.Slots, 1)
	instanceID := full.Slots[0].InstanceID

	// Selecting the only slot returns the same rows/counters.
	kept, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:        listDate,
		Target:      slotlists.TargetSlots,
		Source:      slotlists.SourceReconciliation,
		InstanceIDs: []int64{instanceID},
	})
	require.NoError(t, err)
	assert.Len(t, kept.Rows, 3)
	assert.Equal(t, full.Counters, kept.Counters)

	// Deselecting it (filter to a different id) empties rows/counters, but the
	// slot stays listed so the UI can offer re-selection.
	dropped, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:        listDate,
		Target:      slotlists.TargetSlots,
		Source:      slotlists.SourceReconciliation,
		InstanceIDs: []int64{instanceID + 99999},
	})
	require.NoError(t, err)
	assert.Empty(t, dropped.Rows)
	assert.Equal(t, 0, dropped.Counters.Planned)
	require.Len(t, dropped.Slots, 1, "filtered-out slot must still be listed for re-selection")

	cleared, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:           listDate,
		Target:         slotlists.TargetSlots,
		Source:         slotlists.SourceReconciliation,
		InstanceIDsSet: true,
		InstanceIDs:    []int64{},
	})
	require.NoError(t, err)
	assert.Empty(t, cleared.Rows, "explicit empty slot selection must not fall back to all slots")
	require.Len(t, cleared.Slots, 1, "available slots stay visible after clearing the selection")
}

func TestBuildList_ClassFilterAndOptions(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	full, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	// Available class options cover every child in the (unfiltered) list.
	assert.ElementsMatch(t, []string{"3a", "3b"}, full.Classes)

	// Filtering to class 3b keeps only the walk-in child (the only 3b student),
	// and the class options stay the full set for re-selection.
	filtered, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:    listDate,
		Target:  slotlists.TargetSlots,
		Source:  slotlists.SourceReconciliation,
		Classes: []string{"3b"},
	})
	require.NoError(t, err)
	require.Len(t, filtered.Rows, 1)
	assert.Equal(t, f.walkInID, filtered.Rows[0].StudentID)
	assert.ElementsMatch(t, []string{"3a", "3b"}, filtered.Classes)
}

func TestBuildList_MensaPlannedAndActualModes(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	planned, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, planned.Counters.Planned)
	require.Len(t, planned.Rows, 2)
	assert.Nil(t, rowByStudent(planned.Rows, f.walkInID), "walk-in must not appear in planned mode")

	actual, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceActual,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, actual.Counters.Present)
	require.Len(t, actual.Rows, 2)
	assert.Nil(t, rowByStudent(actual.Rows, f.missingID), "absent child must not appear in actual mode")
}

func TestBuildList_TimetablePresentStatusCountsAsActual(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	planned, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	require.Len(t, planned.Slots, 1)

	isRepo := scheduleRepo.NewInstanceStudentRepository(f.db)
	rows, err := isRepo.FindByInstanceID(ctx, planned.Slots[0].InstanceID)
	require.NoError(t, err)
	patchedRow := false
	for _, row := range rows {
		if row.StudentID == f.missingID {
			row.Status = scheduleModels.AttendanceStatusPresent
			require.NoError(t, isRepo.Update(ctx, row))
			patchedRow = true
		}
	}
	require.True(t, patchedRow, "fixture must contain the planned row that will be manually marked present")

	actual, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceActual,
	})
	require.NoError(t, err)
	markedPresent := rowByStudent(actual.Rows, f.missingID)
	require.NotNil(t, markedPresent, "timetable PATCH present status must appear in Ist without requiring an active visit")
	assert.True(t, markedPresent.Present)
	assert.Equal(t, "Anwesend", markedPresent.StatusLabel)
	assert.Equal(t, 3, actual.Counters.Present)

	recon, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	markedPresent = rowByStudent(recon.Rows, f.missingID)
	require.NotNil(t, markedPresent)
	assert.True(t, markedPresent.Present)
	assert.Equal(t, "Anwesend", markedPresent.StatusLabel)
	assert.Equal(t, 0, recon.Counters.Missing)
	assert.Equal(t, 3, recon.Counters.Present)
}

func TestBuildList_RoomLookupErrorsFail(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)
	roomErr := errors.New("room repository unavailable")
	// Settings is a required dependency (NewService panics on nil); this slot
	// list never reads the pickup cutoffs, so an empty stub is enough.
	svc := newTestServiceWithCustomRoomRepo(f.db, failingRoomRepo{err: roomErr}, stubSlotListSettings{})

	_, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, roomErr)
}

func TestBuildList_FullDayPickupCohorts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	early := testpkg.CreateTestStudent(t, db, "SL-Early", fmt.Sprintf("E-%d", suffix), "2a")
	late := testpkg.CreateTestStudent(t, db, "SL-Late", fmt.Sprintf("L-%d", suffix), "2a")
	staff := testpkg.CreateTestStaff(t, db, "SL-Staff", fmt.Sprintf("S-%d", suffix))

	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, row := range []*scheduleModels.StudentPickupSchedule{
		{StudentID: early.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: late.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupActivityFixtures(t, db, early.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, late.ID)
	})

	svc := newTestService(db)

	cohort1430, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.NotNil(t, rowByStudent(cohort1430.Rows, early.ID), "14:00 pickup belongs to the 14:30 cohort")
	assert.Nil(t, rowByStudent(cohort1430.Rows, late.ID), "16:00 pickup must not be in the 14:30 cohort")

	cohort1600, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.NotNil(t, rowByStudent(cohort1600.Rows, late.ID), "16:00 pickup belongs to the 16:00 cohort")
	assert.Nil(t, rowByStudent(cohort1600.Rows, early.ID), "14:00 pickup must not be in the 16:00 cohort")

	earlyRow := rowByStudent(cohort1430.Rows, early.ID)
	require.NotNil(t, earlyRow)
	assert.Equal(t, "14:00", earlyRow.PickupTime)
}

func TestBuildList_FullDayPickupCohortsUseTenantCutoffs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	shortDay := testpkg.CreateTestStudent(t, db, "SL-CfgShort", fmt.Sprintf("CSH-%d", suffix), "2a")
	longDay := testpkg.CreateTestStudent(t, db, "SL-CfgLong", fmt.Sprintf("CLO-%d", suffix), "2a")
	tooLate := testpkg.CreateTestStudent(t, db, "SL-CfgLate", fmt.Sprintf("CLA-%d", suffix), "2a")
	staff := testpkg.CreateTestStaff(t, db, "SL-CfgStaff", fmt.Sprintf("CST-%d", suffix))

	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, row := range []*scheduleModels.StudentPickupSchedule{
		{StudentID: shortDay.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 14, 45, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: longDay.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 15, 30, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: tooLate.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 17, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupActivityFixtures(t, db, shortDay.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, longDay.ID)
		testpkg.CleanupActivityFixtures(t, db, tooLate.ID)
	})

	svc := newTestServiceWithSettings(db, stubSlotListSettings{
		configModels.KeySlotListShortDayCutoff: "15:00",
		configModels.KeySlotListLongDayCutoff:  "16:30",
	})

	shortCohort, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.Equal(t, "Ganztag bis 15:00", shortCohort.ListLabel)
	assert.NotNil(t, rowByStudent(shortCohort.Rows, shortDay.ID), "14:45 belongs to the configured short-day cohort")
	assert.Nil(t, rowByStudent(shortCohort.Rows, longDay.ID), "15:30 must not belong to the short-day cohort")

	longCohort, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.Equal(t, "Ganztag bis 16:30", longCohort.ListLabel)
	assert.NotNil(t, rowByStudent(longCohort.Rows, longDay.ID), "15:30 belongs to the configured long-day cohort")
	assert.Nil(t, rowByStudent(longCohort.Rows, shortDay.ID), "short-day child must not also appear in the long-day cohort")
	assert.Nil(t, rowByStudent(longCohort.Rows, tooLate.ID), "17:00 is beyond the configured long-day cutoff")
}

func TestBuildList_FullDayPickupCutoffsNormalizeBeforeComparison(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	svc := newTestServiceWithSettings(db, stubSlotListSettings{
		configModels.KeySlotListShortDayCutoff: "9:00",
		configModels.KeySlotListLongDayCutoff:  "16:00",
	})

	shortCohort, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.Equal(t, "Ganztag bis 09:00", shortCohort.ListLabel)

	longCohort, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.Equal(t, "Ganztag bis 16:00", longCohort.ListLabel)
}

func TestBuildList_FullDayPickupSettingsErrorsFail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	svc := newTestServiceWithSettingsReader(db, failingSlotListSettings{})

	_, err := svc.BuildList(testpkg.TenantContext(1), slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourcePlanned,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve short-day pickup cutoff")
}

// TestBuildList_GroupByClass verifies grouping stamps a section heading on
// every row and rejects a grouping that doesn't fit the target.
func TestBuildList_GroupByClass(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	grouped, err := f.svc.BuildList(ctx, slotlists.Params{
		Date:    listDate,
		Target:  slotlists.TargetSlots,
		Source:  slotlists.SourceReconciliation,
		GroupBy: slotlists.GroupByClass,
	})
	require.NoError(t, err)
	assert.Equal(t, slotlists.GroupByClass, grouped.GroupBy)
	require.NotEmpty(t, grouped.Rows)
	for _, row := range grouped.Rows {
		assert.NotEmpty(t, row.GroupTitle, "every grouped row needs a section heading")
		assert.Contains(t, row.GroupTitle, "Klasse: ")
	}
	// Rows stay contiguous per section (sorted by GroupTitle).
	planned3a := rowByStudent(grouped.Rows, f.plannedID)
	require.NotNil(t, planned3a)
	assert.Equal(t, "Klasse: 3a", planned3a.GroupTitle)

	// slot grouping is invalid for a pickup-based list type.
	_, err = f.svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourcePlanned,
		GroupBy:      slotlists.GroupBySlot,
	})
	require.Error(t, err, "slot grouping must be rejected for pickup-based lists")
}

// TestBuildList_FullDayActualScopedToCohort guards the pickup-list "Ist" mode:
// a present child outside the cohort must NOT appear in actual mode (it has no
// physical room — only the merge view surfaces it as unplanned).
func TestBuildList_FullDayActualScopedToCohort(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	early := testpkg.CreateTestStudent(t, db, "SL-CohEarly", fmt.Sprintf("CE-%d", suffix), "1a") // 14:00 → 14:30 cohort
	late := testpkg.CreateTestStudent(t, db, "SL-CohLate", fmt.Sprintf("CL-%d", suffix), "1a")   // 16:00 → not in cohort
	staff := testpkg.CreateTestStaff(t, db, "SL-CohStaff", fmt.Sprintf("CS-%d", suffix))
	device := testpkg.CreateTestDevice(t, db, fmt.Sprintf("sl-coh-%d", suffix))

	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, p := range []*scheduleModels.StudentPickupSchedule{
		{StudentID: early.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: late.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		p.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, p))
		pickupIDs = append(pickupIDs, p.ID)
	}

	// Both children were physically present on pickupDate. Closed attendance
	// must still count for list generation.
	checkIn := atOn(pickupDate, 8, 0)
	checkOut := atOn(pickupDate, 15, 30)
	var attendanceIDs []int64
	for _, sid := range []int64{early.ID, late.ID} {
		att := &activeModels.Attendance{
			StudentID:    sid,
			Date:         pickupDate,
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			CheckedInBy:  staff.ID,
			DeviceID:     device.ID,
		}
		att.SetTenantID(1)
		_, err := db.NewInsert().Model(att).ModelTableExpr(`active.attendance`).Exec(ctx)
		require.NoError(t, err)
		attendanceIDs = append(attendanceIDs, att.ID)
	}

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "active.attendance", attendanceIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupActivityFixtures(t, db, early.ID, staff.ID, device.ID)
		testpkg.CleanupActivityFixtures(t, db, late.ID)
	})

	svc := newTestService(db)

	// Ist (actual): only the cohort child, even though both are present.
	actual, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourceActual,
	})
	require.NoError(t, err)
	assert.NotNil(t, rowByStudent(actual.Rows, early.ID), "cohort child present → in Ist")
	assert.Nil(t, rowByStudent(actual.Rows, late.ID), "non-cohort present child must not pollute the pickup Ist list")

	// Abgleich (reconciliation): the merge still surfaces the late child as unplanned.
	recon, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	lateRow := rowByStudent(recon.Rows, late.ID)
	require.NotNil(t, lateRow, "reconciliation must still surface the non-cohort present child")
	assert.True(t, lateRow.Unplanned)
}

// TestBuildList_ExcusedAbsence proves only an explicit registered sign-off
// (instance_students status=absent plus substatus) is reported as a distinct
// "Abgemeldet" status. Lifecycle no-shows (status=absent without substatus)
// stay in the unexplained "Fehlt" counter.
func TestBuildList_ExcusedAbsence(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-ExRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-ExAct-%d", suffix))

	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa %d", suffix),
		StartTime:       time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	expected := testpkg.CreateTestStudent(t, db, "SL-Expected", fmt.Sprintf("EX-%d", suffix), "5a")
	excused := testpkg.CreateTestStudent(t, db, "SL-Excused", fmt.Sprintf("AB-%d", suffix), "5a")
	noShow := testpkg.CreateTestStudent(t, db, "SL-NoShow", fmt.Sprintf("NS-%d", suffix), "5a")

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	var isIDs []int64
	excusedSubstatus := scheduleModels.AttendanceSubstatusExcused
	for _, sc := range []struct {
		studentID int64
		status    string
		substatus *string
	}{
		{studentID: expected.ID, status: scheduleModels.AttendanceStatusExpected},
		{studentID: excused.ID, status: scheduleModels.AttendanceStatusAbsent, substatus: &excusedSubstatus},
		{studentID: noShow.ID, status: scheduleModels.AttendanceStatusAbsent},
	} {
		row := &scheduleModels.InstanceStudent{
			InstanceID: instance.ID,
			StudentID:  sc.studentID,
			Status:     sc.status,
			Substatus:  sc.substatus,
		}
		row.SetTenantID(1)
		require.NoError(t, isRepo.Create(ctx, row))
		isIDs = append(isIDs, row.ID)
	}

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", isIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, expected.ID)
		testpkg.CleanupActivityFixtures(t, db, excused.ID)
		testpkg.CleanupActivityFixtures(t, db, noShow.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.Counters.Missing, "expected and no-show absences count as Fehlt")
	assert.Equal(t, 1, result.Counters.Excused, "only the explicit sign-off counts as Abgemeldet")

	excusedRow := rowByStudent(result.Rows, excused.ID)
	require.NotNil(t, excusedRow)
	assert.True(t, excusedRow.Excused)
	assert.Equal(t, "Abgemeldet (entschuldigt)", excusedRow.StatusLabel)

	missingRow := rowByStudent(result.Rows, expected.ID)
	require.NotNil(t, missingRow)
	assert.False(t, missingRow.Excused)
	assert.Equal(t, "Fehlt", missingRow.StatusLabel)

	noShowRow := rowByStudent(result.Rows, noShow.ID)
	require.NotNil(t, noShowRow)
	assert.False(t, noShowRow.Excused)
	assert.Equal(t, "Fehlt", noShowRow.StatusLabel)
}

// TestBuildList_CancelledCareDayCompletedNoShowStaysAbgemeldet guards the
// timetable-merge path against losing a sign-off once the row leaves the
// expected state. Completing a block stamps the cancelled child absent WITHOUT
// a substatus (BulkUpdateStatus only flips status), so the finalized row is
// indistinguishable from a genuine no-show — the cancellation survives only in
// the care-day verdict. The merge must consult that verdict, not the row's
// current status, or the no-show is mislabelled as unexplained "Fehlt" (#1565
// review).
func TestBuildList_CancelledCareDayCompletedNoShowStaysAbgemeldet(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-CxlRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-CxlAct-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "SL-Staff", fmt.Sprintf("SCC-%d", suffix))

	// A completed instance: the block already ran and ended.
	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa %d", suffix),
		StartTime:       time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusCompleted,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	child := testpkg.CreateTestStudent(t, db, "SL-Cxl", fmt.Sprintf("CC-%d", suffix), "3b")

	// Completion stamped the cancelled child absent with no substatus, exactly
	// how BulkUpdateStatus leaves it — the row carries no sign-off evidence.
	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	row := &scheduleModels.InstanceStudent{
		InstanceID: instance.ID,
		StudentID:  child.ID,
		Status:     scheduleModels.AttendanceStatusAbsent,
	}
	row.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, row))

	// A timeless "Kommt heute nicht" pickup exception on the list date is the
	// only thing that makes the care-day verdict CareDayCancelled.
	exc := &scheduleModels.StudentPickupException{
		StudentID: child.ID, ExceptionDate: listDate, PickupTime: nil, CreatedBy: staff.ID,
	}
	exc.SetTenantID(1)
	_, err = db.NewInsert().Model(exc).ModelTableExpr(`schedule.student_pickup_exceptions`).Exec(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_exceptions", exc.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, child.ID, staff.ID, activity.ID, room.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	childRow := rowByStudent(result.Rows, child.ID)
	require.NotNil(t, childRow, "a cancelled no-show stays on the reconciliation list")
	assert.True(t, childRow.Excused, "a cancelled care day is a registered sign-off")
	assert.Equal(t, "Abgemeldet (entschuldigt)", childRow.StatusLabel)
	assert.Equal(t, 1, result.Counters.Excused)
	assert.Equal(t, 0, result.Counters.Missing, "the cancellation must not count as unexplained Fehlt")
}

func TestRenderList_PDFSmoke(t *testing.T) {
	f := buildMensaFixture(t)
	ctx := testpkg.TenantContext(1)

	file, err := f.svc.RenderList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	}, listexport.FormatPDF)
	require.NoError(t, err)
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.NotEmpty(t, file.Data)
	assert.Contains(t, file.Filename, "tagesliste-abgleich-freie-angebotsauswahl")
}

// A registered sick/excused/class-trip day (active.student_status_days) must
// show as "Abgemeldet" in the Ganztag reconciliation, not as unexplained
// "Fehlt" — a pickup cohort has no instance_students row to carry the
// sign-off, so the day status is the only absence evidence (#1565 review).
func TestBuildList_PickupReconciliationMarksStatusDayAsExcused(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	sick := testpkg.CreateTestStudent(t, db, "SL-Sick", fmt.Sprintf("K-%d", suffix), "2b")
	missing := testpkg.CreateTestStudent(t, db, "SL-Missing", fmt.Sprintf("M-%d", suffix), "2b")
	staff := testpkg.CreateTestStaff(t, db, "SL-Staff", fmt.Sprintf("SD-%d", suffix))

	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, row := range []*scheduleModels.StudentPickupSchedule{
		{StudentID: sick.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: missing.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}
	statusDay := testpkg.CreateTestStudentStatusDay(t, db, sick.ID, pickupDate, activeModels.StudentStatusDaySick)
	t.Cleanup(func() {
		testpkg.CleanupStudentStatusDays(t, db, statusDay.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupActivityFixtures(t, db, sick.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, missing.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	sickRow := rowByStudent(result.Rows, sick.ID)
	require.NotNil(t, sickRow, "signed-off child stays in the cohort")
	assert.True(t, sickRow.Excused, "status day counts as a registered sign-off")
	assert.Equal(t, "Abgemeldet (entschuldigt)", sickRow.StatusLabel)

	missingRow := rowByStudent(result.Rows, missing.ID)
	require.NotNil(t, missingRow)
	assert.False(t, missingRow.Excused)
	assert.Equal(t, "Fehlt", missingRow.StatusLabel)

	assert.Equal(t, 1, result.Counters.Excused, "excused counter reflects the status day")
	assert.Equal(t, 1, result.Counters.Missing, "unexplained absence stays a missing child")
}

// A Ganztag Abgleich generated mid-morning must not report a child who has not
// yet reached their scheduled arrival time as "Fehlt": with no check-in and the
// arrival still ahead, the empty attendance row is not a no-show. This mirrors
// collectSlotEntries excluding a not-yet-started slot. A child whose arrival is
// already past on the same list stays a genuine missing child (#1565 review
// pass 1).
func TestBuildList_PickupReconciliationDefersNotYetArrivedChild(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	// pickupNow is pinned to 12:00 Berlin (see newTestServiceWithCustomAccess).
	notYetArrived := testpkg.CreateTestStudent(t, db, "SL-Later", fmt.Sprintf("A-%d", suffix), "2b")
	alreadyDue := testpkg.CreateTestStudent(t, db, "SL-Due", fmt.Sprintf("B-%d", suffix), "2b")
	staff := testpkg.CreateTestStaff(t, db, "SL-Staff", fmt.Sprintf("AR-%d", suffix))

	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, row := range []*scheduleModels.StudentPickupSchedule{
		{StudentID: notYetArrived.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: alreadyDue.ID, Weekday: weekday, PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}

	var arrivalIDs []int64
	for _, row := range []*scheduleModels.StudentArrivalSchedule{
		// 14:00 arrival is still ahead of the 12:00 clock → deferred.
		{StudentID: notYetArrived.ID, Weekday: weekday, ExpectedArrival: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
		// 11:00 arrival is already past → a genuine no-show.
		{StudentID: alreadyDue.ID, Weekday: weekday, ExpectedArrival: time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		row.SetTenantID(1)
		_, err := db.NewInsert().Model(row).ModelTableExpr(`schedule.student_arrival_schedules`).Exec(ctx)
		require.NoError(t, err)
		arrivalIDs = append(arrivalIDs, row.ID)
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_arrival_schedules", arrivalIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupActivityFixtures(t, db, notYetArrived.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, alreadyDue.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	assert.Nil(t, rowByStudent(result.Rows, notYetArrived.ID),
		"a child not yet expected must not print before their arrival time")

	dueRow := rowByStudent(result.Rows, alreadyDue.ID)
	require.NotNil(t, dueRow, "a child past their arrival time is a genuine no-show")
	assert.Equal(t, "Fehlt", dueRow.StatusLabel)

	assert.Equal(t, 1, result.Counters.Missing,
		"only the already-due child counts as missing")

	// The Plan view lists both children — it is the schedule, not attendance —
	// so the deferral is scoped to the Abgleich alone.
	plan, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.NotNil(t, rowByStudent(plan.Rows, notYetArrived.ID),
		"the not-yet-arrived child is still planned")
	assert.NotNil(t, rowByStudent(plan.Rows, alreadyDue.ID))
}

// A cancelled care day ("Kommt heute nicht") is a registered absence carried
// by a timeless arrival/pickup exception, not a status day. The Ganztag
// reconciliation must show such a child as "Abgemeldet", never as unexplained
// "Fehlt": a cleared pickup exception drops the effective time (fall back to
// the regular bucket), a cleared arrival exception leaves the regular pickup
// (mark it cancelled anyway) — both would otherwise lose the sign-off (#1565
// review).
func TestBuildList_PickupReconciliationMarksCancelledCareDayAsExcused(t *testing.T) {
	weekday := int(pickupDate.Weekday())

	cases := []struct {
		name      string
		clearWhat string // "pickup" or "arrival"
	}{
		{"cleared pickup exception (effective time gone)", "pickup"},
		{"cleared arrival exception (regular pickup survives)", "arrival"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testpkg.SetupTestDB(t)
			t.Cleanup(func() { _ = db.Close() })
			ctx := testpkg.TenantContext(1)
			suffix := time.Now().UnixNano()

			child := testpkg.CreateTestStudent(t, db, "SL-Cxl", fmt.Sprintf("C-%d", suffix), "2c")
			staff := testpkg.CreateTestStaff(t, db, "SL-Staff", fmt.Sprintf("SC-%d", suffix))

			pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
			sched := &scheduleModels.StudentPickupSchedule{
				StudentID: child.ID, Weekday: weekday,
				PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID,
			}
			sched.SetTenantID(1)
			require.NoError(t, pickupRepo.Create(ctx, sched))

			var exceptionTable string
			var exceptionID int64
			if tc.clearWhat == "pickup" {
				exc := &scheduleModels.StudentPickupException{
					StudentID: child.ID, ExceptionDate: pickupDate, PickupTime: nil, CreatedBy: staff.ID,
				}
				exc.SetTenantID(1)
				_, err := db.NewInsert().Model(exc).ModelTableExpr(`schedule.student_pickup_exceptions`).Exec(ctx)
				require.NoError(t, err)
				exceptionTable, exceptionID = "schedule.student_pickup_exceptions", exc.ID
			} else {
				exc := &scheduleModels.StudentArrivalException{
					StudentID: child.ID, ExceptionDate: pickupDate, ExpectedArrival: nil, CreatedBy: staff.ID,
				}
				exc.SetTenantID(1)
				_, err := db.NewInsert().Model(exc).ModelTableExpr(`schedule.student_arrival_exceptions`).Exec(ctx)
				require.NoError(t, err)
				exceptionTable, exceptionID = "schedule.student_arrival_exceptions", exc.ID
			}
			t.Cleanup(func() {
				testpkg.CleanupTableRecords(t, db, exceptionTable, exceptionID)
				testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", sched.ID)
				testpkg.CleanupActivityFixtures(t, db, child.ID, staff.ID)
			})

			svc := newTestService(db)
			result, err := svc.BuildList(ctx, slotlists.Params{
				Date:         pickupDate,
				Target:       slotlists.TargetPickupCohort,
				PickupCohort: slotlists.PickupCohortLongDay,
				Source:       slotlists.SourceReconciliation,
			})
			require.NoError(t, err)

			row := rowByStudent(result.Rows, child.ID)
			require.NotNil(t, row, "a cancelled child stays in the cohort via the regular pickup bucket")
			assert.True(t, row.Excused, "a cancelled care day is a registered sign-off")
			assert.Equal(t, "Abgemeldet (entschuldigt)", row.StatusLabel)
			assert.Equal(t, "16:00", row.PickupTime)
			assert.Equal(t, 1, result.Counters.Excused)
			assert.Equal(t, 0, result.Counters.Missing)
		})
	}
}

// A pending or inactive student keeps their recurring pickup schedule
// (lifecycle deactivation does not delete it). They must not be placed into a
// cohort as a planned / "Fehlt" child, and must not inflate options
// availability (#1565 review).
func TestBuildList_PickupCohortExcludesInactiveStudents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	active := testpkg.CreateTestStudent(t, db, "SL-Active", fmt.Sprintf("A-%d", suffix), "2d")
	gone := testpkg.CreateTestStudent(t, db, "SL-Gone", fmt.Sprintf("G-%d", suffix), "2d")
	staff := testpkg.CreateTestStaff(t, db, "SL-Staff", fmt.Sprintf("SI-%d", suffix))

	_, err := db.NewUpdate().TableExpr(`users.students`).
		Set(`status = ?`, string(userModels.StudentStatusInactive)).
		Where(`id = ?`, gone.ID).Exec(ctx)
	require.NoError(t, err)

	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, id := range []int64{active.ID, gone.ID} {
		row := &scheduleModels.StudentPickupSchedule{
			StudentID: id, Weekday: weekday,
			PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID,
		}
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupActivityFixtures(t, db, active.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, gone.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.NotNil(t, rowByStudent(result.Rows, active.ID), "active child is in the cohort")
	assert.Nil(t, rowByStudent(result.Rows, gone.ID), "inactive child must not appear")

	options, err := svc.ListOptions(ctx, pickupDate)
	require.NoError(t, err)
	byCohort := map[slotlists.PickupCohort]slotlists.PickupCohortOption{}
	for _, opt := range options.PickupCohorts {
		byCohort[opt.Cohort] = opt
	}
	assert.Equal(t, 1, byCohort[slotlists.PickupCohortLongDay].RowCount, "inactive child must not inflate availability")
}

// A Ganztag pickup plan cannot be reconstructed for a past date — the pickup
// schedule has no history — so BuildList refuses it (#1565 review). Slot-based
// lists stay available for the same past date.
func TestBuildList_PickupCohortRejectsPastDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	svc := newTestService(db)

	// Relative to the injected service clock (pickupDate == "today"), so the
	// past-date guards fire deterministically on any real weekday.
	pastDate := pickupDate.AddDays(-1)

	_, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pastDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourcePlanned,
	})
	require.ErrorIs(t, err, slotlists.ErrPickupCohortPastDate)

	// A slot list for the same past date is allowed (materialized instances and
	// attendance are historical).
	_, err = svc.BuildList(ctx, slotlists.Params{
		Date:   pastDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	// Options degrades gracefully rather than erroring: pickup cohorts report
	// unavailable for the past date.
	options, err := svc.ListOptions(ctx, pastDate)
	require.NoError(t, err)
	for _, opt := range options.PickupCohorts {
		assert.False(t, opt.Available, "pickup cohort must be unavailable for a past date")
		assert.Equal(t, 0, opt.RowCount)
	}
}

// Cohort membership must be decided from the enrollment interval relative to the
// requested (future) date, not the scheduler-maintained current status: a child
// whose enrollment ends before the date is excluded even while still "active"
// today, and a pending child whose enrollment has already started is included
// (#1565 review).
func TestBuildList_PickupCohortUsesEnrollmentInterval(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	// Enrollment intervals below are relative to pickupDate (today).
	ended := testpkg.CreateTestStudent(t, db, "SL-Ended", fmt.Sprintf("EN-%d", suffix), "2a")     // active today, enrollment already over
	started := testpkg.CreateTestStudent(t, db, "SL-Started", fmt.Sprintf("ST-%d", suffix), "2a") // pending today, already enrolled on pickupDate
	notYet := testpkg.CreateTestStudent(t, db, "SL-NotYet", fmt.Sprintf("NY-%d", suffix), "2a")   // pending, enrolled only after pickupDate
	staff := testpkg.CreateTestStaff(t, db, "SL-EnrStaff", fmt.Sprintf("ES-%d", suffix))

	setEnrollment := func(id int64, status userModels.StudentStatus, from, until *timezone.Date) {
		_, err := db.NewUpdate().TableExpr(`users.students`).
			Set(`status = ?`, string(status)).
			Set(`enrolled_from = ?`, from).
			Set(`enrolled_until = ?`, until).
			Where(`id = ?`, id).Exec(ctx)
		require.NoError(t, err)
	}
	endedUntil := pickupDate.AddDays(-1)   // day before pickupDate
	startedFrom := pickupDate.AddDays(-34) // before pickupDate
	notYetFrom := pickupDate.AddDays(88)   // after pickupDate
	setEnrollment(ended.ID, userModels.StudentStatusActive, nil, &endedUntil)
	setEnrollment(started.ID, userModels.StudentStatusPending, &startedFrom, nil)
	setEnrollment(notYet.ID, userModels.StudentStatusPending, &notYetFrom, nil)

	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, id := range []int64{ended.ID, started.ID, notYet.ID} {
		row := &scheduleModels.StudentPickupSchedule{
			StudentID: id, Weekday: weekday,
			PickupTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC), CreatedBy: staff.ID, // 14:00 → short cohort
		}
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupActivityFixtures(t, db, ended.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, started.ID)
		testpkg.CleanupActivityFixtures(t, db, notYet.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortShortDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	assert.Nil(t, rowByStudent(result.Rows, ended.ID), "enrollment ended before the date → excluded despite active status")
	assert.NotNil(t, rowByStudent(result.Rows, started.ID), "pending child already enrolled on the date → included")
	assert.Nil(t, rowByStudent(result.Rows, notYet.ID), "enrollment starts after the date → excluded")

	options, err := svc.ListOptions(ctx, pickupDate)
	require.NoError(t, err)
	byCohort := map[slotlists.PickupCohort]slotlists.PickupCohortOption{}
	for _, opt := range options.PickupCohorts {
		byCohort[opt.Cohort] = opt
	}
	assert.Equal(t, 1, byCohort[slotlists.PickupCohortShortDay].RowCount, "options availability matches the preview cohort")
}

// A cancelled care day whose same-day exception cleared the effective pickup
// time must still be counted in the options availability (via the regular
// weekly bucket), matching what the preview/export show (#1565 review).
func TestListOptions_CancelledCareDayCountedInCohort(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	child := testpkg.CreateTestStudent(t, db, "SL-CxlOpt", fmt.Sprintf("CO-%d", suffix), "2c")
	staff := testpkg.CreateTestStaff(t, db, "SL-CxlStaff", fmt.Sprintf("CS-%d", suffix))

	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	sched := &scheduleModels.StudentPickupSchedule{
		StudentID: child.ID, Weekday: weekday,
		PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID, // 16:00 → long cohort
	}
	sched.SetTenantID(1)
	require.NoError(t, pickupRepo.Create(ctx, sched))

	// A timeless pickup exception ("Kommt heute nicht") clears the effective
	// pickup time for pickupDate.
	exc := &scheduleModels.StudentPickupException{
		StudentID: child.ID, ExceptionDate: pickupDate, PickupTime: nil, CreatedBy: staff.ID,
	}
	exc.SetTenantID(1)
	_, err := db.NewInsert().Model(exc).ModelTableExpr(`schedule.student_pickup_exceptions`).Exec(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_exceptions", exc.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", sched.ID)
		testpkg.CleanupActivityFixtures(t, db, child.ID, staff.ID)
	})

	svc := newTestService(db)

	// Preview keeps the cancelled child (falls back to the 16:00 bucket).
	preview, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	require.NotNil(t, rowByStudent(preview.Rows, child.ID), "cancelled child stays in the cohort via the regular bucket")

	// Options must count the same child, not report zero/unavailable.
	options, err := svc.ListOptions(ctx, pickupDate)
	require.NoError(t, err)
	byCohort := map[slotlists.PickupCohort]slotlists.PickupCohortOption{}
	for _, opt := range options.PickupCohorts {
		byCohort[opt.Cohort] = opt
	}
	assert.Equal(t, 1, byCohort[slotlists.PickupCohortLongDay].RowCount, "cancelled child must be counted in options")
	assert.True(t, byCohort[slotlists.PickupCohortLongDay].Available)
}

// A planned instance_students row that survives a lifecycle inactivation (the
// scheduler flips users.students.status / lets enrolled_until pass but leaves
// the roster row) must be dropped from a slot list rather than reported as an
// unexplained "Fehlt". An enrolled planned-absent child still shows as Fehlt
// (#1565 review).
func TestBuildList_SlotListDropsPlannedRowForEndedEnrollment(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-EndRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-EndAct-%d", suffix))
	listKind := activitiesModels.ListKindMensa
	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa %d", suffix),
		StartTime:       time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive,
		ListKind:        &listKind,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	enrolled := testpkg.CreateTestStudent(t, db, "SL-StillHere", fmt.Sprintf("SH-%d", suffix), "5a")
	ended := testpkg.CreateTestStudent(t, db, "SL-Left", fmt.Sprintf("LF-%d", suffix), "5a")

	endedUntil := listDate.AddDays(-1) // day before listDate
	_, err = db.NewUpdate().TableExpr(`users.students`).
		Set(`status = ?`, string(userModels.StudentStatusInactive)).
		Set(`enrolled_until = ?`, endedUntil).
		Where(`id = ?`, ended.ID).Exec(ctx)
	require.NoError(t, err)

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	var isIDs []int64
	for _, id := range []int64{enrolled.ID, ended.ID} {
		row := &scheduleModels.InstanceStudent{
			InstanceID: instance.ID,
			StudentID:  id,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(1)
		require.NoError(t, isRepo.Create(ctx, row))
		isIDs = append(isIDs, row.ID)
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", isIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, enrolled.ID)
		testpkg.CleanupActivityFixtures(t, db, ended.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	assert.Nil(t, rowByStudent(result.Rows, ended.ID), "stale planned row for an ended enrollment must be dropped, not shown as Fehlt")
	enrolledRow := rowByStudent(result.Rows, enrolled.ID)
	require.NotNil(t, enrolledRow, "an enrolled planned-absent child is still surfaced")
	assert.Equal(t, "Fehlt", enrolledRow.StatusLabel)
	assert.Equal(t, 1, result.Counters.Missing, "only the enrolled child counts as Fehlt")
}

// TestBuildList_SlotListDropsPlannedRowForUnbookedCareDay covers the #1565
// review P1: a whole-group/year assignment lists every member as Expected on
// every occurrence, but the care plan only books some of them on this weekday.
// On a live (not-yet-ended) instance the not_scheduled marker is not written
// yet, so the raw flag cannot tell the two apart — the service must resolve the
// live care-day verdict and drop the unbooked child instead of reporting it as
// planned and, in the Abgleich, as "Fehlt".
func TestBuildList_SlotListDropsPlannedRowForUnbookedCareDay(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-UnbookedRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-UnbookedAct-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "SL-UnbookedStaff", fmt.Sprintf("US-%d", suffix))
	listKind := activitiesModels.ListKindMensa
	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa %d", suffix),
		StartTime:       time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive, // live: not_scheduled not stamped yet
		ListKind:        &listKind,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	booked := testpkg.CreateTestStudent(t, db, "SL-Booked", fmt.Sprintf("BK-%d", suffix), "5a")
	unbooked := testpkg.CreateTestStudent(t, db, "SL-Unbooked", fmt.Sprintf("UB-%d", suffix), "5a")

	// Both children have a care plan, so neither is "unknown" (which would stay
	// expected). booked is scheduled on listDate's weekday (Wednesday, ISO 3);
	// unbooked is only scheduled on Monday (ISO 1) → not_scheduled on Wednesday.
	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, row := range []*scheduleModels.StudentPickupSchedule{
		{StudentID: booked.ID, Weekday: 3, PickupTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: unbooked.ID, Weekday: 1, PickupTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	var isIDs []int64
	for _, id := range []int64{booked.ID, unbooked.ID} {
		row := &scheduleModels.InstanceStudent{
			InstanceID: instance.ID,
			StudentID:  id,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(1)
		require.NoError(t, isRepo.Create(ctx, row))
		isIDs = append(isIDs, row.ID)
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", isIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, booked.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, unbooked.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	assert.Nil(t, rowByStudent(result.Rows, unbooked.ID), "a child not booked into care on this weekday must not show as planned or Fehlt")
	bookedRow := rowByStudent(result.Rows, booked.ID)
	require.NotNil(t, bookedRow, "the booked child is still surfaced")
	assert.Equal(t, "Fehlt", bookedRow.StatusLabel)
	assert.Equal(t, 1, result.Counters.Missing, "only the booked child counts as Fehlt")
	assert.Equal(t, 1, result.Counters.Planned, "only the booked child counts as planned")

	// ListOptions' per-kind planned row count must exclude the unbooked child too.
	options, err := svc.ListOptions(ctx, listDate)
	require.NoError(t, err)
	var mensaOption *slotlists.ListKindOption
	for i := range options.ListKinds {
		if options.ListKinds[i].Kind == slotlists.ListKindMensa {
			mensaOption = &options.ListKinds[i]
		}
	}
	require.NotNil(t, mensaOption, "the Mensa list kind is available")
	assert.Equal(t, 1, mensaOption.RowCount, "options planned count must exclude the unbooked child")
}

// TestBuildList_SlotListUnbookedButPresentIsUnplanned covers the #1565 review
// P2: a child bulk-assigned to the slot but not booked into care on this weekday
// who checks in anyway (attendance sync flips the roster row to present) is
// unplanned presence ("Ungeplant anwesend"), never planned-and-present
// ("Anwesend"). AttendanceRowCareDay reports unknown for a present row, so the
// service must key on the raw not_scheduled care-day verdict — exactly as it does
// for a signed-off (cancelled) child who attends anyway.
func TestBuildList_SlotListUnbookedButPresentIsUnplanned(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-UpRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-UpAct-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "SL-UpStaff", fmt.Sprintf("UP-%d", suffix))
	listKind := activitiesModels.ListKindMensa
	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa %d", suffix),
		StartTime:       time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive, // live: not_scheduled not stamped yet
		ListKind:        &listKind,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	booked := testpkg.CreateTestStudent(t, db, "SL-UpBooked", fmt.Sprintf("UB-%d", suffix), "5a")
	unbooked := testpkg.CreateTestStudent(t, db, "SL-UpUnbooked", fmt.Sprintf("UU-%d", suffix), "5a")

	// booked is scheduled on listDate's weekday (Wednesday, ISO 3); unbooked is
	// only scheduled on Monday (ISO 1) → not_scheduled on Wednesday. Both have a
	// care plan, so neither is "unknown" (which would stay expected).
	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, row := range []*scheduleModels.StudentPickupSchedule{
		{StudentID: booked.ID, Weekday: 3, PickupTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: unbooked.ID, Weekday: 1, PickupTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	bookedRow := &scheduleModels.InstanceStudent{
		InstanceID: instance.ID,
		StudentID:  booked.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	bookedRow.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, bookedRow))
	// The unbooked child checked in: attendance sync flipped the roster row to
	// present (a timetable PATCH, no active-group visit required — see
	// TestBuildList_TimetablePresentStatusCountsAsActual).
	presentRow := &scheduleModels.InstanceStudent{
		InstanceID: instance.ID,
		StudentID:  unbooked.ID,
		Status:     scheduleModels.AttendanceStatusPresent,
	}
	presentRow.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, presentRow))

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", bookedRow.ID, presentRow.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, booked.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, unbooked.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	unbookedRow := rowByStudent(result.Rows, unbooked.ID)
	require.NotNil(t, unbookedRow, "an unbooked-but-present child stays on the reconciliation list")
	assert.True(t, unbookedRow.Present, "the child physically attended")
	assert.True(t, unbookedRow.Unplanned, "attendance on an unbooked day is unplanned presence")
	assert.False(t, unbookedRow.Planned, "a child the plan never booked is not planned-and-present")
	assert.Equal(t, "Ungeplant anwesend", unbookedRow.StatusLabel)

	bookedResult := rowByStudent(result.Rows, booked.ID)
	require.NotNil(t, bookedResult, "the booked child is still surfaced")
	assert.Equal(t, "Fehlt", bookedResult.StatusLabel)

	assert.Equal(t, 1, result.Counters.Unplanned, "only the unbooked walk-in counts as unplanned")
	assert.Equal(t, 1, result.Counters.Present, "the walk-in is present")
	assert.Equal(t, 1, result.Counters.Missing, "only the booked no-show counts as Fehlt")
}

// A broad sick/excused status day stamps every expected roster row of the day
// as absent — including days the care plan never booked the child into care. On
// such a not-scheduled day the row (status=absent, owned by the status day via
// student_status_day_id) must be dropped from the slot list, exactly like a
// still-expected unbooked row, not printed as a planned "Fehlt"/"Abgemeldet"
// child the plan never expected (#1565 review).
func TestBuildList_SlotListDropsStatusDayAbsenceOnUnbookedDay(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-SdRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-SdAct-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "SL-SdStaff", fmt.Sprintf("SDS-%d", suffix))
	listKind := activitiesModels.ListKindMensa
	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa %d", suffix),
		StartTime:       time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive, // live: not_scheduled not stamped yet
		ListKind:        &listKind,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	booked := testpkg.CreateTestStudent(t, db, "SL-SdBooked", fmt.Sprintf("SB-%d", suffix), "5a")
	sickUnbooked := testpkg.CreateTestStudent(t, db, "SL-SdSick", fmt.Sprintf("SS-%d", suffix), "5a")

	// booked is scheduled on listDate's weekday (Wednesday, ISO 3); sickUnbooked
	// is only scheduled on Monday (ISO 1) → not booked into care on Wednesday.
	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	var pickupIDs []int64
	for _, row := range []*scheduleModels.StudentPickupSchedule{
		{StudentID: booked.ID, Weekday: 3, PickupTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
		{StudentID: sickUnbooked.ID, Weekday: 1, PickupTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID},
	} {
		row.SetTenantID(1)
		require.NoError(t, pickupRepo.Create(ctx, row))
		pickupIDs = append(pickupIDs, row.ID)
	}

	// A broad sick day stamps sickUnbooked's expected row as absent and takes
	// ownership of it via student_status_day_id.
	statusDay := testpkg.CreateTestStudentStatusDay(t, db, sickUnbooked.ID, listDate, activeModels.StudentStatusDaySick)

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	bookedRow := &scheduleModels.InstanceStudent{
		InstanceID: instance.ID,
		StudentID:  booked.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	bookedRow.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, bookedRow))
	sickRow := &scheduleModels.InstanceStudent{
		InstanceID:         instance.ID,
		StudentID:          sickUnbooked.ID,
		Status:             scheduleModels.AttendanceStatusAbsent,
		StudentStatusDayID: &statusDay.ID,
	}
	sickRow.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, sickRow))

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", bookedRow.ID, sickRow.ID)
		testpkg.CleanupStudentStatusDays(t, db, statusDay.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", pickupIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, booked.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, sickUnbooked.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	assert.Nil(t, rowByStudent(result.Rows, sickUnbooked.ID),
		"a status-day absence on a day the child was never booked must be dropped, not shown")
	bookedResult := rowByStudent(result.Rows, booked.ID)
	require.NotNil(t, bookedResult, "the booked child is still surfaced")
	assert.Equal(t, "Fehlt", bookedResult.StatusLabel)
	assert.Equal(t, 1, result.Counters.Missing, "only the booked child counts as Fehlt")
	assert.Equal(t, 0, result.Counters.Excused, "the unbooked status-day row is not a sign-off on the list")
	assert.Equal(t, 1, result.Counters.Planned, "only the booked child counts as planned")

	options, err := svc.ListOptions(ctx, listDate)
	require.NoError(t, err)
	var mensaOption *slotlists.ListKindOption
	for i := range options.ListKinds {
		if options.ListKinds[i].Kind == slotlists.ListKindMensa {
			mensaOption = &options.ListKinds[i]
		}
	}
	require.NotNil(t, mensaOption, "the Mensa list kind is available")
	assert.Equal(t, 1, mensaOption.RowCount, "options planned count must exclude the unbooked status-day child")
}

// A signed-off ("Kommt heute nicht") child stays on a classified slot list as a
// planned "Abgemeldet" row, so ListOptions' per-kind planned count must include
// them too — otherwise the list-kind availability underreports what the
// preview/export render (#1565 review).
func TestListOptions_CancelledCareDayCountedInSlotList(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-CxRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-CxAct-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "SL-CxStaff", fmt.Sprintf("CXS-%d", suffix))
	listKind := activitiesModels.ListKindMensa
	instance := &scheduleModels.ActivityInstance{
		Date:            listDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa %d", suffix),
		StartTime:       time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive,
		ListKind:        &listKind,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	cancelled := testpkg.CreateTestStudent(t, db, "SL-CxChild", fmt.Sprintf("CX-%d", suffix), "5a")

	// Booked on listDate's Wednesday, then signed off for the day with a timeless
	// "Kommt heute nicht" pickup exception → a cancelled care day.
	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	sched := &scheduleModels.StudentPickupSchedule{
		StudentID: cancelled.ID, Weekday: 3,
		PickupTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID,
	}
	sched.SetTenantID(1)
	require.NoError(t, pickupRepo.Create(ctx, sched))
	exc := &scheduleModels.StudentPickupException{
		StudentID: cancelled.ID, ExceptionDate: listDate, PickupTime: nil, CreatedBy: staff.ID,
	}
	exc.SetTenantID(1)
	_, err = db.NewInsert().Model(exc).ModelTableExpr(`schedule.student_pickup_exceptions`).Exec(ctx)
	require.NoError(t, err)

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	row := &scheduleModels.InstanceStudent{
		InstanceID: instance.ID,
		StudentID:  cancelled.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, row))

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_exceptions", exc.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", sched.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, cancelled.ID, staff.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)

	// Preview keeps the signed-off child as a planned "Abgemeldet" row.
	preview, err := svc.BuildList(ctx, slotlists.Params{
		Date:   listDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	previewRow := rowByStudent(preview.Rows, cancelled.ID)
	require.NotNil(t, previewRow, "a signed-off child stays on the slot list")
	assert.True(t, previewRow.Excused)
	assert.Equal(t, "Abgemeldet (entschuldigt)", previewRow.StatusLabel)
	assert.Equal(t, 1, preview.Counters.Excused)

	// ListOptions must count the same child, matching the preview.
	options, err := svc.ListOptions(ctx, listDate)
	require.NoError(t, err)
	var mensaOption *slotlists.ListKindOption
	for i := range options.ListKinds {
		if options.ListKinds[i].Kind == slotlists.ListKindMensa {
			mensaOption = &options.ListKinds[i]
		}
	}
	require.NotNil(t, mensaOption, "the Mensa list kind is available")
	assert.Equal(t, 1, mensaOption.RowCount, "options must count the signed-off child, like preview/export")
}

// TestBuildList_SlotReconciliationExcludesNotYetStartedSlot proves the per-slot
// today guard (#1565 review): an Abgleich requested before a slot's scheduled
// start has no presence evidence yet, so its planned children must be excluded
// from the merge rather than reported as a group of missing ("Fehlt") children.
// A slot whose start has already passed still reconciles normally.
func TestBuildList_SlotReconciliationExcludesNotYetStartedSlot(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-NysRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-NysAct-%d", suffix))
	mensa := activitiesModels.ListKindMensa

	// pickupNow is 12:00; this occurrence starts at 15:00 → not yet started.
	future := &scheduleModels.ActivityInstance{
		Date:            pickupDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa-Future %d", suffix),
		StartTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusPlanned,
		ListKind:        &mensa,
	}
	future.SetTenantID(1)
	_, err := db.NewInsert().Model(future).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	planned := testpkg.CreateTestStudent(t, db, "SL-NysPlanned", fmt.Sprintf("NP-%d", suffix), "3a")

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	row := &scheduleModels.InstanceStudent{
		InstanceID: future.ID,
		StudentID:  planned.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, row))

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", future.ID)
		testpkg.CleanupActivityFixtures(t, db, planned.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)

	// Plan still lists the expected child (the plan is exactly what has not
	// happened yet).
	plan, err := svc.BuildList(ctx, slotlists.Params{
		Date:   pickupDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourcePlanned,
	})
	require.NoError(t, err)
	require.NotNil(t, rowByStudent(plan.Rows, planned.ID), "Plan must show the not-yet-started slot's expected child")

	// Reconciliation excludes the not-yet-started slot: no false "Fehlt".
	recon, err := svc.BuildList(ctx, slotlists.Params{
		Date:   pickupDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	assert.Nil(t, rowByStudent(recon.Rows, planned.ID), "a not-yet-started slot must not report its planned children as missing")
	assert.Equal(t, 0, recon.Counters.Missing, "no missing-child count before the slot has started")
	require.Len(t, recon.Slots, 1, "the slot stays selectable context even when excluded from the merge")

	// A slot that has already started (start 09:00 < 12:00) with no presence
	// evidence does reconcile — its expected no-show is a genuine "Fehlt".
	started := &scheduleModels.ActivityInstance{
		Date:            pickupDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa-Started %d", suffix),
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusPlanned,
		ListKind:        &mensa,
	}
	started.SetTenantID(1)
	_, err = db.NewInsert().Model(started).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)
	startedRow := &scheduleModels.InstanceStudent{
		InstanceID: started.ID,
		StudentID:  planned.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	startedRow.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, startedRow))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", startedRow.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", started.ID)
	})

	reconStarted, err := svc.BuildList(ctx, slotlists.Params{
		Date:        pickupDate,
		Target:      slotlists.TargetSlots,
		InstanceIDs: []int64{started.ID},
		Source:      slotlists.SourceReconciliation,
	})
	require.NoError(t, err)
	missing := rowByStudent(reconStarted.Rows, planned.ID)
	require.NotNil(t, missing, "an already-started slot reconciles its expected children")
	assert.Equal(t, "Fehlt", missing.StatusLabel)
	assert.Equal(t, 1, reconStarted.Counters.Missing)
}

// A slot reconciliation generated before the slot's scheduled start defers the
// unexplained no-show (no false "Fehlt"), but a child already signed off
// sick/excused carries a registered absence on the roster row (ApplyStatusDay
// stamps it absent + substatus) that is a valid all-day sign-off — not a
// not-yet-due no-show. The deferred void plan must keep it as "Abgemeldet" and
// count it as Excused, mirroring the pickup-cohort reconciliation, instead of
// discarding the row until the slot's start (#1565 review pass 1 P2).
func TestBuildList_SlotReconciliationKeepsRegisteredAbsenceBeforeStart(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-RegAbsRoom-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("SL-RegAbsAct-%d", suffix))
	mensa := activitiesModels.ListKindMensa

	// pickupNow is 12:00; this occurrence starts at 15:00 → not yet started, so
	// the reconciliation reaches it deferred (no active group yet).
	future := &scheduleModels.ActivityInstance{
		Date:            pickupDate,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("Mensa-Future %d", suffix),
		StartTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusPlanned,
		ListKind:        &mensa,
	}
	future.SetTenantID(1)
	_, err := db.NewInsert().Model(future).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	excused := testpkg.CreateTestStudent(t, db, "SL-RegAbsExcused", fmt.Sprintf("RE-%d", suffix), "3a")
	notYetDue := testpkg.CreateTestStudent(t, db, "SL-RegAbsExpected", fmt.Sprintf("RP-%d", suffix), "3a")

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	var isIDs []int64
	excusedSubstatus := scheduleModels.AttendanceSubstatusExcused
	for _, sc := range []struct {
		studentID int64
		status    string
		substatus *string
	}{
		// Signed off excused: absent + substatus, exactly what ApplyStatusDay writes.
		{studentID: excused.ID, status: scheduleModels.AttendanceStatusAbsent, substatus: &excusedSubstatus},
		// A plain expected child with no sign-off — the not-yet-due no-show that must
		// still be deferred.
		{studentID: notYetDue.ID, status: scheduleModels.AttendanceStatusExpected},
	} {
		row := &scheduleModels.InstanceStudent{
			InstanceID: future.ID,
			StudentID:  sc.studentID,
			Status:     sc.status,
			Substatus:  sc.substatus,
		}
		row.SetTenantID(1)
		require.NoError(t, isRepo.Create(ctx, row))
		isIDs = append(isIDs, row.ID)
	}

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", isIDs...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", future.ID)
		testpkg.CleanupActivityFixtures(t, db, excused.ID)
		testpkg.CleanupActivityFixtures(t, db, notYetDue.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
	})

	svc := newTestService(db)
	recon, err := svc.BuildList(ctx, slotlists.Params{
		Date:   pickupDate,
		Target: slotlists.TargetSlots,
		Source: slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	// The registered absence survives the deferred void plan as "Abgemeldet".
	excusedRow := rowByStudent(recon.Rows, excused.ID)
	require.NotNil(t, excusedRow, "a signed-off child must still show before the slot starts")
	assert.True(t, excusedRow.Excused)
	assert.Equal(t, "Abgemeldet (entschuldigt)", excusedRow.StatusLabel)

	// The unexplained not-yet-due no-show is still deferred — no false "Fehlt".
	assert.Nil(t, rowByStudent(recon.Rows, notYetDue.ID),
		"a not-yet-due no-show must not print before the slot starts")

	assert.Equal(t, 1, recon.Counters.Excused, "the sign-off counts as Abgemeldet before start")
	assert.Equal(t, 0, recon.Counters.Missing, "no missing-child count before the slot has started")
}

// TestBuildList_PickupReconciliationCancelledButPresentIsUnplanned proves the
// pickup path agrees with the slot path (#1565 review): a child whose care day
// was signed off ("Kommt heute nicht") but who attends anyway is unplanned
// presence ("Ungeplant anwesend"), never planned-and-present ("Anwesend").
func TestBuildList_PickupReconciliationCancelledButPresentIsUnplanned(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	weekday := int(pickupDate.Weekday())

	child := testpkg.CreateTestStudent(t, db, "SL-CxlPresent", fmt.Sprintf("CP-%d", suffix), "2c")
	staff := testpkg.CreateTestStaff(t, db, "SL-Staff", fmt.Sprintf("SCP-%d", suffix))
	device := testpkg.CreateTestDevice(t, db, fmt.Sprintf("sl-cxlp-%d", suffix))

	// Regular long-day pickup, then a timeless "Kommt heute nicht" pickup
	// exception → cancelled care day whose cohort falls back to the regular
	// 16:00 bucket.
	pickupRepo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	sched := &scheduleModels.StudentPickupSchedule{
		StudentID: child.ID, Weekday: weekday,
		PickupTime: time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC), CreatedBy: staff.ID,
	}
	sched.SetTenantID(1)
	require.NoError(t, pickupRepo.Create(ctx, sched))

	exc := &scheduleModels.StudentPickupException{
		StudentID: child.ID, ExceptionDate: pickupDate, PickupTime: nil, CreatedBy: staff.ID,
	}
	exc.SetTenantID(1)
	_, err := db.NewInsert().Model(exc).ModelTableExpr(`schedule.student_pickup_exceptions`).Exec(ctx)
	require.NoError(t, err)

	// The child attended anyway (closed attendance still counts).
	checkIn := atOn(pickupDate, 8, 0)
	checkOut := atOn(pickupDate, 15, 30)
	att := &activeModels.Attendance{
		StudentID:    child.ID,
		Date:         pickupDate,
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CheckedInBy:  staff.ID,
		DeviceID:     device.ID,
	}
	att.SetTenantID(1)
	_, err = db.NewInsert().Model(att).ModelTableExpr(`active.attendance`).Exec(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "active.attendance", att.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_exceptions", exc.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_schedules", sched.ID)
		testpkg.CleanupActivityFixtures(t, db, child.ID, staff.ID, device.ID)
	})

	svc := newTestService(db)
	result, err := svc.BuildList(ctx, slotlists.Params{
		Date:         pickupDate,
		Target:       slotlists.TargetPickupCohort,
		PickupCohort: slotlists.PickupCohortLongDay,
		Source:       slotlists.SourceReconciliation,
	})
	require.NoError(t, err)

	row := rowByStudent(result.Rows, child.ID)
	require.NotNil(t, row, "a cancelled-but-present child stays on the reconciliation list")
	assert.True(t, row.Unplanned, "cancelled-but-attended is unplanned presence")
	assert.False(t, row.Planned, "a signed-off child who attends is not planned-and-present")
	assert.False(t, row.Excused, "attendance overrides the sign-off — it is not an excused absence")
	assert.Equal(t, "Ungeplant anwesend", row.StatusLabel)
	assert.Equal(t, 1, result.Counters.Unplanned)
	assert.Equal(t, 0, result.Counters.Excused)
	assert.Equal(t, 0, result.Counters.Missing)
}
