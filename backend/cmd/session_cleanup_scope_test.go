package cmd

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
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

func (s *scopedSessionCleanup) EndDailySessions(ctx context.Context) (*studentpresence.DailySessionCleanupResult, error) {
	id := (testpkg.SettingsRuntimeAdapter{}).TenantID(ctx)
	require.Positive(s.t, id, "daily cleanup must receive a school")
	s.daily[id] = true
	return &studentpresence.DailySessionCleanupResult{Success: true}, nil
}

func TestSessionCleanupExecutesForEachSchool(t *testing.T) {
	t.Parallel()
	cleanupDB, schools, _ := setupTestSessionCleanup(t)
	first, _ := testpkg.CreateTestTenant(t, cleanupDB.Pool())
	second, _ := testpkg.CreateTestTenant(t, cleanupDB.Pool())
	service := &scopedSessionCleanup{t: t, abandoned: map[int64]bool{}, daily: map[int64]bool{}}
	logger := log.New(io.Discard, "", 0)
	require.NoError(t, runAbandonedSessionCleanup(cleanupDB, schools, service, io.Discard, logger, time.Minute, false))
	require.NoError(t, runDailySessionCleanup(cleanupDB, schools, service, io.Discard, logger, false, false))
	for _, id := range []int64{first, second} {
		require.True(t, service.abandoned[id])
		require.True(t, service.daily[id])
	}
}
