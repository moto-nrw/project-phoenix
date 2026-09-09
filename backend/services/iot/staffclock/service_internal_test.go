package staffclock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The workflow is exercised over fakes of its ports: the card and staff
// lookups, the time clock and the clock. Dates are Berlin calendar days,
// which is what the production clock adapter derives too.

var berlin = mustLoadBerlin()

func mustLoadBerlin() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return location
}

const (
	testTag     = "A1654BEEF"
	testStaffID = int64(7)
)

type stubCards struct{ card *Card }

func (s *stubCards) FindCard(context.Context, string) (*Card, error) { return s.card, nil }

type stubStaff struct {
	identity StaffIdentity
	err      error
}

func (s *stubStaff) ResolveStaffByTag(context.Context, string) (StaffIdentity, error) {
	return s.identity, s.err
}

// fakeClock is the instant the workflow reads and the Berlin day it derives.
// The time clock stub reads the same clock: both read the time independently
// in production too, and the midnight cases only mean anything when the stub
// can drift with the workflow.
type fakeClock struct{ now func() time.Time }

func (c *fakeClock) Now() time.Time               { return c.now() }
func (c *fakeClock) Day(instant time.Time) string { return berlinDay(instant) }

func berlinDay(instant time.Time) string { return instant.In(berlin).Format("2006-01-02") }

// stubTimeClock mirrors the parts of the work session service the workflow
// depends on: the pinned day of every action, the day a fresh stamp is filed
// on, and the interval read that has no upper bound for an open block.
type stubTimeClock struct {
	clock       *fakeClock
	checkInErr  error
	checkInCall int
	// checkInDays records the calendar day every check-in was pinned to.
	checkInDays []string
	// latestOpen is the still-running session the day resolution starts from.
	latestOpen *Session
	// actionDays records the calendar day every mutating action was pinned to.
	actionDays []string
	// openSessionByDay mirrors the day-scoped lookup inside the service: an
	// action only finds the session on the day it was written on.
	openSessionByDay map[string]Session
	// workdays is what the interval read returns per day; workdayNow records
	// the instant the labor-time figures were requested for.
	workdays   map[string]Workday
	workdayNow []time.Time
	// intersecting, when set, comes back for EVERY day: an open block has no
	// upper bound.
	intersecting *Workday
}

func (s *stubTimeClock) CheckIn(_ context.Context, stamp Stamp) (Session, error) {
	s.checkInCall++
	s.checkInDays = append(s.checkInDays, stamp.Day)
	if s.checkInErr != nil && s.checkInCall == 1 {
		return Session{}, s.checkInErr
	}
	if existing, ok := s.openSessionByDay[stamp.Day]; ok {
		return existing, nil
	}
	// The pinned day selects the open block to act on, but a session created
	// fresh carries the day of its own stamp.
	stampedAt := s.clock.Now()
	return Session{StaffID: stamp.StaffID, Day: berlinDay(stampedAt), Status: stamp.Status, Source: SourceNFC, CheckInTime: stampedAt}, nil
}

// openOn is the stand-in for the day-pinned lookup the real service performs.
func (s *stubTimeClock) openOn(day string) (Session, error) {
	s.actionDays = append(s.actionDays, day)
	session, ok := s.openSessionByDay[day]
	if !ok {
		return Session{}, &StampError{Kind: ErrStateConflict, Cause: errors.New("no active session found")}
	}
	return session, nil
}

func (s *stubTimeClock) CheckOutOn(_ context.Context, _ int64, day, _ string) (Session, error) {
	return s.openOn(day)
}

func (s *stubTimeClock) StartBreakOn(_ context.Context, _ int64, day string, _ *int) error {
	_, err := s.openOn(day)
	return err
}

func (s *stubTimeClock) EndBreakOn(_ context.Context, _ int64, day string) (Session, error) {
	return s.openOn(day)
}

func (s *stubTimeClock) LatestOpenSession(context.Context, int64) (Session, bool, error) {
	if s.latestOpen == nil {
		return Session{}, false, nil
	}
	return *s.latestOpen, true, nil
}

func (s *stubTimeClock) Workday(_ context.Context, _ int64, day string, now time.Time) (Workday, error) {
	s.workdayNow = append(s.workdayNow, now)
	if s.intersecting != nil {
		return *s.intersecting, nil
	}
	if workday, ok := s.workdays[day]; ok {
		return workday, nil
	}
	return Workday{}, nil
}

