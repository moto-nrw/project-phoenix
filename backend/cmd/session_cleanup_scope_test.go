package cmd

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type scopedSessionCleanup struct {
	t                *testing.T
	abandoned, daily map[int64]bool
}

func (s *scopedSessionCleanup) CleanupAbandonedSessions(ctx context.Context, _ time.Duration) (int, error) {
	id := (testpkg.SettingsRuntimeAdapter{}).TenantID(ctx)
	require.Positive(s.t, id, "abandoned cleanup must receive a school")
	s.abandoned[id] = true
	return 0, nil
}

func (s *scopedSessionCleanup) EndDailySessions(ctx context.Context) (*active.DailySessionCleanupResult, error) {
	id := (testpkg.SettingsRuntimeAdapter{}).TenantID(ctx)
	require.Positive(s.t, id, "daily cleanup must receive a school")
	s.daily[id] = true
	return &active.DailySessionCleanupResult{Success: true}, nil
}

func TestSessionCleanupExecutesForEachSchool(t *testing.T) {
	t.Parallel()
	cc := setupTestCleanupContextWithServices(t)
	first, _ := testpkg.CreateTestTenant(t, cc.DB)
	second, _ := testpkg.CreateTestTenant(t, cc.DB)
	service := &scopedSessionCleanup{t: t, abandoned: map[int64]bool{}, daily: map[int64]bool{}}
	cc.SessionCleanupService = service
	cc.Output = io.Discard
	require.NoError(t, runAbandonedSessionCleanup(cc, time.Minute, false))
	require.NoError(t, runDailySessionCleanup(cc, false, false))
	for _, id := range []int64{first, second} {
		require.True(t, service.abandoned[id])
		require.True(t, service.daily[id])
	}
}
