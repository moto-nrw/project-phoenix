package timetracking_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/application/timetracking"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

// TestCleanupOpenSessions_ClosesSessionsCheckedInAfterTheirDay pins #6288
// (ZE-1): a session whose recorded check-in lies after the end of its date
// (a self-edit moved the date back or the check-in forward before the save
// validated it) used to be closed at the end of its date. That violates
// chk_work_sessions_checkout_after_checkin, aborts the school's nightly
// cleanup transaction and repeats every night. Such existing rows close one
// second after their check-in, or at their running break's start, instead.
func TestCleanupOpenSessions_ClosesSessionsCheckedInAfterTheirDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	workTime := owners.MustNewWorkforce(testpkg.NewDatabase(t, db))
	poisoned := func(lastName string, checkIn time.Time) *timetracking.WorkSession {
		t.Helper()
		staff := testpkg.CreateTestStaff(t, db, "Vergiftet", lastName)
		session := &timetracking.WorkSession{
			StaffID: staff.ID, Date: cleanupStaleDay,
			Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
			CheckInTime: checkIn, CreatedBy: staff.ID,
		}
		session.SetTenantID(testpkg.Tenant(t))
		createSessionFixture(t, workTime, ctx, session)
		return session
	}
	laterDay := poisoned("Datum", cleanupStaleDay.AddDays(3).BerlinMidnight().Add(8*time.Hour))
	withBreak := poisoned("Pause", cleanupStaleDay.AddDays(2).BerlinMidnight().Add(9*time.Hour))
	breakStart := withBreak.CheckInTime.Add(2 * time.Hour)
	running := &timetracking.WorkSessionBreak{SessionID: withBreak.ID, StartedAt: breakStart}
	running.SetTenantID(testpkg.Tenant(t))
	createBreakFixture(t, workTime, ctx, running)

	service := newAutoCheckoutService(t, db, workTime)
	var closed int
	err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		var runErr error
		closed, runErr = service.CleanupOpenSessions(txCtx)
		return runErr
	})
	require.NoError(t, err, "an existing session checked in after its day must not fail the nightly cleanup")
	assert.Equal(t, 2, closed)

	stored, err := workTime.ListWorkSessions(ctx, workforce.WorkSessionFilter{IDs: []int64{laterDay.ID, withBreak.ID}})
	require.NoError(t, err)
	byID := make(map[int64]workforce.WorkSession, len(stored))
	for _, session := range stored {
		byID[session.ID] = session
	}
	require.NotNil(t, byID[laterDay.ID].CheckOutTime)
	assert.True(t, byID[laterDay.ID].CheckOutTime.Equal(laterDay.CheckInTime.Add(time.Second)))
	require.NotNil(t, byID[withBreak.ID].CheckOutTime)
	assert.True(t, byID[withBreak.ID].CheckOutTime.Equal(breakStart), "the session cannot end before its running break began")
	breaks, err := workTime.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: withBreak.ID})
	require.NoError(t, err)
	require.Len(t, breaks, 1)
	require.NotNil(t, breaks[0].EndedAt)
	assert.True(t, breaks[0].EndedAt.Equal(breakStart), "the break never ends before it started")
}

// failingBreakEnd refuses ending a break with a database error inside the
// cleanup's transaction. In PostgreSQL such an error aborts the whole
// transaction unless a savepoint contains it.
type failingBreakEnd struct {
	timetracking.WorkSessionBreakRecords
}

func (failingBreakEnd) EndWorkSessionBreak(ctx context.Context, _ int64, _ time.Time, _ int) (int64, error) {
	return 0, testpkg.FailInCurrentTransaction(ctx)
}

// TestCleanupOpenSessions_BreakEndFailureStillClosesTheSession pins the
// promise "continue cleanup even if break ending fails" against a real
// transaction (#6288): a failed break end used to abort the cleanup's
// transaction, so the session close and every later step failed with it.
func TestCleanupOpenSessions_BreakEndFailureStillClosesTheSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	workTime := owners.MustNewWorkforce(testpkg.NewDatabase(t, db))
	staff := testpkg.CreateTestStaff(t, db, "Pause", "Bleibt")
	session := &timetracking.WorkSession{
		StaffID: staff.ID, Date: cleanupStaleDay,
		Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
		CheckInTime: cleanupStaleDay.BerlinMidnight().Add(8 * time.Hour), CreatedBy: staff.ID,
	}
	session.SetTenantID(testpkg.Tenant(t))
	createSessionFixture(t, workTime, ctx, session)
	running := &timetracking.WorkSessionBreak{SessionID: session.ID, StartedAt: session.CheckInTime.Add(4 * time.Hour)}
	running.SetTenantID(testpkg.Tenant(t))
	createBreakFixture(t, workTime, ctx, running)

	service := newWorkSessionTestService(t, db, workTime, failingBreakEnd{WorkSessionBreakRecords: workTime},
		api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db)).GroupSupervisor)
	var closed int
	err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		var runErr error
		closed, runErr = service.CleanupOpenSessions(txCtx)
		return runErr
	})
	require.NoError(t, err, "a failed break end must not fail the nightly close")
	assert.Equal(t, 1, closed)
	stored, err := workTime.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffID: staff.ID})
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.NotNil(t, stored[0].CheckOutTime, "the session closes")
	assert.True(t, stored[0].CheckOutTime.Equal(cleanupStaleDay.EndOfDay()))
}

// TestCheckIn_StaleBlockBreakEndFailureStillChecksIn covers the same promise
// on the check-in repair of a forgotten block (#6288): a break that cannot be
// ended must not keep the person from stamping.
func TestCheckIn_StaleBlockBreakEndFailureStillChecksIn(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	workTime := owners.MustNewWorkforce(testpkg.NewDatabase(t, db))
	staff := testpkg.CreateTestStaff(t, db, "Vergessen", "Pause")
	stale := &timetracking.WorkSession{
		StaffID: staff.ID, Date: cleanupStaleDay,
		Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
		CheckInTime: cleanupStaleDay.BerlinMidnight().Add(8 * time.Hour), CreatedBy: staff.ID,
	}
	stale.SetTenantID(testpkg.Tenant(t))
	createSessionFixture(t, workTime, ctx, stale)
	running := &timetracking.WorkSessionBreak{SessionID: stale.ID, StartedAt: stale.CheckInTime.Add(4 * time.Hour)}
	running.SetTenantID(testpkg.Tenant(t))
	createBreakFixture(t, workTime, ctx, running)

	service := newWorkSessionTestService(t, db, workTime, failingBreakEnd{WorkSessionBreakRecords: workTime},
		api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db)).GroupSupervisor)
	err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		_, checkInErr := service.CheckIn(txCtx, staff.ID, workforce.WorkSessionStatusPresent, workforce.WorkSessionSourceApp, "")
		return checkInErr
	})
	require.NoError(t, err, "a failed break end must not lock the person out of the clock")
	open, err := workTime.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffID: staff.ID, Open: new(true)})
	require.NoError(t, err)
	require.Len(t, open, 1)
	assert.NotEqual(t, stale.ID, open[0].ID, "the forgotten block is closed and a new one runs")
}
