package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingGroupVisitClose struct {
	activeService.StudentPresence
	afterClose error
}

func (p *failingGroupVisitClose) CloseGroupVisits(ctx context.Context, ids []int64) (int64, error) {
	count, err := p.StudentPresence.CloseGroupVisits(ctx, ids)
	if err == nil {
		err = p.afterClose
	}
	return count, err
}

func TestDailySessionClosureRollsBackPresenceWriteAndRetries(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"visit write", "presence setting"} {
		t.Run(fault, func(t *testing.T) {
			testDailySessionClosureFailure(t, fault)
		})
	}
}

func testDailySessionClosureFailure(t *testing.T, fault string) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := testSchoolPresence(t, db)
	injected := errors.New("fail after bulk visit closure")
	presence := &failingGroupVisitClose{StudentPresence: module}
	var settingsErr error
	if fault == "visit write" {
		presence.afterClose = injected
	} else {
		settingsErr = injected
	}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	svc.SetSettingsService(&configtest.Mock{ResolveStringFn: func(context.Context, string) (string, error) {
		return activeService.PresenceModeDetailed, settingsErr
	}})
	testpkg.SetTenantRuntime(t, svc, db)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	student := testpkg.CreateTestStudent(t, db, "Bulk", "Closure", "3a")
	visit, err := module.RecordVisit(ctx, studentpresence.Visit{
		StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now(),
	})
	require.NoError(t, err)

	failed, err := svc.EndDailySessions(ctx)
	require.ErrorIs(t, err, injected)
	if fault == "presence setting" {
		require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
	}
	assert.Empty(t, broadcaster.Calls())
	assert.False(t, failed.Success)
	stored, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ExitTime)
	session, err := svc.GetActiveGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Nil(t, session.EndTime, "failed visit closure must not close the session")

	presence.afterClose = nil
	settingsErr = nil
	result, err := svc.EndDailySessions(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.VisitsEnded)
	assert.Contains(t, result.EndedActiveGroupIDs, group.ID)
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ExitTime)
	closedAt := *stored.ExitTime

	result, err = svc.EndDailySessions(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Zero(t, result.VisitsEnded)
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Equal(t, closedAt, *stored.ExitTime, "retry must not rewrite the recorded departure")
}
