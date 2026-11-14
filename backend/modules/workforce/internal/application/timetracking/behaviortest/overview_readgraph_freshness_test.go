package timetracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type quotaReadBoundary struct {
	timetracking.VacationQuotaRecords
	after func()
}

func (q quotaReadBoundary) ListStaffVacationQuotas(ctx context.Context, filter workforce.StaffVacationFilter) ([]workforce.StaffVacationQuota, error) {
	rows, err := q.VacationQuotaRecords.ListStaffVacationQuotas(ctx, filter)
	if err == nil {
		q.after()
	}
	return rows, err
}

// When the month prefetch covers the Resturlaub window, both use that one
// absence read (#5787, decision E2: an early shared read replaces the former
// second READ COMMITTED read). A vacation committed after it is therefore in
// neither the month arithmetic nor the Resturlaub of this view; the next
// view counts it.
func TestOverviewVacationSharesTheMonthPrefetch(t *testing.T) {
	t.Parallel()
	f := newOverviewFixture(t, 1)
	date := calendar.NewDate(2025, time.January, 6)
	committed := false
	quota := quotaReadBoundary{VacationQuotaRecords: f.workforce, after: func() {
		if committed {
			return
		}
		committed = true
		// f.ctx predates the overview's ambient unit. This owner command opens
		// and commits a separate tenant transaction before Resturlaub reads.
		f.addAbsence(t, f.staff[0], workforce.AbsenceTypeVacation, workforce.AbsenceStatusApproved, date, date)
	}}
	svc := timetracking.NewStaffOverviewService(
		owners.OverviewStaff(f.repos.Staff), f.workforce, f.workforce, f.reposAbsences.StaffAbsence,
		f.workforce, quota, timetracking.MonthSnapshotCapability(f.workforce),
		owners.NewWorkScheduleTargets(owners.NewStaffWorkScheduleRepository(f.workforce)), owners.NewWorkTimeTargetModels(owners.NewWorkTimeModelRepository(f.workforce)), workforceCompose.NewTimeTrackingShifts(f.workforce),
		wtmIntSettings{accountStart: "2025-01-01"}, nil,
	)
	view, err := testpkg.TenantQuery1(t, f.ctx, f.db, svc.GetTimeTrackingOverview, timetracking.OverviewFilters{Year: 2025, Month: 1})
	require.NoError(t, err)
	require.Len(t, view.Rows, 1)
	require.Equal(t, float64(30), view.Rows[0].RemainingVacationDays, "the vacation committed after the shared read is not in this view")
	counter := testpkg.CaptureQueriesForContext(t, f.db)
	next, err := testpkg.TenantQuery1(t, counter.Context(f.ctx), f.db, svc.GetTimeTrackingOverview, timetracking.OverviewFilters{Year: 2025, Month: 1})
	counter.Stop()
	t.Logf("overview SQL=%d staff_absences=%d", counter.Total(), len(counter.Selects("active.staff_absences")))
	require.Len(t, counter.Selects("active.staff_absences"), 1, "month math and Resturlaub share one absence read")
	require.NoError(t, err)
	require.Len(t, next.Rows, 1)
	require.Equal(t, float64(29), next.Rows[0].RemainingVacationDays, "the next view spends the committed vacation")
}
