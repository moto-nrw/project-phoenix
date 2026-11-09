package timetracking_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// autoCheckoutDay is a past Monday, so its 08:00-16:00 shift is over for
// every run of the test.
var autoCheckoutDay = calendar.NewDate(2025, time.August, 4)

// TestAutoCheckout_LocksOnlyDueSessions pins #5823: the 60-second tick checks
// each listed open session against its planned shift end before it locks.
// Sessions that are not due take neither the staff balance lock nor the
// session row lock, so with one due session the tick issues the same
// statements whether three or eight not-due sessions are open beside it.
func TestAutoCheckout_LocksOnlyDueSessions(t *testing.T) {
	t.Parallel()

	n3 := runAutoCheckoutWithNotDueSessions(t, 3)
	n8 := runAutoCheckoutWithNotDueSessions(t, 8)
	t.Logf("#5823 auto-checkout statements: N3=%d N8=%d (balance locks %d/%d, row locks %d/%d)",
		n3.total, n8.total, n3.balanceLocks, n8.balanceLocks, n3.rowLocks, n8.rowLocks)

	assert.Equal(t, 1, n3.balanceLocks, "only the due session's staff member is locked")
	assert.Equal(t, n3.balanceLocks, n8.balanceLocks)
	assert.Equal(t, n3.rowLocks, n8.rowLocks, "not-due sessions are not row-locked")
	assert.Equal(t, n3.total, n8.total, "not-due sessions issue no statement")
}

type autoCheckoutStatements struct {
	balanceLocks, rowLocks, total int
}

func runAutoCheckoutWithNotDueSessions(t *testing.T, notDue int) autoCheckoutStatements {
	t.Helper()
	var counted autoCheckoutStatements
	t.Run(fmt.Sprintf("not_due_%d", notDue), func(t *testing.T) {
		db := testpkg.SetupTestDB(t)
		ctx := testpkg.OwnCtx(t)
		workTime := owners.MustNewWorkforce(testpkg.NewDatabase(t, db))
		openSession := func(staffID int64, checkInHour int) *timetracking.WorkSession {
			session := &timetracking.WorkSession{
				StaffID: staffID, Date: autoCheckoutDay,
				Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
				CheckInTime: autoCheckoutDay.BerlinMidnight().Add(time.Duration(checkInHour) * time.Hour),
				CreatedBy:   staffID,
			}
			session.SetTenantID(testpkg.Tenant(t))
			createSessionFixture(t, workTime, ctx, session)
			testpkg.CreateTestStaffShift(t, db, staffID, autoCheckoutDay, testpkg.StaffShiftOpts{})
			return session
		}
		dueStaff := testpkg.CreateTestStaff(t, db, "Auto", "Faellig")
		due := openSession(dueStaff.ID, 8)
		for i := range notDue {
			// Checked in at 17:00, after the shift ended: never closed
			// against that shift end.
			staff := testpkg.CreateTestStaff(t, db, "Auto", fmt.Sprintf("Spaet-%d", i))
			openSession(staff.ID, 17)
		}

		service := newAutoCheckoutService(t, db, workTime)
		counter := testpkg.CaptureQueriesForContext(t, db)
		var closed int
		err := testpkg.WithinTenantContext(t, counter.Context(ctx), db, testpkg.Tenant(t), func(txCtx context.Context) error {
			var runErr error
			closed, runErr = service.AutoCheckoutDueSessions(txCtx, 15*time.Minute)
			return runErr
		})
		counter.Stop()
		require.NoError(t, err)
		assert.Equal(t, 1, closed)

		sessions, err := workTime.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffID: dueStaff.ID})
		require.NoError(t, err)
		require.Len(t, sessions, 1)
		require.Equal(t, due.ID, sessions[0].ID)
		assert.NotNil(t, sessions[0].CheckOutTime, "the due session is checked out")
		assert.True(t, sessions[0].AutoCheckedOut)

		counted = autoCheckoutStatements{
			balanceLocks: len(counter.Matching(func(sql string) bool { return strings.Contains(sql, "pg_advisory_xact_lock") })),
			rowLocks:     len(counter.Matching(func(sql string) bool { return strings.Contains(sql, "for update") })),
			total:        counter.Total(),
		}
	})
	return counted
}

func newAutoCheckoutService(t *testing.T, db *testpkg.DB, workTime workforceCompose.WorkTime) timetracking.WorkSessionService {
	t.Helper()
	return newWorkSessionTestService(t, db, workTime, workTime, api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db)).GroupSupervisor)
}
