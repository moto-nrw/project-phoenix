package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// pickupExtensionRow is the stored task state the trigger tests assert on.
type pickupExtensionRow struct {
	PickupExceptionID *int64 `bun:"pickup_exception_id"`
	TaskDate          string `bun:"task_date"`
	Weekday           int    `bun:"weekday"`
	PreviousPickup    string `bun:"previous_pickup"`
	Pickup            string `bun:"pickup"`
}

type pickupExtensionHarness struct {
	db      *bun.DB
	ctx     context.Context
	svc     scheduleService.PickupScheduleService
	student int64
	staff   int64
	monday  timezone.Date
}

// setupPickupExtensionHarness wires the pickup service with the real
// Timetable owner as extension recorder (#3261), on a far-future Monday with a
// weekly pickup at 15:00.
func setupPickupExtensionHarness(t *testing.T) *pickupExtensionHarness {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := autoExcusalRepositories(db)
	baselines := scheduletest.NewPickupBaselineService(repos.StudentPickupSchedule, approvedOfferingProjection(t), repos.CareOffering)
	syncer := scheduleService.NewPickupAutoExcusalSyncer(
		repos.StudentPickupException,
		baselines,
		repos.InstanceStudent,
		db,
		scheduleService.WithPickupExtensions(timetabletest.New(t, db)),
	)
	svc := scheduleService.NewPickupScheduleServiceWithBulk(
		repos.StudentPickupSchedule, repos.StudentPickupException, repos.StudentPickupNote,
		repos.Student, repos.Person, syncer, baselines, db, nil,
	)
	student := testpkg.CreateTestStudent(t, db, "Later", "Pickup", "LP1")
	staff := testpkg.CreateTestStaff(t, db, "Later", "Staff")
	testpkg.CreateTestPickupSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "15:00")

	monday := timezone.NewDate(2099, 1, 5)
	require.Equal(t, time.Monday, monday.Weekday(), "fixture date must be a Monday")
	return &pickupExtensionHarness{db: db, ctx: testpkg.Ctx(t), svc: svc, student: student.ID, staff: staff.ID, monday: monday}
}

func (h *pickupExtensionHarness) rows(t *testing.T) []pickupExtensionRow {
	t.Helper()
	rows := make([]pickupExtensionRow, 0)
	err := h.db.NewSelect().TableExpr(`schedule.pickup_extension_tasks AS "task"`).
		ColumnExpr(`"task".pickup_exception_id, COALESCE("task".task_date::text, '') AS task_date`).
		ColumnExpr(`COALESCE("task".weekday, 0) AS weekday`).
		ColumnExpr(`to_char("task".previous_pickup_time, 'HH24:MI') AS previous_pickup`).
		ColumnExpr(`to_char("task".pickup_time, 'HH24:MI') AS pickup`).
		Where(`"task".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"task".student_id = ?`, h.student).
		OrderExpr(`"task".id`).
		Scan(testpkg.WithPackageTenantRuntime(context.Background()), &rows)
	require.NoError(t, err)
	return rows
}

func (h *pickupExtensionHarness) resolveStaff() (int64, error) { return h.staff, nil }

