package timetracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkSessionHistoryQueryBudget(t *testing.T) {
	t.Parallel()
	f := newSnapshotFixture(t)
	add := func(from, to int) {
		for i := from; i < to; i++ {
			day := snapshotSessionDay + i + 1
			checkIn := time.Date(snapshotYear, time.August, day, 8, 0, 0, 0, time.UTC)
			checkOut := checkIn.Add(8 * time.Hour)
			session := &timetracking.WorkSession{StaffID: f.staff, Date: calendar.NewDate(snapshotYear, time.August, day),
				Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
				CheckInTime: checkIn, CheckOutTime: &checkOut, CreatedBy: f.staff}
			session.SetTenantID(f.tenantID)
			createSessionFixture(t, f.workforce, f.ctx, session)
			ended := checkIn.Add(30 * time.Minute)
			brk := &timetracking.WorkSessionBreak{SessionID: session.ID, StartedAt: checkIn, EndedAt: &ended, DurationMinutes: 30}
			brk.SetTenantID(f.tenantID)
			createBreakFixture(t, f.workforce, f.ctx, brk)
		}
	}
	add(0, 2)
	service := f.newAdminSessionService()
	require.NoError(t, testpkg.WithinTenantContext(t, f.ctx, f.db, testpkg.TenantIDFromContext(f.ctx), func(ctx context.Context) error {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		countedCtx := counter.Context(ctx)
		run := func() []string {
			counter.Reset()
			_, err := service.GetHistory(countedCtx, f.staff, calendar.NewDate(snapshotYear, time.August, 1), calendar.NewDate(snapshotYear, time.August, 31))
			require.NoError(t, err)
			return counter.Operation("SELECT")
		}
		small := run()
		add(2, 7)
		large := run()
		assert.Equal(t, len(small), len(large))
		querybudgets.AssertQueryBudget(t, "services.active.work_session_history.reads", large)
		return nil
	}))
}
