package staffclock

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// pgUniqueViolation builds the driver error a duplicate INSERT produces. The
// pgdriver fields are unexported, so the map is written through reflection —
// same approach as api/timetable/instances_update_test.go.
func pgUniqueViolation(constraint string) error {
	pgErr := pgdriver.Error{}
	v := reflect.ValueOf(&pgErr).Elem()
	field := v.FieldByName("m")
	ptr := unsafe.Pointer(field.UnsafeAddr()) //nolint:gosec // test-only field access
	*(*map[byte]string)(ptr) = map[byte]string{'C': "23505", 'n': constraint}
	return pgErr
}

type stubPeople struct {
	person *userModels.Person
	staff  *userModels.Staff
}

func (s *stubPeople) FindByTagID(context.Context, string) (*userModels.Person, error) {
	return s.person, nil
}

func (s *stubPeople) GetStaffByPersonID(context.Context, int64) (*userModels.Staff, error) {
	return s.staff, nil
}

type stubCards struct {
	card *userModels.RFIDCard
}

func (s *stubCards) FindByID(context.Context, string) (*userModels.RFIDCard, error) {
	return s.card, nil
}

func (s *stubCards) Create(context.Context, *userModels.RFIDCard) error { return nil }
func (s *stubCards) Update(context.Context, *userModels.RFIDCard) error {
	return nil
}
func (s *stubCards) Delete(context.Context, string) error { return nil }
func (s *stubCards) List(context.Context, map[string]any) ([]*userModels.RFIDCard, error) {
	return nil, nil
}
func (s *stubCards) Deactivate(context.Context, string) error { return nil }

type stubWorkSessions struct {
	checkInErr     error
	checkInCall    int
	checkInSession *activeModels.WorkSession
	// actionDays records the calendar day every mutating action was pinned to.
	actionDays []timezone.Date
	// openSessionByDay mirrors the day-scoped lookup inside the work session
	// service: an action only finds the session on the day it was written on.
	openSessionByDay map[string]*activeModels.WorkSession
	// historyByDay mirrors the date-scoped read: only the day the session was
	// stamped on returns it.
	historyByDay map[string]*activeSvc.SessionResponse
}

func (s *stubWorkSessions) CheckIn(context.Context, int64, string, string, string) (*activeModels.WorkSession, error) {
	s.checkInCall++
	if s.checkInErr != nil {
		return nil, s.checkInErr
	}
	return s.checkInSession, nil
}

// openOn is the stand-in for the day-pinned lookup the real service performs.
func (s *stubWorkSessions) openOn(day timezone.Date) (*activeModels.WorkSession, error) {
	s.actionDays = append(s.actionDays, day)
	session, ok := s.openSessionByDay[day.String()]
	if !ok {
		return nil, errors.New("no active session found")
	}
	return session, nil
}

func (s *stubWorkSessions) CheckOutOn(_ context.Context, _ int64, day timezone.Date, _ string) (*activeModels.WorkSession, error) {
	return s.openOn(day)
}

func (s *stubWorkSessions) StartBreakOn(_ context.Context, _ int64, day timezone.Date, _ *int) (*activeModels.WorkSessionBreak, error) {
	if _, err := s.openOn(day); err != nil {
		return nil, err
	}
	return &activeModels.WorkSessionBreak{}, nil
}

func (s *stubWorkSessions) EndBreakOn(_ context.Context, _ int64, day timezone.Date) (*activeModels.WorkSession, error) {
	return s.openOn(day)
}

func (s *stubWorkSessions) UpdateSession(context.Context, int64, int64, activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error) {
	return nil, nil
}

func (s *stubWorkSessions) GetHistory(_ context.Context, _ int64, from, _ timezone.Date) (*activeSvc.HistoryResponse, error) {
	if session, ok := s.historyByDay[from.String()]; ok {
		return &activeSvc.HistoryResponse{Sessions: []*activeSvc.SessionResponse{session}}, nil
	}
	return &activeSvc.HistoryResponse{}, nil
}

func newRacedService(checkInErr error) (*Service, *stubWorkSessions) {
	person := &userModels.Person{FirstName: "Nora", LastName: "Kiosk"}
	person.ID = 42
	staff := &userModels.Staff{PersonID: person.ID}
	staff.ID = 7
	card := &userModels.RFIDCard{Active: true}
	card.ID = "A1654BEEF"

	sessions := &stubWorkSessions{checkInErr: checkInErr}
	service := NewService(&stubPeople{person: person, staff: staff}, &stubCards{card: card}, sessions)
	service.now = func() time.Time { return time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC) }
	return service, sessions
}

func checkInCommand() Command {
	return Command{
		RFIDTag: "A1654BEEF",
		Action:  ActionCheckIn,
		Status:  activeModels.WorkSessionStatusPresent,
	}
}

