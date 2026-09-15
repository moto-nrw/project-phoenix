package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type overviewStaffQueryFunc func(context.Context) ([]OverviewStaff, error)

func (f overviewStaffQueryFunc) ListOverviewStaff(ctx context.Context) ([]OverviewStaff, error) {
	return f(ctx)
}

func TestOverviewStaffFailureStopsReports(t *testing.T) {
	t.Parallel()
	failure := errors.New("staff directory unavailable")
	svc := &staffOverviewService{staffRepo: overviewStaffQueryFunc(func(context.Context) ([]OverviewStaff, error) {
		return []OverviewStaff{{ID: 42}}, failure
	})}
	svc.todayFunc = func() timezone.Date { return timezone.NewDate(2026, time.July, 15) }
	ctx := context.Background()
	overview, err := svc.GetTimeTrackingOverview(ctx, OverviewFilters{})
	require.ErrorIs(t, err, failure)
	assert.ErrorContains(t, err, "failed to list staff")
	assert.Nil(t, overview)
	dashboard, err := svc.GetDashboardSummary(ctx, OverviewPeriodMonth)
	require.ErrorIs(t, err, failure)
	assert.Nil(t, dashboard)
	rows, err := svc.GetMonthExportRows(ctx, 2026, 7)
	require.ErrorIs(t, err, failure)
	assert.Nil(t, rows)
}
