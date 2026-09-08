package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The work-session family (#2690): active.work_sessions,
// active.work_session_breaks, active.staff_balance_adjustments,
// active.staff_vacation_openings and active.staff_vacation_quota. The tests
// below carry over the row rules and live-window behavior the deleted
// repository tests covered, and prove tenant isolation and rollback through
// the public capability.

// buildWorkforceAt composes the module over a pinned clock so the live window
// and "today" are deterministic.
func buildWorkforceAt(t *testing.T, db *bun.DB, now time.Time) workforce.Capability {
	t.Helper()
	runtime := testpkg.ConfigRuntime(db)
	capability, err := New(Dependencies{
		DB:                db,
		AssignedStaffIDs:  runtime.AssignedStaffIDs,
		RebaseStaffAnchor: runtime.RebaseAssignedStaffAnchor,
		Observe:           func(Observation) {},
		Now:               func() time.Time { return now },
	})
	require.NoError(t, err)
	return capability
}

func testWorkSession(staffID int64, day timezone.Date, checkIn time.Time) workforce.WorkSession {
	return workforce.WorkSession{
		StaffID: staffID, Date: day.String(), Status: workforce.WorkSessionStatusPresent,
		Source: workforce.WorkSessionSourceApp, CheckInTime: checkIn, CreatedBy: staffID,
	}
}

func TestWorkSessionRowRulesMatchTheLegacyRepository(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Session", "Rules")
	capability := buildWorkforce(t, db)
	today := timezone.TodayDate()

	created, err := capability.CreateWorkSession(ctx, testWorkSession(staff.ID, today, time.Now()))
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID, "the ambient tenant is stamped on the row")
	assert.Equal(t, today.String(), created.Date)
	assert.True(t, created.IsOpen())

	homeOffice := testWorkSession(staff.ID, today, time.Now())
	homeOffice.Status = workforce.WorkSessionStatusHomeOffice
	// A second open block on the same day trips the unique index and surfaces
	// as the conflict the kiosk classifies, with the driver error reachable.
	_, err = capability.CreateWorkSession(ctx, homeOffice)
	require.ErrorIs(t, err, workforce.ErrWorkSessionAlreadyOpen)
	var conflict *workforce.ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Error(t, conflict.Cause, "the driver error stays reachable for constraint inspection")

	for name, broken := range map[string]func(*workforce.WorkSession){
		"status must be 'present' or 'home_office'": func(ws *workforce.WorkSession) { ws.Status = "invalid_status" },
		"staff ID is required":                      func(ws *workforce.WorkSession) { ws.StaffID = 0 },
		"check-in time is required":                 func(ws *workforce.WorkSession) { ws.CheckInTime = time.Time{} },
		"break minutes cannot be negative":          func(ws *workforce.WorkSession) { ws.BreakMinutes = -1 },
		"created_by is required":                    func(ws *workforce.WorkSession) { ws.CreatedBy = 0 },
	} {
		value := testWorkSession(staff.ID, today.AddDays(-30), time.Now())
		broken(&value)
		_, err := capability.CreateWorkSession(ctx, value)
		require.ErrorIs(t, err, workforce.ErrInvalidWorkSession, name)
		assert.EqualError(t, err, name)
	}

	// Closing stamps the check-out once; a second close is a no-op.
	checkOut := time.Now()
	closed, err := capability.CloseWorkSession(ctx, created.ID, checkOut, true)
	require.NoError(t, err)
	assert.True(t, closed)
	closed, err = capability.CloseWorkSession(ctx, created.ID, checkOut.Add(time.Hour), false)
	require.NoError(t, err)
	assert.False(t, closed, "an already closed block is not closed again")
	stored, err := capability.FindWorkSession(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.CheckOutTime)
	assert.WithinDuration(t, checkOut, *stored.CheckOutTime, time.Second)
	assert.True(t, stored.AutoCheckedOut)

	// The break cache is rewritten in place; a foreign tenant cannot reach it.
	affected, err := capability.SetWorkSessionBreakMinutes(ctx, created.ID, 30)
	require.NoError(t, err)
	assert.EqualValues(t, 1, affected)
	otherTenant := testpkg.NewTenantScope(t, db)
	affected, err = capability.SetWorkSessionBreakMinutes(otherTenant.Context(), created.ID, 45)
	require.NoError(t, err)
	assert.Zero(t, affected, "a foreign tenant updates nothing")
	stored, err = capability.FindWorkSession(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 30, stored.BreakMinutes)

	// Listing by staff and day is the ordinary filter path; an unknown day is
	// empty, not an error.
	listed, err := capability.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffID: staff.ID, Date: today.String()})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)
	listed, err = capability.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffID: staff.ID, Date: today.AddDays(-400).String()})
	require.NoError(t, err)
	assert.Empty(t, listed)
	count, err := capability.CountWorkSessions(ctx, workforce.WorkSessionFilter{StaffID: staff.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	_, err = capability.FindWorkSession(ctx, created.ID+1_000_000)
	require.ErrorIs(t, err, workforce.ErrWorkSessionNotFound)
}

func TestWorkSessionHistoryAndOpenSessionsFollowTheStoredDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "History", "Staff")
	capability := buildWorkforce(t, db)
	today := timezone.TodayDate()

	for _, offset := range []int{-2, -1, 0} {
		day := today.AddDays(offset)
		checkIn := day.BerlinMidnight().Add(8 * time.Hour)
		session := testWorkSession(staff.ID, day, checkIn)
		if offset != 0 {
			checkOut := checkIn.Add(6 * time.Hour)
			session.CheckOutTime = &checkOut
		}
		_, err := capability.CreateWorkSession(ctx, session)
		require.NoError(t, err)
	}

	history, err := capability.ListWorkSessions(ctx, workforce.WorkSessionFilter{
		StaffID: staff.ID, DateFrom: today.AddDays(-1).String(), DateTo: today.String(),
		Order: []workforce.WorkSessionOrder{{Field: workforce.WorkSessionOrderDate}, {Field: workforce.WorkSessionOrderCheckInTime}},
	})
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, today.AddDays(-1).String(), history[0].Date)
	assert.Equal(t, today.String(), history[1].Date)

	open := true
	stillOpen, err := capability.ListWorkSessions(ctx, workforce.WorkSessionFilter{DateBefore: today.AddDays(1).String(), Open: &open, StaffID: staff.ID})
	require.NoError(t, err)
	require.Len(t, stillOpen, 1)
	assert.Equal(t, today.String(), stillOpen[0].Date)

	oldest, err := capability.OldestWorkSessionDate(ctx, workforce.WorkSessionDateColumn, today.String())
	require.NoError(t, err)
	assert.Equal(t, today.AddDays(-2).String(), oldest)
	oldest, err = capability.OldestWorkSessionDate(ctx, workforce.WorkSessionDateColumn, today.AddDays(-2).String())
	require.NoError(t, err)
	assert.Empty(t, oldest, "no block before the earliest day")

	deleted, err := capability.DeleteWorkSessionsOlderThan(ctx, workforce.WorkSessionDateColumn, today.AddDays(-1).String())
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	_, err = capability.DeleteWorkSessionsOlderThan(ctx, "check_in_time", today.String())
	require.ErrorIs(t, err, workforce.ErrInvalidWorkSession, "only the date column may drive retention")
}

