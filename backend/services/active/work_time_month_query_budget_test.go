package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFutureCompTimeCommitmentQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "CompTime", "Budget")
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.TenantContext(tenantID)

	schedule := &configModels.StaffWorkSchedule{
		TenantID: tenantID, StaffID: staff.ID, DayOfWeek: configModels.DayMonday,
		TargetMinutes: 480, WeekIndex: 0, RotationLength: 1,
		ValidFrom: configModels.NewCalendarDate(2020, time.January, 1),
	}
	_, err := db.NewInsert().Model(schedule).ModelTableExpr("config.staff_work_schedules").Exec(ctx)
	require.NoError(t, err)
	service := active.NewWorkTimeMonthService(
		repos.WorkSession, repos.WorkSessionBreak, repos.StaffAbsence, repos.Staff,
		repos.StaffWorkSchedule, repos.WorkTimeModel, repos.StaffShift,
		wtmIntSettings{accountStart: "2020-01-01"}, nil,
	)
	first := timezone.TodayDate().AddDays(14)
	add := func(from, to int) {
		for i := from; i < to; i++ {
			date := first.AddDays(i * 7)
			absence := &activeModels.StaffAbsence{
				StaffID: staff.ID, AbsenceType: activeModels.AbsenceTypeCompTime,
				Status: activeModels.AbsenceStatusApproved, DateStart: date, DateEnd: date,
				CreatedBy: staff.ID,
			}
			absence.SetTenantID(tenantID)
			require.NoError(t, repos.StaffAbsence.Create(ctx, absence))
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueriesForContext(t, db)
	countedCtx := counter.Context(ctx)
	run := func() []string {
		counter.Reset()
		_, err := service.GetFutureCompTimeCommitmentMinutes(countedCtx, staff.ID)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.active.future_comp_time_commitment.reads", large)
}
