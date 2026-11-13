package timetracking_test

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/application/timetracking"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTargetCoverage_CountsTheOverviewsHasTarget is the payroll check of
// #6890: the count must come from the same has_target the Zeitkonten rows
// carry (#6887), not from a second rule.
func TestTargetCoverage_CountsTheOverviewsHasTarget(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 2)
	for range 2 {
		staff := testpkg.CreateTestStaffForTenant(t, f.db, f.tenantID, "Ohne", "Soll")
		f.staff = append(f.staff, staff.ID)
	}

	coverage, err := testpkg.TenantQuery2(t, f.ctx, f.db, f.svc.TargetCoverage, 0, 0)
	require.NoError(t, err)

	overview, err := testpkg.TenantQuery1(t, f.ctx, f.db, f.svc.GetTimeTrackingOverview, timetracking.OverviewFilters{})
	require.NoError(t, err)
	withoutTarget := 0
	for _, row := range overview.Rows {
		if !row.HasTarget {
			withoutTarget++
		}
	}

	assert.Equal(t, overview.Year, coverage.Year)
	assert.Equal(t, overview.Month, coverage.Month)
	assert.Equal(t, len(overview.Rows), coverage.StaffTotal)
	assert.Equal(t, withoutTarget, coverage.StaffWithoutTarget)
	assert.Equal(t, 4, coverage.StaffTotal)
	assert.Equal(t, 2, coverage.StaffWithoutTarget)

	// Without a month the check covers the next Monatsabschluss: the
	// previous month while it is open, then the current one.
	previous := calendar.NewDate(f.today.Year(), f.today.Month(), 1).AddDays(-1)
	settings := wtmIntSettings{accountStart: calendar.NewDate(previous.Year(), previous.Month(), 1).String()}
	svc := f.newOverviewService(settings)
	open, err := testpkg.TenantQuery2(t, f.ctx, f.db, svc.TargetCoverage, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, previous.Year(), open.Year)
	assert.Equal(t, int(previous.Month()), open.Month)
	assert.Equal(t, 2, open.StaffWithoutTarget)

	monthSvc := timetracking.NewWorkTimeMonthService(
		f.workforce, f.workforce, f.reposAbsences.StaffAbsence, workforceCompose.StaffScheduleAssignments(api.MustNewStaffEmployment(testpkg.NewDatabase(t, f.db))),
		owners.NewWorkScheduleTargets(owners.NewStaffWorkScheduleRepository(f.workforce)), owners.NewWorkTimeTargetModels(owners.NewWorkTimeModelRepository(f.workforce)), workforceCompose.NewTimeTrackingShifts(f.workforce),
		settings, nil,
		timetracking.WithMonthAdjustments(f.workforce),
		timetracking.WithMonthSnapshots(timetracking.MonthSnapshotCapability(f.workforce)),
	)
	closeSvc := timetracking.NewStaffMonthCloseService(timetracking.MonthSnapshotCapability(f.workforce), monthSvc, owners.MonthCloseStaff(f.repos.Staff), settings, nil, timetracking.WithMonthCloseUnitOfWork(testpkg.RunInTenantTransaction))
	_, err = closeSvc.CloseMonth(f.ctx, f.staff[0], previous.Year(), int(previous.Month()), "Abschluss")
	require.NoError(t, err)
	closed, err := testpkg.TenantQuery2(t, f.ctx, f.db, svc.TargetCoverage, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, coverage.Year, closed.Year)
	assert.Equal(t, coverage.Month, closed.Month)

	// The month rules are the overview's: no future month.
	next := f.today.AddDays(40)
	_, err = f.svc.TargetCoverage(f.ctx, next.Year(), int(next.Month()))
	require.ErrorIs(t, err, workforce.ErrOverviewInvalid)
}