type harness struct {
	service *Service
	clock   *fakeClock
	stamps  *stubTimeClock
}

func newHarness(checkInErr error) harness {
	clock := &fakeClock{now: func() time.Time { return time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC) }}
	stamps := &stubTimeClock{clock: clock, checkInErr: checkInErr}
	service := NewService(Dependencies{
		Cards:     &stubCards{card: &Card{ID: testTag, Active: true}},
		Staff:     &stubStaff{identity: StaffIdentity{StaffID: testStaffID, FullName: "Nora Kiosk"}},
		TimeClock: stamps,
		Clock:     clock,
	})
	return harness{service: service, clock: clock, stamps: stamps}
}

func checkInCommand() StaffClockCommand {
	return StaffClockCommand{RFIDTag: testTag, Action: ActionCheckIn, Status: StatusPresent}
}

// workdayOf renders a day carrying exactly the given block.
func workdayOf(session Session, breaks ...Break) Workday {
	return Workday{Sessions: []Session{session}, Breaks: map[int64][]Break{session.ID: breaks}, IsBreakCompliant: true}
}

// Two kiosks scanning the same card at the same moment both see "no session
// today" and both insert. The loser must learn it lost the race, not that the
// server broke.
func TestExecute_ConcurrentCheckInReportsStateConflict(t *testing.T) {
	t.Parallel()
	h := newHarness(&StampError{Kind: ErrCheckInRaced, Cause: errors.New("duplicate key")})

	state, err := h.service.ExecuteStaffClock(context.Background(), checkInCommand())

	require.ErrorIs(t, err, ErrCheckInRaced)
	assert.Nil(t, state)
	assert.Equal(t, 1, h.stamps.checkInCall)
}

// A check-in that straddles Berlin midnight is persisted on the day the write
// happened, and that day decides what is read back. Anchoring the reload to
// the request clock reported "checked_out, no session" for a successful stamp.
func TestExecute_CheckInAcrossMidnightReportsTheStampedDay(t *testing.T) {
	t.Parallel()
	h := newHarness(nil)

	requestedAt := time.Date(2026, 7, 21, 23, 59, 59, 0, berlin)
	stampedAt := requestedAt.Add(2 * time.Second) // the write lands after midnight
	stampedDay := berlinDay(stampedAt)
	require.NotEqual(t, berlinDay(requestedAt), stampedDay)

	// Only the day the row carries returns it; the request day is empty.
	h.stamps.workdays = map[string]Workday{
		stampedDay: workdayOf(Session{StaffID: testStaffID, Day: stampedDay, Status: StatusPresent, Source: SourceNFC, CheckInTime: stampedAt}),
	}
	calls := 0
	h.clock.now = func() time.Time {
		calls++
		if calls == 1 {
			return requestedAt
		}
		return stampedAt
	}

	state, err := h.service.ExecuteStaffClock(context.Background(), checkInCommand())

	require.NoError(t, err)
	require.NotNil(t, state.Session)
	assert.Equal(t, []string{berlinDay(requestedAt)}, h.stamps.checkInDays)
	assert.Equal(t, stampedDay, berlinDay(state.Session.CheckInTime))
	assert.Equal(t, StateCheckedIn, state.State)
	assert.Equal(t, []string{ActionBreakStart, ActionCheckOut}, state.AllowedActions)
	assert.Equal(t, SourceNFC, state.Session.Source)
}

// Check-out, break start and break end must look up the session on the day
// the request was taken on. An unpinned lookup re-derives "today" while the
// request runs and misses the session opened seconds before midnight.
func TestExecute_ActionsPinTheLookupToTheRequestDay(t *testing.T) {
	t.Parallel()
	requestedAt := time.Date(2026, 7, 21, 23, 59, 59, 0, berlin)
	requestDay := berlinDay(requestedAt)
	open := Session{StaffID: testStaffID, Day: requestDay, Status: StatusPresent, Source: SourceNFC, CheckInTime: requestedAt.Add(-8 * time.Hour)}

	for _, action := range []string{ActionCheckOut, ActionBreakStart, ActionBreakEnd} {
		t.Run(action, func(t *testing.T) {
			h := newHarness(nil)
			h.stamps.latestOpen = &open
			h.stamps.openSessionByDay = map[string]Session{requestDay: open}
			h.stamps.workdays = map[string]Workday{requestDay: workdayOf(open)}
			calls := 0
			h.clock.now = func() time.Time {
				calls++
				if calls == 1 {
					return requestedAt
				}
				return requestedAt.Add(2 * time.Second)
			}

			state, err := h.service.ExecuteStaffClock(context.Background(), StaffClockCommand{RFIDTag: testTag, Action: action})

			require.NoError(t, err)
			require.NotNil(t, state.Session)
			assert.Equal(t, []string{requestDay}, h.stamps.actionDays)
		})
	}
}

