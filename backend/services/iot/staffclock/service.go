// Package staffclock coordinates NFC-based staff time tracking for kiosks.
package staffclock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// workSessionDateConstraint is the unique index behind one work session per
// staff member and day (migration 1.10.1). Two kiosks scanning the same card at
// once both find no session and both insert; the loser hits this constraint.
const workSessionDateConstraint = "uq_work_sessions_staff_date"

const (
	ActionCheckIn    = "checkin"
	ActionCheckOut   = "checkout"
	ActionBreakStart = "break_start"
	ActionBreakEnd   = "break_end"

	StateCheckedOut = "checked_out"
	StateCheckedIn  = "checked_in"
	StateOnBreak    = "on_break"
)

var (
	ErrInvalidRFIDTag  = errors.New("invalid RFID tag")
	ErrRFIDTagNotFound = errors.New("RFID tag not found")
	ErrRFIDTagInactive = errors.New("RFID tag is inactive")
	ErrRFIDTagNotStaff = errors.New("RFID tag is not assigned to staff")
	ErrInvalidAction   = errors.New("invalid staff clock action")
	ErrStatusRequired  = errors.New("status is required for check-in")
	// ErrCheckInRaced reports that a concurrent scan of the same card already
	// created today's session. It is a state conflict, not a server fault.
	ErrCheckInRaced = errors.New("check-in for today was already recorded")
)

type personService interface {
	FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error)
}

// workSessionService is the day-pinned subset of the work session service. The
// kiosk deliberately uses the *On variants: every action of one request acts on
// the same calendar day, so a stamp cannot be written on one day and looked up
// on the next when the request straddles Berlin midnight.
type workSessionService interface {
	CheckInOn(ctx context.Context, staffID int64, day timezone.Date, status, source, reason string) (*activeModels.WorkSession, error)
	CheckOutOn(ctx context.Context, staffID int64, day timezone.Date, reason string) (*activeModels.WorkSession, error)
	StartBreakOn(ctx context.Context, staffID int64, day timezone.Date, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error)
	EndBreakOn(ctx context.Context, staffID int64, day timezone.Date) (*activeModels.WorkSession, error)
	UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error)
	GetLatestOpenSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	GetHistory(ctx context.Context, staffID int64, from, to timezone.Date) (*activeSvc.HistoryResponse, error)
}

// Service owns the pure NFC stamp workflow. It deliberately accepts narrow
// interfaces so its state machine can be tested without HTTP or a database.
type Service struct {
	people       personService
	cards        userModels.RFIDCardRepository
	workSessions workSessionService
	now          func() time.Time
}

type State struct {
	StaffID              int64    `json:"staff_id"`
	StaffName            string   `json:"staff_name"`
	State                string   `json:"state"`
	AllowedActions       []string `json:"allowed_actions"`
	Session              *Session `json:"session,omitempty"`
	ActiveBreak          *Break   `json:"active_break,omitempty"`
	NetMinutes           int      `json:"net_minutes"`
	BreakMinutes         int      `json:"break_minutes"`
	RequiredBreakMinutes int      `json:"required_break_minutes"`
	IsBreakCompliant     bool     `json:"is_break_compliant"`
}

// Session is the kiosk projection of a work session. The persistence model
// carries tenant, audit and free-text note columns — status-change and
// administrative notes among them — that a shared device in a hallway has no
// use for and must not receive, so only the rendered fields are exposed.
type Session struct {
	ID           int64      `json:"id"`
	StaffID      int64      `json:"staff_id"`
	CheckInTime  time.Time  `json:"check_in_time"`
	CheckOutTime *time.Time `json:"check_out_time,omitempty"`
	Status       string     `json:"status"`
	Source       string     `json:"source"`
}

// Break is the kiosk projection of an active break: enough to show that one is
// running, nothing more.
type Break struct {
	ID        int64     `json:"id"`
	StartedAt time.Time `json:"started_at"`
}

func newSession(session *activeModels.WorkSession) *Session {
	if session == nil {
		return nil
	}
	return &Session{
		ID:           session.ID,
		StaffID:      session.StaffID,
		CheckInTime:  session.CheckInTime,
		CheckOutTime: session.CheckOutTime,
		Status:       session.Status,
		Source:       session.Source,
	}
}

func newBreak(workBreak *activeModels.WorkSessionBreak) *Break {
	if workBreak == nil {
		return nil
	}
	return &Break{ID: workBreak.ID, StartedAt: workBreak.StartedAt}
}

type Command struct {
	RFIDTag                string
	Action                 string
	Status                 string
	Reason                 string
	PlannedDurationMinutes *int
}

func NewService(people personService, cards userModels.RFIDCardRepository, workSessions workSessionService) *Service {
	return &Service{people: people, cards: cards, workSessions: workSessions}
}