// GetLatestOpen answers "is this person clocked in right now" with the same
// live limit the balance applies (#2402): a block that crossed Berlin midnight
// is still running, a checkout that never happened is not.
func TestLatestOpenWorkSessionAppliesTheLiveWindow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	today := timezone.DateFromTime(now)
	capability := buildWorkforceAt(t, db, now)

	night := testpkg.CreateTestStaff(t, db, "Night", "Staff")
	nightBlock, err := capability.CreateWorkSession(ctx, testWorkSession(night.ID, today.AddDays(-1), now.Add(-3*time.Hour)))
	require.NoError(t, err)
	found, err := capability.LatestOpenWorkSession(ctx, night.ID)
	require.NoError(t, err)
	assert.Equal(t, nightBlock.ID, found.ID, "a block that crossed midnight is still running")

	forgot := testpkg.CreateTestStaff(t, db, "Forgot", "Staff")
	threeDaysAgo := today.AddDays(-3)
	_, err = capability.CreateWorkSession(ctx, testWorkSession(forgot.ID, threeDaysAgo, threeDaysAgo.BerlinMidnight().Add(8*time.Hour)))
	require.NoError(t, err)
	_, err = capability.LatestOpenWorkSession(ctx, forgot.ID)
	require.ErrorIs(t, err, workforce.ErrWorkSessionNotFound, "an expired block is not a running one")

	long := testpkg.CreateTestStaff(t, db, "Long", "Staff")
	longBlock, err := capability.CreateWorkSession(ctx, testWorkSession(long.ID, today, today.BerlinMidnight()))
	require.NoError(t, err)
	found, err = capability.LatestOpenWorkSession(ctx, long.ID)
	require.NoError(t, err, "a long shift on today is not a mistake")
	assert.Equal(t, longBlock.ID, found.ID)

	todayOpen, err := capability.TodayOpenWorkSession(ctx, long.ID)
	require.NoError(t, err)
	assert.Equal(t, longBlock.ID, todayOpen.ID)
	_, err = capability.TodayOpenWorkSession(ctx, night.ID)
	require.ErrorIs(t, err, workforce.ErrWorkSessionNotFound, "the night block is filed on yesterday")

	// The presence map reports the night block's owner present, the forgotten
	// checkout absent, and a closed block of today as checked out.
	closedStaff := testpkg.CreateTestStaff(t, db, "Closed", "Staff")
	closedSession := testWorkSession(closedStaff.ID, today, now.Add(-2*time.Hour))
	closedSession.Status = workforce.WorkSessionStatusHomeOffice
	checkOut := now
	closedSession.CheckOutTime = &checkOut
	_, err = capability.CreateWorkSession(ctx, closedSession)
	require.NoError(t, err)
	presence, err := capability.WorkPresenceMap(ctx)
	require.NoError(t, err)
	assert.Equal(t, workforce.WorkSessionStatusPresent, presence[night.ID])
	assert.Equal(t, workforce.WorkSessionStatusPresent, presence[long.ID])
	assert.Equal(t, "checked_out", presence[closedStaff.ID])
	_, listed := presence[forgot.ID]
	assert.False(t, listed, "a stale open block must not report presence")

	// Locking the open block returns it; a closed block is not lockable.
	locked, err := capability.LockOpenWorkSession(ctx, longBlock.ID)
	require.NoError(t, err)
	assert.Equal(t, longBlock.ID, locked.ID)
	lockedOn, err := capability.LockOpenWorkSessionOn(ctx, long.ID, today.String())
	require.NoError(t, err)
	assert.Equal(t, longBlock.ID, lockedOn.ID)
	_, err = capability.LockOpenWorkSession(ctx, nightBlock.ID+1_000_000)
	require.ErrorIs(t, err, workforce.ErrWorkSessionNotFound)
}