// nightSession is a session opened the evening before and still running after
// the Berlin midnight rollover.
func nightSession(openedAt time.Time) Session {
	return Session{ID: 91, StaffID: testStaffID, Day: berlinDay(openedAt), Status: StatusPresent, Source: SourceNFC, CheckInTime: openedAt}
}

// Somebody who clocked in yesterday evening and never clocked out is still at
// work after midnight. Reading only today's row reported checked_out and
// offered a second check-in on the new day.
func TestGetState_OpenSessionFromPreviousDayStaysCheckedIn(t *testing.T) {
	t.Parallel()
	h := newHarness(nil)

	openedAt := time.Date(2026, 7, 21, 22, 30, 0, 0, berlin)
	open := nightSession(openedAt)
	h.stamps.latestOpen = &open
	h.stamps.workdays = map[string]Workday{open.Day: workdayOf(open)}
	h.clock.now = func() time.Time { return openedAt.Add(3 * time.Hour) } // 01:30 the next day

	state, err := h.service.StaffClockState(context.Background(), testTag)

	require.NoError(t, err)
	require.NotNil(t, state.Session)
	assert.Equal(t, open.ID, state.Session.ID)
	assert.Equal(t, StateCheckedIn, state.State)
	assert.Equal(t, []string{ActionBreakStart, ActionCheckOut}, state.AllowedActions)
	assert.Equal(t, "Nora Kiosk", state.StaffName)
}

// The labor-time figures of the day are the capability's; the kiosk renders
// them without recomputing, and a running break flips the state.
func TestGetState_RendersLaborTimeAndActiveBreak(t *testing.T) {
	t.Parallel()
	h := newHarness(nil)

	openedAt := time.Date(2026, 7, 21, 23, 45, 0, 0, berlin)
	open := nightSession(openedAt)
	workday := workdayOf(open, Break{ID: 5, StartedAt: openedAt.Add(20 * time.Minute)})
	workday.NetMinutes, workday.BreakMinutes, workday.RequiredBreakMinutes, workday.IsBreakCompliant = 20, 25, 0, true
	h.stamps.latestOpen = &open
	h.stamps.workdays = map[string]Workday{open.Day: workday}
	h.clock.now = func() time.Time { return openedAt.Add(45 * time.Minute) }

	state, err := h.service.StaffClockState(context.Background(), testTag)

	require.NoError(t, err)
	assert.Equal(t, StateOnBreak, state.State)
	require.NotNil(t, state.ActiveBreak)
	assert.EqualValues(t, 5, state.ActiveBreak.ID)
	assert.Equal(t, []string{ActionBreakEnd, ActionCheckOut}, state.AllowedActions)
	assert.Equal(t, 20, state.NetMinutes)
	assert.Equal(t, 25, state.BreakMinutes)
	assert.True(t, state.IsBreakCompliant)
}

// The stamp that ends a night session must be dispatched on the day the
// session carries. Pinning it to the new day looks up a day the session was
// never written on and refuses a valid scan.
func TestExecute_ActionsAfterMidnightUseTheOpenSessionDay(t *testing.T) {
	t.Parallel()
	openedAt := time.Date(2026, 7, 21, 22, 30, 0, 0, berlin)
	open := nightSession(openedAt)
	afterMidnight := openedAt.Add(3 * time.Hour)
	require.NotEqual(t, open.Day, berlinDay(afterMidnight))

	for _, action := range []string{ActionCheckOut, ActionBreakStart, ActionBreakEnd} {
		t.Run(action, func(t *testing.T) {
			h := newHarness(nil)
			h.stamps.latestOpen = &open
			h.stamps.openSessionByDay = map[string]Session{open.Day: open}
			h.stamps.workdays = map[string]Workday{open.Day: workdayOf(open)}
			h.clock.now = func() time.Time { return afterMidnight }

			state, err := h.service.ExecuteStaffClock(context.Background(), StaffClockCommand{RFIDTag: testTag, Action: action})

			require.NoError(t, err)
			require.NotNil(t, state.Session)
			assert.Equal(t, []string{open.Day}, h.stamps.actionDays)
		})
	}
}

