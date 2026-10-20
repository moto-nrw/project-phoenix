package timetracking_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	workTime := owners.MustNewWorkforce(testpkg.NewDatabase(t, db))
	repos := api.NewStaffAbsenceSuiteRepositories(testpkg.NewDatabase(t, db))
	ctx := testpkg.TenantContext(tenantID)

	testpkg.CreateTestStaffWorkScheduleForTenant(t, db, tenantID, staff.ID, timetracking.DayMonday, 480, scheduleValidFrom)
	service := timetracking.NewWorkTimeMonthService(
		workTime, workTime, repos.StaffAbsence, workforceCompose.StaffScheduleAssignments(api.MustNewStaffEmployment(testpkg.NewDatabase(t, db))),
		owners.NewWorkScheduleTargets(owners.NewStaffWorkScheduleRepository(workTime)), owners.NewWorkTimeTargetModels(owners.NewWorkTimeModelRepository(workTime)), workforceCompose.NewTimeTrackingShifts(workTime),
		wtmIntSettings{accountStart: "2020-01-01"}, nil,
	)
	first := calendar.TodayDate().AddDays(14)
	add := func(from, to int) {
		for i := from; i < to; i++ {
			date := first.AddDays(i * 7)
			absence := &timetracking.StaffAbsence{
				StaffID: staff.ID, AbsenceType: workforce.AbsenceTypeCompTime,
				Status: workforce.AbsenceStatusApproved, DateStart: date, DateEnd: date,
				CreatedBy: staff.ID,
			}
			absence.SetTenantID(tenantID)
			absenceCommand1 := timetracking.StaffAbsenceToCapability(absence)
			createdTimeRecord1, err := repos.StaffAbsence.CreateStaffAbsence(ctx, &absenceCommand1)
			require.NoError(t, err)
			*absence = *timetracking.AbsenceRecord(createdTimeRecord1)
		}
	}
	add(0, 3)
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.TenantIDFromContext(ctx), func(ctx context.Context) error {
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
		return nil
	}))
}
