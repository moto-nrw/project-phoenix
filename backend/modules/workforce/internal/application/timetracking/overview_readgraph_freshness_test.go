package timetracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

// Month arithmetic and Resturlaub deliberately observe absences at different
// READ COMMITTED boundaries, even when their date windows happen to match.
func TestOverviewVacationReadsCommitAfterMonthPrefetch(t *testing.T) {
	t.Parallel()
	f := newOverviewFixture(t, 1)
	date := calendar.NewDate(2025, time.January, 6)
	quota := quotaReadBoundary{VacationQuotaRecords: f.workforce, after: func() {
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
	require.Equal(t, float64(29), view.Rows[0].RemainingVacationDays, "the committed vacation must be spent in the later Resturlaub observation")
}