func TestPickupExtension_LaterDayPickupOpensTask(t *testing.T) {
	t.Parallel()
	h := setupPickupExtensionHarness(t)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student, h.monday, wallClockAt(16, 30), nil, h.staff, h.resolveStaff)
	require.NoError(t, err)
	rows := h.rows(t)
	require.Len(t, rows, 1)
	assert.Equal(t, h.monday.String(), rows[0].TaskDate)
	require.NotNil(t, rows[0].PickupExceptionID)
	assert.Equal(t, row.ID, *rows[0].PickupExceptionID)
	assert.Equal(t, "15:00", rows[0].PreviousPickup, "the weekly time is the previous time")
	assert.Equal(t, "16:30", rows[0].Pickup)

	// Moving the day later again updates the same task.
	_, err = h.svc.UpdateException(h.ctx, row.ID, h.student, h.monday, nil, wallClockAt(17, 0), false, h.resolveStaff)
	require.NoError(t, err)
	rows = h.rows(t)
	require.Len(t, rows, 1)
	assert.Equal(t, "17:00", rows[0].Pickup)

	// An earlier pickup is the partial-absence path; no task remains.
	_, err = h.svc.UpdateException(h.ctx, row.ID, h.student, h.monday, nil, wallClockAt(14, 0), false, h.resolveStaff)
	require.NoError(t, err)
	assert.Empty(t, h.rows(t))

	// Later again, then deleting the day removes the task with the exception.
	_, err = h.svc.UpdateException(h.ctx, row.ID, h.student, h.monday, nil, wallClockAt(16, 0), false, h.resolveStaff)
	require.NoError(t, err)
	require.Len(t, h.rows(t), 1)
	require.NoError(t, h.svc.DeleteStudentPickupException(h.ctx, row.ID))
	assert.Empty(t, h.rows(t))
}

func TestPickupExtension_DayWithoutWeeklyTimeOpensNothing(t *testing.T) {
	t.Parallel()
	h := setupPickupExtensionHarness(t)

	tuesday := h.monday.AddDays(1)
	_, err := h.svc.CreateOrReclaimException(h.ctx, h.student, tuesday, wallClockAt(16, 30), nil, h.staff, h.resolveStaff)
	require.NoError(t, err)
	assert.Empty(t, h.rows(t), "without a weekly time there is no \"longer than before\"")
}

func TestPickupExtension_LaterWeekdayOpensTask(t *testing.T) {
	t.Parallel()
	h := setupPickupExtensionHarness(t)

	require.NoError(t, h.svc.UpsertStudentPickupSchedule(h.ctx, &scheduleModel.StudentPickupSchedule{
		StudentID: h.student, Weekday: scheduleModel.WeekdayMonday, PickupTime: *wallClockAt(16, 0), CreatedBy: h.staff,
	}))
	rows := h.rows(t)
	require.Len(t, rows, 1)
	assert.Equal(t, scheduleModel.WeekdayMonday, rows[0].Weekday)
	assert.Empty(t, rows[0].TaskDate)
	assert.Nil(t, rows[0].PickupExceptionID)
	assert.Equal(t, "15:00", rows[0].PreviousPickup)
	assert.Equal(t, "16:00", rows[0].Pickup)

	// A new weekday without a previous time opens nothing.
	require.NoError(t, h.svc.UpsertStudentPickupSchedule(h.ctx, &scheduleModel.StudentPickupSchedule{
		StudentID: h.student, Weekday: scheduleModel.WeekdayWednesday, PickupTime: *wallClockAt(16, 0), CreatedBy: h.staff,
	}))
	assert.Len(t, h.rows(t), 1)

	// The bulk editor path records through the same comparison.
	require.NoError(t, h.svc.UpsertBulkStudentPickupSchedules(h.ctx, h.student, []*scheduleModel.StudentPickupSchedule{
		{StudentID: h.student, Weekday: scheduleModel.WeekdayMonday, PickupTime: *wallClockAt(15, 0), CreatedBy: h.staff},
		{StudentID: h.student, Weekday: scheduleModel.WeekdayWednesday, PickupTime: *wallClockAt(16, 0), CreatedBy: h.staff},
	}))
	assert.Empty(t, h.rows(t), "back to the first time closes the task")

	require.NoError(t, h.svc.UpsertStudentPickupSchedule(h.ctx, &scheduleModel.StudentPickupSchedule{
		StudentID: h.student, Weekday: scheduleModel.WeekdayMonday, PickupTime: *wallClockAt(16, 15), CreatedBy: h.staff,
	}))
	require.Len(t, h.rows(t), 1)
	require.NoError(t, h.svc.DeleteAllStudentPickupSchedules(h.ctx, h.student))
	assert.Empty(t, h.rows(t), "a removed weekday closes its task")
}
