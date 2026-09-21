package timetracking_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services"
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

	testpkg.CreateTestStaffWorkScheduleForTenant(t, db, tenantID, staff.ID, timetracking.DayMonday, 480, scheduleValidFrom)
	service := timetracking.NewWorkTimeMonthService(
		repos.WorkSession, repos.WorkSessionBreak, repos.StaffAbsence, services.StaffScheduleAssignments(repositories.MustNewStaffEmployment(db)),
		services.NewWorkScheduleTargets(repos.StaffWorkSchedule), services.NewWorkTimeTargetModels(repos.WorkTimeModel), services.NewTimeTrackingShifts(repos.StaffShift),
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