// A check-in with a DIFFERENT status after a same-day checkout simply starts a
// new block carrying that status (#2402): one stamp, no conflict retry.
func TestExecute_SecondBlockWithDifferentStatusStampsOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(nil)

	command := checkInCommand()
	command.Status = StatusHomeOffice

	state, err := h.service.ExecuteStaffClock(context.Background(), command)

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, 1, h.stamps.checkInCall, "a status switch needs exactly one stamp")
}

// Failures the time clock does not classify stay genuine faults and keep
// their 500 classification instead of being dressed up as a state conflict.
func TestExecute_UnclassifiedFailureStaysAnError(t *testing.T) {
	t.Parallel()
	failure := errors.New("connection reset")
	h := newHarness(failure)

	_, err := h.service.ExecuteStaffClock(context.Background(), checkInCommand())

	require.ErrorIs(t, err, failure)
	assert.NotErrorIs(t, err, ErrCheckInRaced)
	assert.NotErrorIs(t, err, ErrStateConflict)
}

// Classified stamp refusals reach the kiosk as their kind while keeping the
// wording of the time clock; typed refusals keep their payload.
func TestExecute_PassesStampRefusalsThrough(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cause error
		kind  error
	}{
		"state conflict":   {&StampError{Kind: ErrStateConflict, Cause: errors.New("already checked in")}, ErrStateConflict},
		"invalid duration": {&StampError{Kind: ErrInvalidStamp, Cause: errors.New("planned_duration_minutes must be between 1 and 240")}, ErrInvalidStamp},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(tc.cause)
			_, err := h.service.ExecuteStaffClock(context.Background(), checkInCommand())
			require.ErrorIs(t, err, tc.kind)
			assert.EqualError(t, err, tc.cause.Error())
		})
	}

	h := newHarness(&DeviationReasonRequiredError{Action: "check_in", PlannedTime: "08:00", ActualTime: "08:30", DeviationMinutes: 30})
	_, err := h.service.ExecuteStaffClock(context.Background(), checkInCommand())
	deviation, ok := errors.AsType[*DeviationReasonRequiredError](err)
	require.True(t, ok)
	assert.Equal(t, 30, deviation.DeviationMinutes)

	h = newHarness(&PlannedStartNotReachedError{PlannedStartTime: "08:00", CurrentTime: "07:30"})
	_, err = h.service.ExecuteStaffClock(context.Background(), checkInCommand())
	planned, ok := errors.AsType[*PlannedStartNotReachedError](err)
	require.True(t, ok)
	assert.Equal(t, "07:30", planned.CurrentTime)
}

// An RFID card that is active but linked to nobody resolves to no person. The
// terminal must be told the tag is unknown, the same answer a never-seen card
// gets.
func TestGetState_UnassignedCardIsReportedAsUnknownTag(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: time.Now}
	service := NewService(Dependencies{
		Cards:     &stubCards{card: &Card{ID: testTag, Active: true}},
		Staff:     &stubStaff{err: ErrRFIDTagNotFound},
		TimeClock: &stubTimeClock{clock: clock},
		Clock:     clock,
	})

	state, err := service.StaffClockState(context.Background(), testTag)

	require.ErrorIs(t, err, ErrRFIDTagNotFound)
	assert.Nil(t, state)
}

func TestGetState_CardOutcomesAreReportedAsStableErrors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cards CardLookup
		staff StaffLookup
		kind  error
	}{
		"unknown card":  {&stubCards{}, &stubStaff{}, ErrRFIDTagNotFound},
		"inactive card": {&stubCards{card: &Card{ID: testTag}}, &stubStaff{}, ErrRFIDTagInactive},
		"not staff":     {&stubCards{card: &Card{ID: testTag, Active: true}}, &stubStaff{err: ErrRFIDTagNotStaff}, ErrRFIDTagNotStaff},
		"no staff row":  {&stubCards{card: &Card{ID: testTag, Active: true}}, &stubStaff{identity: StaffIdentity{FullName: "Nobody"}}, ErrRFIDTagNotStaff},
	} {
		t.Run(name, func(t *testing.T) {
			clock := &fakeClock{now: time.Now}
			service := NewService(Dependencies{Cards: tc.cards, Staff: tc.staff, TimeClock: &stubTimeClock{clock: clock}, Clock: clock})
			_, err := service.StaffClockState(context.Background(), testTag)
			require.ErrorIs(t, err, tc.kind)
		})
	}
}

