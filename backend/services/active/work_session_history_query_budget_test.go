package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
			session := &activeModels.WorkSession{StaffID: f.staff.ID, Date: timezone.NewDate(snapshotYear, time.August, day),
				Status: activeModels.WorkSessionStatusPresent, Source: activeModels.WorkSessionSourceApp,
				CheckInTime: checkIn, CheckOutTime: &checkOut, CreatedBy: f.staff.ID}
			session.SetTenantID(f.tenantID)
			require.NoError(t, f.repos.WorkSession.Create(f.ctx, session))
			ended := checkIn.Add(30 * time.Minute)
			brk := &activeModels.WorkSessionBreak{SessionID: session.ID, StartedAt: checkIn, EndedAt: &ended, DurationMinutes: 30}
			brk.SetTenantID(f.tenantID)
			require.NoError(t, f.repos.WorkSessionBreak.Create(f.ctx, brk))
		}
	}
	add(0, 2)
	service := f.newAdminSessionService()
	counter := testpkg.CaptureQueriesForContext(t, f.db)
	ctx := counter.Context(f.ctx)
	run := func() []string {
		counter.Reset()
		_, err := service.GetHistory(ctx, f.staff.ID, timezone.NewDate(snapshotYear, time.August, 1), timezone.NewDate(snapshotYear, time.August, 31))
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(2, 7)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.active.work_session_history.reads", large)
}