// Two kiosks scanning the same card at the same moment both see "no session
// today" and both insert. The loser must learn it lost the race, not that the
// server broke.
func TestExecute_ConcurrentCheckInReportsStateConflict(t *testing.T) {
	service, sessions := newRacedService(
		&modelBase.DatabaseError{Op: "create", Err: pgUniqueViolation(workSessionDateConstraint)},
	)
	ctx := tenant.WithRollbackMarker(context.Background())

	state, err := service.Execute(ctx, checkInCommand())

	require.ErrorIs(t, err, ErrCheckInRaced)
	assert.Nil(t, state)
	assert.Equal(t, 1, sessions.checkInCall)
	// The duplicate INSERT left the request transaction aborted, so the partial
	// work must never be committed alongside the 409.
	assert.True(t, tenant.RollbackRequested(ctx))
}

// A check-in that straddles Berlin midnight is persisted on the day the write
// happened, and that day — not the day the request started on — decides what is
// read back. Anchoring the reload to the request clock reported "checked_out, no
// session" for a successful stamp, which invites the kiosk to check in a second
// time and leaves the first session open overnight.
func TestExecute_CheckInAcrossMidnightReportsTheStampedDay(t *testing.T) {
	service, sessions := newRacedService(nil)

	requestedAt := time.Date(2026, 7, 21, 23, 59, 59, 0, timezone.Berlin)
	stampedAt := requestedAt.Add(2 * time.Second) // the write lands after midnight
	stampedDay := timezone.DateFromTime(stampedAt)
	require.NotEqual(t, timezone.DateFromTime(requestedAt), stampedDay)

	stamped := &activeModels.WorkSession{
		StaffID:     7,
		Date:        stampedDay,
		Status:      activeModels.WorkSessionStatusPresent,
		Source:      activeModels.WorkSessionSourceNFC,
		CheckInTime: stampedAt,
	}
	sessions.checkInSession = stamped
	// Only the day the row carries returns it — the request day is empty.
	sessions.historyByDay = map[string]*activeSvc.SessionResponse{
		stampedDay.String(): {WorkSession: stamped},
	}

	service.now = func() time.Time { return requestedAt }

	state, err := service.Execute(context.Background(), checkInCommand())

	require.NoError(t, err)
	require.NotNil(t, state.Session)
	assert.Equal(t, stampedDay, timezone.DateFromTime(state.Session.CheckInTime))
	assert.Equal(t, StateCheckedIn, state.State)
	assert.Equal(t, []string{ActionBreakStart, ActionCheckOut}, state.AllowedActions)
}

// Check-out, break start and break end must look up the session on the day the
// request was taken on. An unpinned lookup re-derives "today" while the request
// runs and misses the session opened seconds before midnight, refusing a valid
// stamp with "no active session found".
func TestExecute_ActionsPinTheLookupToTheRequestDay(t *testing.T) {
	requestedAt := time.Date(2026, 7, 21, 23, 59, 59, 0, timezone.Berlin)
	requestDay := timezone.DateFromTime(requestedAt)

	open := &activeModels.WorkSession{
		StaffID:     7,
		Date:        requestDay,
		Status:      activeModels.WorkSessionStatusPresent,
		Source:      activeModels.WorkSessionSourceNFC,
		CheckInTime: requestedAt.Add(-8 * time.Hour),
	}

	for _, action := range []string{ActionCheckOut, ActionBreakStart, ActionBreakEnd} {
		t.Run(action, func(t *testing.T) {
			service, sessions := newRacedService(nil)
			sessions.openSessionByDay = map[string]*activeModels.WorkSession{requestDay.String(): open}
			sessions.historyByDay = map[string]*activeSvc.SessionResponse{
				requestDay.String(): {WorkSession: open},
			}

			// The clock rolls past midnight right after the request timestamp is
			// taken; every lookup must still land on the request day.
			calls := 0
			service.now = func() time.Time {
				calls++
				if calls == 1 {
					return requestedAt
				}
				return requestedAt.Add(2 * time.Second)
			}

			state, err := service.Execute(context.Background(), Command{RFIDTag: "A1654BEEF", Action: action})

			require.NoError(t, err)
			require.NotNil(t, state.Session)
			assert.Equal(t, []timezone.Date{requestDay}, sessions.actionDays)
		})
	}
}

// A unique violation on another constraint is a genuine fault and must keep
// its 500 classification instead of being dressed up as a state conflict.
func TestExecute_UnrelatedUniqueViolationStaysAnError(t *testing.T) {
	service, _ := newRacedService(
		&modelBase.DatabaseError{Op: "create", Err: pgUniqueViolation("uq_something_else")},
	)
	ctx := tenant.WithRollbackMarker(context.Background())

	_, err := service.Execute(ctx, checkInCommand())

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCheckInRaced)
	assert.False(t, tenant.RollbackRequested(ctx))
}