// The labor-time figures belong to the clock as it stands after the stamp.
// The instant that resolved the day is older than the write it preceded, so
// reusing it would measure a session from before its own check-in.
func TestExecute_LaborTimeIsEvaluatedAfterTheStamp(t *testing.T) {
	t.Parallel()
	requestedAt := time.Date(2026, 7, 21, 23, 59, 59, 0, berlin)
	stampedAt := requestedAt.Add(2 * time.Second) // the write lands after midnight
	evaluatedAt := stampedAt.Add(10 * time.Minute)

	h := newHarness(nil)
	stamped := Session{ID: 93, StaffID: testStaffID, Day: berlinDay(stampedAt), Status: StatusPresent, Source: SourceNFC, CheckInTime: stampedAt}
	h.stamps.workdays = map[string]Workday{stamped.Day: workdayOf(stamped)}

	// The clock the request resolved its day on, the clock the write landed on,
	// and the clock the state is rendered against are three distinct instants.
	instants := []time.Time{requestedAt, stampedAt, evaluatedAt}
	calls := 0
	h.clock.now = func() time.Time {
		if calls < len(instants) {
			calls++
		}
		return instants[calls-1]
	}

	state, err := h.service.ExecuteStaffClock(context.Background(), checkInCommand())

	require.NoError(t, err)
	require.NotNil(t, state.Session)
	assert.Equal(t, StateCheckedIn, state.State)
	require.Len(t, h.stamps.workdayNow, 1)
	assert.Equal(t, evaluatedAt, h.stamps.workdayNow[0], "elapsed work is measured against the clock after the write")
}

// A forgotten checkout that the capability already cut at its live limit is
// this day's closed block, never a running one: the kiosk offers a check-in,
// not a check-out that would be refused.
func TestGetState_ExpiredOpenBlockIsRenderedAsClosed(t *testing.T) {
	t.Parallel()
	h := newHarness(nil)

	openedAt := time.Date(2026, 7, 20, 20, 0, 0, 0, berlin)
	expiredAt := openedAt.Add(12 * time.Hour)
	forgotten := nightSession(openedAt)
	forgotten.CheckOutTime = &expiredAt
	h.stamps.intersecting = &Workday{Sessions: []Session{forgotten}, Breaks: map[int64][]Break{}, IsBreakCompliant: true}
	renderedAt := time.Date(2026, 7, 21, 10, 0, 0, 0, berlin)
	h.clock.now = func() time.Time { return renderedAt }

	state, err := h.service.StaffClockState(context.Background(), testTag)

	require.NoError(t, err)
	require.NotNil(t, state.Session)
	require.NotNil(t, state.Session.CheckOutTime, "the row hangs open, the kiosk state does not")
	assert.Equal(t, StateCheckedOut, state.State)
	assert.Equal(t, []string{ActionCheckIn}, state.AllowedActions)
	assert.Equal(t, []time.Time{renderedAt}, h.stamps.workdayNow)
}

// With no block on the day the kiosk shows the checked-out summary.
func TestGetState_NoBlockIsCheckedOut(t *testing.T) {
	t.Parallel()
	h := newHarness(nil)

	state, err := h.service.StaffClockState(context.Background(), testTag)

	require.NoError(t, err)
	assert.Nil(t, state.Session)
	assert.Equal(t, StateCheckedOut, state.State)
	assert.Equal(t, []string{ActionCheckIn}, state.AllowedActions)
	assert.True(t, state.IsBreakCompliant)
	assert.EqualValues(t, testStaffID, state.StaffID)
}

func TestExecute_RejectsUnknownActionsAndMissingStatus(t *testing.T) {
	t.Parallel()
	h := newHarness(nil)

	_, err := h.service.ExecuteStaffClock(context.Background(), StaffClockCommand{RFIDTag: testTag, Action: "dance"})
	require.ErrorIs(t, err, ErrInvalidAction)

	_, err = h.service.ExecuteStaffClock(context.Background(), StaffClockCommand{RFIDTag: testTag, Action: ActionCheckIn})
	require.ErrorIs(t, err, ErrStatusRequired)

	_, err = h.service.ExecuteStaffClock(context.Background(), StaffClockCommand{RFIDTag: testTag, Action: ActionCheckIn, Status: "remote"})
	require.ErrorIs(t, err, ErrStatusRequired)
	assert.Zero(t, h.stamps.checkInCall, "an invalid status never reaches the time clock")
}
