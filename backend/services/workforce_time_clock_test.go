package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The time-clock adapter serves the kiosk's day view from the retained work
// session service. These tests pin the two rules the kiosk depends on: an
// open block keeps counting across Berlin midnight, and a block that passed
// its live limit is rendered closed and drops out of later days.

var berlin = mustBerlin()

func mustBerlin() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return location
}

func TestTimeClockWorkday_OpenNightSessionCountsThroughMidnight(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Night", "Shift")
	module, err := NewWorkSessionTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	clock := TimeClockCapability(module.WorkSession)

	openedAt := time.Date(2026, 3, 9, 22, 30, 0, 0, berlin)
	_, err = db.ExecContext(ctx, `INSERT INTO active.work_sessions
		(tenant_id, staff_id, date, status, source, check_in_time, break_minutes, notes, auto_checked_out, created_by)
		VALUES (?, ?, ?, 'present', 'nfc', ?, 0, '', false, ?)`,
		testpkg.Tenant(t), staff.ID, "2026-03-09", openedAt, staff.ID)
	require.NoError(t, err)

	// 01:30 the next day: the block is still inside its live window.
	workday, err := clock.Workday(ctx, staff.ID, "2026-03-09", openedAt.Add(3*time.Hour))
	require.NoError(t, err)
	require.Len(t, workday.Sessions, 1)
	assert.True(t, workday.Sessions[0].IsOpen(), "a block inside its live window stays open")
	assert.Equal(t, "2026-03-09", workday.Sessions[0].Date)
	assert.Equal(t, 180, workday.LaborTime.NetMinutes, "the elapsed work counts through midnight")
}

func TestTimeClockWorkday_ExpiredOpenBlockIsClosedAndLeavesLaterDays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Forgotten", "Checkout")
	module, err := NewWorkSessionTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	clock := TimeClockCapability(module.WorkSession)

	openedAt := time.Date(2026, 3, 9, 20, 0, 0, 0, berlin)
	_, err = db.ExecContext(ctx, `INSERT INTO active.work_sessions
		(tenant_id, staff_id, date, status, source, check_in_time, break_minutes, notes, auto_checked_out, created_by)
		VALUES (?, ?, ?, 'present', 'nfc', ?, 0, '', false, ?)`,
		testpkg.Tenant(t), staff.ID, "2026-03-09", openedAt, staff.ID)
	require.NoError(t, err)

	// 10:00 the next day: the twelve-hour live limit passed at 08:00, which is
	// still inside that day, so the block shows up closed at that instant.
	now := time.Date(2026, 3, 10, 10, 0, 0, 0, berlin)
	workday, err := clock.Workday(ctx, staff.ID, "2026-03-10", now)
	require.NoError(t, err)
	require.Len(t, workday.Sessions, 1)
	require.NotNil(t, workday.Sessions[0].CheckOutTime, "the stale block is cut at its live limit")
	assert.True(t, workday.Sessions[0].CheckOutTime.Equal(openedAt.Add(12*time.Hour)))
	assert.False(t, workday.Sessions[0].IsOpen())

	// The stored row still hangs open; the day after the limit no longer sees it.
	later, err := clock.Workday(ctx, staff.ID, "2026-03-11", now.Add(24*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, later.Sessions, "a block that expired before the day began is not part of it")

	// Nor is it the running block the kiosk resolves its day from.
	_, found, err := clock.LatestOpenSession(ctx, staff.ID)
	require.NoError(t, err)
	assert.False(t, found, "an expired block is not offered for checkout")
}