// The overlapping read bounds `from` against check_out_time, never against the
// stored date or the check-in: a block that began days earlier and ends inside
// the range is part of the answer (#2402).
func TestOverlappingWorkSessionsKeepEarlierStarts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Early", "Staff")
	capability := buildWorkforce(t, db)

	from := timezone.NewDate(2026, 8, 24)
	to := from.AddDays(6)
	start := from.AddDays(-5)
	reaching := testWorkSession(staff.ID, start, start.BerlinMidnight().Add(8*time.Hour))
	checkOut := from.BerlinMidnight().Add(10 * time.Hour)
	reaching.CheckOutTime = &checkOut
	reachingRow, err := capability.CreateWorkSession(ctx, reaching)
	require.NoError(t, err)

	past := testWorkSession(staff.ID, start, start.BerlinMidnight().Add(9*time.Hour))
	endedBefore := start.BerlinMidnight().Add(12 * time.Hour)
	past.CheckOutTime = &endedBefore
	pastRow, err := capability.CreateWorkSession(ctx, past)
	require.NoError(t, err)

	rangeEnd := to.AddDays(1).BerlinMidnight()
	found, err := capability.ListOverlappingWorkSessions(ctx, []int64{staff.ID}, from.BerlinMidnight(), &rangeEnd)
	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, session := range found {
		ids = append(ids, session.ID)
	}
	assert.Contains(t, ids, reachingRow.ID, "a block ending inside the range belongs to it, however early it started")
	assert.NotContains(t, ids, pastRow.ID, "a block that ended before the range does not")

	none, err := capability.ListOverlappingWorkSessions(ctx, nil, from.BerlinMidnight(), &rangeEnd)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestWorkSessionBreaksFollowTheLegacyRowRules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Break", "Staff")
	capability := buildWorkforce(t, db)
	today := timezone.TodayDate()

	session, err := capability.CreateWorkSession(ctx, testWorkSession(staff.ID, today, time.Now().Add(-3*time.Hour)))
	require.NoError(t, err)

	_, err = capability.CreateWorkSessionBreak(ctx, workforce.WorkSessionBreak{SessionID: 0, StartedAt: time.Now()})
	require.ErrorIs(t, err, workforce.ErrInvalidWorkSession)
	assert.EqualError(t, err, "session ID is required")

	first, err := capability.CreateWorkSessionBreak(ctx, workforce.WorkSessionBreak{
		SessionID: session.ID, StartedAt: time.Now().Add(-2 * time.Hour),
	})
	require.NoError(t, err)
	assert.NotZero(t, first.ID)
	assert.Equal(t, testpkg.Tenant(t), first.TenantID)
	assert.True(t, first.IsActive())

	active := true
	running, err := capability.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: session.ID, Active: &active})
	require.NoError(t, err)
	require.Len(t, running, 1)
	assert.Equal(t, first.ID, running[0].ID)

	// Ending stamps a running break exactly once; the stale second end is
	// rejected and leaves the first stamp in place.
	endedAt := time.Now().Add(-90 * time.Minute)
	affected, err := capability.EndWorkSessionBreak(ctx, first.ID, endedAt, 30)
	require.NoError(t, err)
	assert.EqualValues(t, 1, affected)
	affected, err = capability.EndWorkSessionBreak(ctx, first.ID, endedAt.Add(time.Minute), 31)
	require.NoError(t, err)
	assert.Zero(t, affected)
	stored, err := capability.FindWorkSessionBreak(ctx, first.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.EndedAt)
	assert.WithinDuration(t, endedAt, *stored.EndedAt, time.Second)
	assert.Equal(t, 30, stored.DurationMinutes)

	// An admin correction rewrites length and end regardless of state; a
	// missing break is not an error, just nothing to rewrite.
	newEnd := time.Now().Add(-80 * time.Minute)
	affected, err = capability.SetWorkSessionBreakDuration(ctx, first.ID, 45, newEnd)
	require.NoError(t, err)
	assert.EqualValues(t, 1, affected)
	affected, err = capability.SetWorkSessionBreakDuration(ctx, first.ID+1_000_000, 45, newEnd)
	require.NoError(t, err)
	assert.Zero(t, affected)
	stored, err = capability.FindWorkSessionBreak(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, stored.DurationMinutes)
	assert.WithinDuration(t, newEnd, *stored.EndedAt, time.Second)

	// A planned break that ran over is an expired one; the ordinary listing
	// groups breaks by session in start order.
	plannedEnd := time.Now().Add(-10 * time.Minute)
	second, err := capability.CreateWorkSessionBreak(ctx, workforce.WorkSessionBreak{
		SessionID: session.ID, StartedAt: time.Now().Add(-40 * time.Minute), PlannedEndTime: &plannedEnd,
	})
	require.NoError(t, err)
	expired, err := capability.ExpiredWorkSessionBreaks(ctx, time.Now())
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, second.ID, expired[0].ID)
	all, err := capability.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionIDs: []int64{session.ID}})
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, first.ID, all[0].ID)
	assert.Equal(t, second.ID, all[1].ID)
	paged, err := capability.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: session.ID, Limit: 1})
	require.NoError(t, err)
	assert.Len(t, paged, 1)

	require.NoError(t, capability.DeleteWorkSessionBreak(ctx, second.ID))
	_, err = capability.FindWorkSessionBreak(ctx, second.ID)
	require.ErrorIs(t, err, workforce.ErrWorkSessionBreakNotFound)
}

func TestBalanceAndVacationRowsAreTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	local := testpkg.CreateTestStaff(t, db, "Balance", "Local")
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Balance", "Foreign")
	capability := buildWorkforce(t, db)

	// --- adjustments ---
	adjustment, err := capability.CreateStaffBalanceAdjustment(ctx, workforce.StaffBalanceAdjustment{
		StaffID: local.ID, Type: workforce.BalanceAdjustmentTypePayout, MinutesDelta: -60,
		EffectiveDate: "2026-03-02", Note: "Auszahlung", DecidedBy: local.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), adjustment.TenantID)
	foreignAdjustment, err := capability.CreateStaffBalanceAdjustment(foreignCtx, workforce.StaffBalanceAdjustment{
		StaffID: foreign.ID, Type: workforce.BalanceAdjustmentTypeReset, MinutesDelta: 0,
		EffectiveDate: "2026-03-02", DecidedBy: foreign.ID,
	})
	require.NoError(t, err)
	_, err = capability.CreateStaffBalanceAdjustment(ctx, workforce.StaffBalanceAdjustment{
		StaffID: local.ID, Type: "bonus", EffectiveDate: "2026-03-02", DecidedBy: local.ID,
	})
	require.ErrorIs(t, err, workforce.ErrInvalidWorkSession)
	assert.EqualError(t, err, "invalid balance adjustment type")

	listed, err := capability.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{
		StaffIDs: []int64{local.ID, foreign.ID}, EffectiveFrom: "2026-03-01", EffectiveTo: "2026-03-31",
	})
	require.NoError(t, err)
	require.Len(t, listed, 1, "the foreign tenant's row is invisible")
	assert.Equal(t, adjustment.ID, listed[0].ID)
	_, err = capability.FindStaffBalanceAdjustment(ctx, foreignAdjustment.ID)
	require.ErrorIs(t, err, workforce.ErrStaffBalanceAdjustmentNotFound)
	require.NoError(t, capability.DeleteStaffBalanceAdjustment(ctx, foreignAdjustment.ID))
	_, err = capability.FindStaffBalanceAdjustment(foreignCtx, foreignAdjustment.ID)
	require.NoError(t, err, "a foreign delete is a no-op")

	byType, err := capability.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{
		StaffID: local.ID, Types: []string{workforce.BalanceAdjustmentTypeReset, workforce.BalanceAdjustmentTypeOpening},
	})
	require.NoError(t, err)
	assert.Empty(t, byType)

	// --- vacation openings ---
	opening, err := capability.CreateStaffVacationOpening(ctx, workforce.StaffVacationOpening{
		StaffID: local.ID, Year: 2026, EffectiveDate: "2026-04-01", TakenBeforeDays: 3.5, EnteredRemainingDays: 26.5,
		Note: "Übernahme", DecidedBy: local.ID,
	})
	require.NoError(t, err)
	_, err = capability.CreateStaffVacationOpening(foreignCtx, workforce.StaffVacationOpening{
		StaffID: foreign.ID, Year: 2026, EffectiveDate: "2026-04-01", TakenBeforeDays: 1, EnteredRemainingDays: 29, DecidedBy: foreign.ID,
	})
	require.NoError(t, err)
	_, err = capability.CreateStaffVacationOpening(ctx, workforce.StaffVacationOpening{
		StaffID: local.ID, Year: 2026, EffectiveDate: "2025-12-31", TakenBeforeDays: 1, EnteredRemainingDays: 29, DecidedBy: local.ID,
	})
	require.ErrorIs(t, err, workforce.ErrInvalidWorkSession)
	assert.EqualError(t, err, "effective_date must lie in the opening year")
	openings, err := capability.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffIDs: []int64{local.ID, foreign.ID}, Year: 2026})
	require.NoError(t, err)
	require.Len(t, openings, 1)
	assert.Equal(t, opening.ID, openings[0].ID)
	assert.InDelta(t, 3.5, openings[0].TakenBeforeDays, 0.001)

	// --- vacation quota ---
	require.NoError(t, capability.UpsertStaffVacationQuota(ctx, workforce.StaffVacationQuota{StaffID: local.ID, Year: 2026, EntitledDays: 29}))
	require.NoError(t, capability.UpsertStaffVacationQuota(ctx, workforce.StaffVacationQuota{StaffID: local.ID, Year: 2026, EntitledDays: 31, CarryoverDays: 2}))
	require.NoError(t, capability.UpsertStaffVacationQuota(foreignCtx, workforce.StaffVacationQuota{StaffID: foreign.ID, Year: 2026, EntitledDays: 25}))
	err = capability.UpsertStaffVacationQuota(ctx, workforce.StaffVacationQuota{StaffID: local.ID, Year: 2026, EntitledDays: 30.25})
	require.ErrorIs(t, err, workforce.ErrInvalidWorkSession)
	assert.EqualError(t, err, "vacation quota days must have at most one decimal place")

	quotas, err := capability.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{
		Year: 2026, Order: []workforce.StaffVacationOrder{{Field: workforce.StaffVacationOrderEntitledDays, Descending: true}}, Limit: 1,
	})
	require.NoError(t, err)
	require.Len(t, quotas, 1, "the foreign tenant's quota is invisible and the upsert replaced the day counts")
	assert.Equal(t, local.ID, quotas[0].StaffID)
	assert.InDelta(t, 31, quotas[0].EntitledDays, 0.001)
	assert.InDelta(t, 2, quotas[0].CarryoverDays, 0.001)
	empty, err := capability.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{Year: 2099})
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// Every work-session writer queues on the staff balance lock, so a session
// write cannot interleave with an adjustment or an effective absence of the
// same staff member.
func TestWorkSessionWritersShareTheBalanceLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Balance", "Lock")
	runtime := testpkg.ConfigRuntime(db)
	capability := buildWorkforce(t, db)

	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- testpkg.WithinTenantContext(t, context.Background(), db, staff.TenantID, func(ctx context.Context) error {
			if err := runtime.LockStaffBalance(ctx, staff.ID); err != nil {
				return err
			}
			close(lockHeld)
			<-releaseLock
			return nil
		})
	}()

	select {
	case <-lockHeld:
	case <-time.After(5 * time.Second):
		close(releaseLock)
		require.FailNow(t, "adjustment writer did not acquire the balance lock")
	}

	writerDone := make(chan error, 1)
	go func() {
		writerDone <- testpkg.WithinTenantContext(t, context.Background(), db, staff.TenantID, func(ctx context.Context) error {
			return capability.LockStaffBalanceWrites(ctx, staff.ID)
		})
	}()

	select {
	case err := <-writerDone:
		close(releaseLock)
		require.NoError(t, <-holderDone)
		require.Failf(t, "work session writer bypassed shared balance lock", "returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseLock)
	require.NoError(t, <-holderDone)
	require.NoError(t, <-writerDone)
}

// A stamp writes the block and its break on the caller's transaction. A
// failure after either write rolls both back, and the same stamp retries
// cleanly.
func TestWorkSessionAndBreakRollBackWithTheCallerTransaction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rollback", "Stamp")
	capability := buildWorkforce(t, db)
	day := timezone.NewDate(2026, 5, 4)
	failure := errors.New("stamp aborted after an authoritative write")

	stamp := func(txCtx context.Context, failAfter string) (int64, error) {
		session, err := capability.CreateWorkSession(txCtx, testWorkSession(staff.ID, day, day.BerlinMidnight().Add(8*time.Hour)))
		if err != nil {
			return 0, err
		}
		if failAfter == "session" {
			return session.ID, failure
		}
		if _, err := capability.CreateWorkSessionBreak(txCtx, workforce.WorkSessionBreak{
			SessionID: session.ID, StartedAt: day.BerlinMidnight().Add(12 * time.Hour),
		}); err != nil {
			return 0, err
		}
		if failAfter == "break" {
			return session.ID, failure
		}
		if _, err := capability.SetWorkSessionBreakMinutes(txCtx, session.ID, 15); err != nil {
			return 0, err
		}
		if failAfter == "minutes" {
			return session.ID, failure
		}
		return session.ID, nil
	}

	for _, failAfter := range []string{"session", "break", "minutes"} {
		var failedID int64
		err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
			id, err := stamp(txCtx, failAfter)
			failedID = id
			return err
		})
		require.ErrorIs(t, err, failure, failAfter)
		require.NotZero(t, failedID, "the writes happened inside the transaction before the failure")

		_, err = capability.FindWorkSession(ctx, failedID)
		require.ErrorIs(t, err, workforce.ErrWorkSessionNotFound, "the block must not survive the rollback after "+failAfter)
		breaks, err := capability.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: failedID})
		require.NoError(t, err)
		assert.Empty(t, breaks, "the break must not survive the rollback after "+failAfter)
	}

	var retriedID int64
	err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		id, err := stamp(txCtx, "")
		retriedID = id
		return err
	})
	require.NoError(t, err)
	retried, err := capability.FindWorkSession(ctx, retriedID)
	require.NoError(t, err)
	assert.Equal(t, 15, retried.BreakMinutes)
	breaks, err := capability.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: retriedID})
	require.NoError(t, err)
	assert.Len(t, breaks, 1)
}

func TestWorkSessionsAreTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	local := testpkg.CreateTestStaff(t, db, "Session", "Local")
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Session", "Foreign")
	capability := buildWorkforce(t, db)
	today := timezone.TodayDate()

	localSession, err := capability.CreateWorkSession(ctx, testWorkSession(local.ID, today, time.Now().Add(-time.Hour)))
	require.NoError(t, err)
	foreignSession, err := capability.CreateWorkSession(foreignCtx, testWorkSession(foreign.ID, today, time.Now().Add(-time.Hour)))
	require.NoError(t, err)
	assert.Equal(t, foreignTenantID, foreignSession.TenantID)
	foreignBreak, err := capability.CreateWorkSessionBreak(foreignCtx, workforce.WorkSessionBreak{SessionID: foreignSession.ID, StartedAt: time.Now()})
	require.NoError(t, err)

	_, err = capability.FindWorkSession(ctx, foreignSession.ID)
	require.ErrorIs(t, err, workforce.ErrWorkSessionNotFound, "a foreign row is invisible, not an error of another kind")
	_, err = capability.FindWorkSessionBreak(ctx, foreignBreak.ID)
	require.ErrorIs(t, err, workforce.ErrWorkSessionBreakNotFound)

	listed, err := capability.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffIDs: []int64{local.ID, foreign.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, localSession.ID, listed[0].ID)

	presence, err := capability.WorkPresenceMap(ctx)
	require.NoError(t, err)
	assert.Contains(t, presence, local.ID)
	assert.NotContains(t, presence, foreign.ID)

	closed, err := capability.CloseWorkSession(ctx, foreignSession.ID, time.Now(), false)
	require.NoError(t, err)
	assert.False(t, closed, "a foreign block cannot be closed")
	affected, err := capability.EndWorkSessionBreak(ctx, foreignBreak.ID, time.Now(), 5)
	require.NoError(t, err)
	assert.Zero(t, affected, "a foreign break cannot be ended")

	foreignSession.Notes = "hijacked"
	_, err = capability.UpdateWorkSession(ctx, foreignSession)
	require.ErrorIs(t, err, workforce.ErrWorkSessionNotFound, "a foreign update matches no row")
	require.NoError(t, capability.DeleteWorkSession(ctx, foreignSession.ID))
	still, err := capability.FindWorkSession(foreignCtx, foreignSession.ID)
	require.NoError(t, err, "a foreign delete is a no-op")
	assert.True(t, still.IsOpen())
}