func (s *Service) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// GetState resolves a staff card and returns the authoritative kiosk state.
func (s *Service) GetState(ctx context.Context, rawTag string) (*State, error) {
	person, staff, err := s.resolveStaff(ctx, rawTag)
	if err != nil {
		return nil, err
	}
	now := s.currentTime()
	day, err := s.clockDay(ctx, staff.ID, now)
	if err != nil {
		return nil, err
	}
	return s.loadState(ctx, person, staff, day, now)
}

// clockDay is the calendar day one kiosk request works on: the day carried by
// the session that is still running, or today when nobody is clocked in.
//
// Taking "today" unconditionally would hide a session opened before Berlin
// midnight. The kiosk would report checked_out to somebody who is demonstrably
// still at work, offer a second check-in on the new day, and leave the first
// session with no way to be closed through this flow at all.
func (s *Service) clockDay(ctx context.Context, staffID int64, now time.Time) (timezone.Date, error) {
	open, err := s.workSessions.GetLatestOpenSession(ctx, staffID)
	if err != nil {
		return timezone.Date{}, fmt.Errorf("look up running work session: %w", err)
	}
	if open != nil {
		return open.Date, nil
	}
	return timezone.DateFromTime(now), nil
}

// Execute performs one explicit stamp action with source=nfc, then reloads
// state so the kiosk never has to predict the result locally.
//
// One calendar day serves the whole request: it is resolved once, up front,
// and handed to every write and read the request performs. Nothing downstream
// re-derives it from the clock, because two places asking the clock
// independently disagree the moment a request straddles Berlin midnight — the
// stamp lands on one day while the lookup searches the other, and a valid scan
// fails with "no active session found" or reports "checked_out" right after a
// successful check-in.
//
// The day is the running session's own day when there is one (a night shift
// keeps working on the day it started), otherwise today.
func (s *Service) Execute(ctx context.Context, command Command) (*State, error) {
	person, staff, err := s.resolveStaff(ctx, command.RFIDTag)
	if err != nil {
		return nil, err
	}
	now := s.currentTime()
	day, err := s.clockDay(ctx, staff.ID, now)
	if err != nil {
		return nil, err
	}

	var stamped *activeModels.WorkSession
	switch command.Action {
	case ActionCheckIn:
		stamped, err = s.checkIn(ctx, staff.ID, day, command)
	case ActionCheckOut:
		stamped, err = s.workSessions.CheckOutOn(ctx, staff.ID, day, command.Reason)
	case ActionBreakStart:
		_, err = s.workSessions.StartBreakOn(ctx, staff.ID, day, command.PlannedDurationMinutes)
	case ActionBreakEnd:
		stamped, err = s.workSessions.EndBreakOn(ctx, staff.ID, day)
	default:
		return nil, ErrInvalidAction
	}
	if err != nil {
		return nil, err
	}
	if stamped != nil {
		day = stamped.Date
	}

	// The labor-time figures are read against the clock as it stands after the
	// write, not against the instant the request was admitted. `now` selected
	// the day and is deliberately left alone; reusing it here would measure the
	// session from before its own check-in whenever the stamp crossed midnight,
	// reporting zero elapsed work and a break requirement computed for the
	// wrong point in the shift.
	return s.loadState(ctx, person, staff, day, s.currentTime())
}

// checkIn stamps the arrival, resolving the reopen-status conflict when the
// caller supplied a reason.
//
// A concurrent scan of the same card is reported as ErrCheckInRaced rather than
// bubbling the raw unique violation out as a 500: both requests read "no
// session today" before either inserted, so the loser is looking at a state
// conflict the winner just created. The duplicate INSERT also leaves the
// request transaction aborted — nothing can be re-read or retried on it — so
// the rollback is requested explicitly and the kiosk is told to rescan for
// authoritative state.
// Both the first stamp and the reopen retry are pinned to the day the request
// resolved, so the retry cannot land on a fresh session of the following day
// while the status update it is paired with rewrites the previous day's row.
// The stamped session is returned so the caller can read the state back on the
// day the row actually carries.
func (s *Service) checkIn(ctx context.Context, staffID int64, day timezone.Date, command Command) (*activeModels.WorkSession, error) {
	if command.Status == "" {
		return nil, ErrStatusRequired
	}
	if command.Status != activeModels.WorkSessionStatusPresent && command.Status != activeModels.WorkSessionStatusHomeOffice {
		return nil, fmt.Errorf("%w: status must be 'present' or 'home_office'", ErrStatusRequired)
	}

	session, err := s.workSessions.CheckInOn(ctx, staffID, day, command.Status, activeModels.WorkSessionSourceNFC, command.Reason)
	var conflict *activeSvc.ReopenStatusConflictError
	if errors.As(err, &conflict) {
		if strings.TrimSpace(command.Reason) == "" {
			return nil, conflict
		}
		if _, err = s.workSessions.CheckInOn(ctx, staffID, day, conflict.ExistingStatus, activeModels.WorkSessionSourceNFC, command.Reason); err != nil {
			return nil, s.classifyCheckInError(ctx, fmt.Errorf("reopen session with existing status: %w", err))
		}
		reason := strings.TrimSpace(command.Reason)
		status := command.Status
		updated, updateErr := s.workSessions.UpdateSession(ctx, staffID, conflict.SessionID, activeSvc.SessionUpdateRequest{Status: &status, Notes: &reason})
		if updateErr != nil {
			// The request transaction must roll back the successful reopen if
			// this audit-bearing status update cannot be persisted.
			return nil, fmt.Errorf("update reopened session status: %w", updateErr)
		}
		return updated, nil
	}
	if err != nil {
		return nil, s.classifyCheckInError(ctx, err)
	}
	return session, nil
}

