package timetracking_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanupStaleDay lies before yesterday on every run, so the nightly cleanup
// closes its open sessions.
var cleanupStaleDay = calendar.NewDate(2025, time.August, 4)

// TestCleanupOpenSessions_ReadsActiveBreaksOnce pins #5855: the nightly
// cleanup looks up the running breaks of all stale sessions with one read
// instead of one per session. The session with a running break still gets it
// ended at the end of its day before the close; with that one break, three
// and eight stale sessions differ only by the close per extra session.
func TestCleanupOpenSessions_ReadsActiveBreaksOnce(t *testing.T) {
	t.Parallel()

	n3 := runCleanupWithStaleSessions(t, 3)
	n8 := runCleanupWithStaleSessions(t, 8)
	t.Logf("#5855 cleanup statements: N3=%d N8=%d (break reads %d/%d)", n3.total, n8.total, n3.breakReads, n8.breakReads)

	assert.Equal(t, 1, n3.breakReads, "running breaks are read once")
	assert.Equal(t, 1, n8.breakReads, "running breaks are read once")
	assert.Equal(t, n3.total+2*5, n8.total, "each extra stale session costs its balance lock and close")
}

type cleanupStatements struct {
	breakReads, total int
}

func runCleanupWithStaleSessions(t *testing.T, stale int) cleanupStatements {
	t.Helper()
	var counted cleanupStatements
	t.Run(fmt.Sprintf("stale_%d", stale), func(t *testing.T) {
		db := testpkg.SetupTestDB(t)
		ctx := testpkg.OwnCtx(t)
		workTime := owners.MustNewWorkforce(testpkg.NewDatabase(t, db))
		sessions := make([]*timetracking.WorkSession, 0, stale)
		for i := range stale {
			staff := testpkg.CreateTestStaff(t, db, "Vergessen", fmt.Sprintf("Sitzung-%d", i))
			session := &timetracking.WorkSession{
				StaffID: staff.ID, Date: cleanupStaleDay,
				Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
				CheckInTime: cleanupStaleDay.BerlinMidnight().Add(8 * time.Hour),
				CreatedBy:   staff.ID,
			}
			session.SetTenantID(testpkg.Tenant(t))
			createSessionFixture(t, workTime, ctx, session)
			sessions = append(sessions, session)
		}
		running := &timetracking.WorkSessionBreak{SessionID: sessions[0].ID, StartedAt: cleanupStaleDay.BerlinMidnight().Add(12 * time.Hour)}
		running.SetTenantID(testpkg.Tenant(t))
		createBreakFixture(t, workTime, ctx, running)

		service := newAutoCheckoutService(t, db, workTime)
		counter := testpkg.CaptureQueriesForContext(t, db)
		var closed int
		err := testpkg.WithinTenantContext(t, counter.Context(ctx), db, testpkg.Tenant(t), func(txCtx context.Context) error {
			var runErr error
			closed, runErr = service.CleanupOpenSessions(txCtx)
			return runErr
		})
		counter.Stop()
		require.NoError(t, err)
		assert.Equal(t, stale, closed)

		breaks, err := workTime.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessions[0].ID})
		require.NoError(t, err)
		require.Len(t, breaks, 1)
		require.NotNil(t, breaks[0].EndedAt, "the running break ends with its session")
		assert.True(t, breaks[0].EndedAt.Equal(cleanupStaleDay.EndOfDay()))
		open, err := workTime.ListWorkSessions(ctx, workforce.WorkSessionFilter{Open: new(true)})
		require.NoError(t, err)
		assert.Empty(t, open, "every stale session is closed")

		counted = cleanupStatements{
			breakReads: len(counter.Matching(func(sql string) bool {
				return strings.HasPrefix(strings.TrimSpace(sql), "select") && strings.Contains(sql, "work_session_breaks") && strings.Contains(sql, "ended_at is null")
			})),
			total: counter.Total(),
		}
	})
	return counted
}
