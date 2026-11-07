package timetracking

// Mock-based invariants of the nightly CleanupOpenSessions protocol (#5855):
// one read finds the running breaks, break failures stay warnings, a close
// failure stays fatal with the count so far. int64 literals are fake
// in-memory IDs, not DB rows.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/tenant/tenanttest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cleanupMockDay = NewDate(2026, time.March, 2)

func cleanupMockSessions(ids ...int64) []*WorkSession {
	sessions := make([]*WorkSession, 0, len(ids))
	for _, id := range ids {
		sessions = append(sessions, &WorkSession{Model: Model{ID: id}, StaffID: id * 10, Date: cleanupMockDay})
	}
	return sessions
}

// Each session's running break ends at the end of its day, before that
// session is closed.
func TestCleanupOpenSessions_EndsRunningBreakBeforeClose(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	sessionRepo.getOpenSessionsFunc = func(context.Context, Date) ([]*WorkSession, error) {
		return cleanupMockSessions(1, 2), nil
	}
	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, sessionID int64) (*WorkSessionBreak, error) {
		if sessionID != 2 {
			return nil, nil
		}
		return &WorkSessionBreak{Model: Model{ID: 20}, SessionID: 2, StartedAt: cleanupMockDay.BerlinMidnight().Add(12 * time.Hour)}, nil
	}
	var events []string
	var endedAt time.Time
	breakRepo.endBreakFunc = func(_ context.Context, id int64, at time.Time, _ int) error {
		events = append(events, "end-break")
		endedAt = at
		return nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, id int64, _ time.Time, _ bool) (bool, error) {
		events = append(events, "close")
		return true, nil
	}

	count, err := svc.CleanupOpenSessions(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []string{"close", "end-break", "close"}, events)
	assert.True(t, endedAt.Equal(cleanupMockDay.EndOfDay()))
}

// A failed break lookup is a warning: every session is still closed.
func TestCleanupOpenSessions_BreakLookupFailureStillClosesSessions(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	sessionRepo.getOpenSessionsFunc = func(context.Context, Date) ([]*WorkSession, error) {
		return cleanupMockSessions(1, 2), nil
	}
	breakRepo.getActiveBySessionIDFunc = func(context.Context, int64) (*WorkSessionBreak, error) {
		return nil, errors.New("connection reset")
	}
	var closed []int64
	sessionRepo.closeSessionFunc = func(_ context.Context, id int64, _ time.Time, _ bool) (bool, error) {
		closed = append(closed, id)
		return true, nil
	}

	count, err := svc.CleanupOpenSessions(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []int64{1, 2}, closed)
}

// A failed break end is a warning for that session only.
func TestCleanupOpenSessions_BreakEndFailureContinues(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	sessionRepo.getOpenSessionsFunc = func(context.Context, Date) ([]*WorkSession, error) {
		return cleanupMockSessions(1, 2), nil
	}
	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, sessionID int64) (*WorkSessionBreak, error) {
		return &WorkSessionBreak{Model: Model{ID: sessionID * 10}, SessionID: sessionID, StartedAt: cleanupMockDay.BerlinMidnight().Add(12 * time.Hour)}, nil
	}
	var ended []int64
	breakRepo.endBreakFunc = func(_ context.Context, id int64, _ time.Time, _ int) error {
		if id == 10 {
			return errors.New("break row locked")
		}
		ended = append(ended, id)
		return nil
	}
	sessionRepo.closeSessionFunc = func(context.Context, int64, time.Time, bool) (bool, error) { return true, nil }

	count, err := svc.CleanupOpenSessions(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []int64{20}, ended)
}

// A failed close is fatal: the count of sessions closed so far comes back and
// no change is announced.
func TestCleanupOpenSessions_CloseFailureReturnsPriorCountWithoutBroadcast(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	broadcaster := &recordingTimeTrackingPublisher{}
	svc.broadcaster = broadcaster
	ctx, commit := tenanttest.WithAfterCommitHooks(tenant.WithTenantID(context.Background(), 42))
	sessionRepo.getOpenSessionsFunc = func(context.Context, Date) ([]*WorkSession, error) {
		return cleanupMockSessions(1, 2, 3), nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, id int64, _ time.Time, _ bool) (bool, error) {
		if id == 2 {
			return false, errors.New("close error")
		}
		return true, nil
	}

	count, err := svc.CleanupOpenSessions(ctx)
	commit()

	require.ErrorContains(t, err, "failed to close session 2")
	assert.Equal(t, 1, count)
	assert.Empty(t, broadcaster.tenants)
}