func (s *Service) classifyCheckInError(ctx context.Context, err error) error {
	if modelBase.IsUniqueViolationOn(err, workSessionDateConstraint) {
		tenant.MarkRollback(ctx)
		return ErrCheckInRaced
	}
	return err
}

func (s *Service) resolveStaff(ctx context.Context, rawTag string) (*userModels.Person, *userModels.Staff, error) {
	normalized := userModels.NormalizeTagID(rawTag)
	cardCandidate := &userModels.RFIDCard{}
	cardCandidate.ID = normalized
	if err := cardCandidate.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidRFIDTag, err)
	}

	card, err := s.cards.FindByID(ctx, normalized)
	if err != nil {
		return nil, nil, fmt.Errorf("look up RFID card: %w", err)
	}
	if card == nil {
		return nil, nil, ErrRFIDTagNotFound
	}
	if !card.Active {
		return nil, nil, ErrRFIDTagInactive
	}

	person, err := s.people.FindByTagID(ctx, normalized)
	if err != nil {
		if errors.Is(err, usersSvc.ErrPersonNotFound) {
			return nil, nil, ErrRFIDTagNotFound
		}
		return nil, nil, fmt.Errorf("look up person by RFID tag: %w", err)
	}
	// An active card that is not linked to anybody resolves to no person. The
	// kiosk gets the same stable "unknown tag" answer it gets for a card the
	// system has never seen — dereferencing the nil would turn an unassigned
	// card into a 500 at the terminal.
	if person == nil {
		return nil, nil, ErrRFIDTagNotFound
	}
	staff, err := s.people.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrRFIDTagNotStaff
		}
		return nil, nil, fmt.Errorf("look up staff for RFID tag: %w", err)
	}
	if staff == nil {
		return nil, nil, ErrRFIDTagNotStaff
	}
	return person, staff, nil
}

// loadState renders the kiosk state of `day`. It never derives that day itself:
// the caller passes the day the session was written on, while `now` is only the
// reference instant for the labor-time figures.
func (s *Service) loadState(ctx context.Context, person *userModels.Person, staff *userModels.Staff, day timezone.Date, now time.Time) (*State, error) {
	result := &State{
		StaffID:          staff.ID,
		StaffName:        person.GetFullName(),
		State:            StateCheckedOut,
		AllowedActions:   []string{ActionCheckIn},
		IsBreakCompliant: true,
	}

	session, breaks, err := s.resolveDay(ctx, staff.ID, day)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return result, nil
	}

	result.Session = newSession(session)
	if session.CheckOutTime == nil {
		result.State = StateCheckedIn
		result.AllowedActions = []string{ActionBreakStart, ActionCheckOut}
		for _, workBreak := range breaks {
			if workBreak.IsActive() {
				result.State = StateOnBreak
				result.ActiveBreak = newBreak(workBreak)
				result.AllowedActions = []string{ActionBreakEnd, ActionCheckOut}
				break
			}
		}
	}

	evaluation := activeSvc.EvaluateLaborTime(session, breaks, now)
	result.NetMinutes = evaluation.NetMinutes
	result.BreakMinutes = evaluation.BreakMinutes
	result.RequiredBreakMinutes = evaluation.RequiredBreakMinutes
	result.IsBreakCompliant = evaluation.IsBreakCompliant
	return result, nil
}

// resolveDay returns the session of `day` together with its breaks.
//
// The day is asked for explicitly instead of going through the open-session
// lookup, which is scoped to the server's current date and would come back
// empty for a session stamped seconds before midnight. This query returns the
// row whether it is still open or already closed, so open and closed state are
// derived from check_out_time rather than from which read happened to hit.
func (s *Service) resolveDay(ctx context.Context, staffID int64, day timezone.Date) (*activeModels.WorkSession, []*activeModels.WorkSessionBreak, error) {
	history, err := s.workSessions.GetHistory(ctx, staffID, day, day)
	if err != nil {
		return nil, nil, fmt.Errorf("load work session of the day: %w", err)
	}
	if history == nil || len(history.Sessions) == 0 {
		return nil, nil, nil
	}
	daySession := history.Sessions[0]
	return daySession.WorkSession, daySession.Breaks, nil
}
