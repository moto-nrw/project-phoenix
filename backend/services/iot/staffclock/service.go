// Package staffclock coordinates NFC-based staff time tracking for kiosks.
package staffclock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// workSessionOpenConstraint is the database index behind "at most one OPEN
// work session per staff member and day" (#2402).
// Two kiosks scanning the same card at once both find no open session and
// both insert an open row; the loser hits this constraint.
const workSessionOpenConstraint = "uq_work_sessions_staff_date_open"

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
	GetLatestOpenSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	GetHistory(ctx context.Context, staffID int64, from, to timezone.Date) (*activeSvc.HistoryResponse, error)
	GetHistoryIntersecting(ctx context.Context, staffID int64, from, to timezone.Date) (*activeSvc.HistoryResponse, error)
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
		return timezone.Date(""), fmt.Errorf("look up running work session: %w", err)
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

// checkIn stamps the arrival. Since #2402 a repeated check-in after a
// checkout creates a NEW work block with its own status — the old
// reopen-status-conflict dance is gone, so a Homeoffice morning followed by
// an OGS afternoon needs no reason and no follow-up edit.
//
// A concurrent scan of the same card is reported as ErrCheckInRaced rather than
// bubbling the raw unique violation out as a 500: both requests read "no open
// session" before either inserted, so the loser is looking at a state
// conflict the winner just created. The duplicate INSERT also leaves the
// request transaction aborted — nothing can be re-read or retried on it — so
// the rollback is requested explicitly and the kiosk is told to rescan for
// authoritative state.
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
	if err != nil {
		return nil, s.classifyCheckInError(ctx, err)
	}
	return session, nil
}

func (s *Service) classifyCheckInError(ctx context.Context, err error) error {
	if modelBase.IsUniqueViolationOn(err, workSessionOpenConstraint) {
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

	sessions, breaksBySession, err := s.resolveDay(ctx, staff.ID, day, now)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return result, nil
	}

	// The block the kiosk acts on: the open one when someone is clocked in,
	// otherwise the chronologically last block of the day (checked-out
	// summary + the offer to start a new block).
	current := sessions[len(sessions)-1]
	for _, session := range sessions {
		if session.CheckOutTime == nil {
			current = session
			break
		}
	}

	result.Session = newSession(current)
	if current.CheckOutTime == nil {
		result.State = StateCheckedIn
		result.AllowedActions = []string{ActionBreakStart, ActionCheckOut}
		for _, workBreak := range breaksBySession[current.ID] {
			if workBreak.IsActive() {
				result.State = StateOnBreak
				result.ActiveBreak = newBreak(workBreak)
				result.AllowedActions = []string{ActionBreakEnd, ActionCheckOut}
				break
			}
		}
	}

	// The labor-time figures cover the complete workday, not just the current block:
	// with a Homeoffice morning and an OGS afternoon (#2402) the kiosk must
	// show the summed work time, and §4 ArbZG judges the day as a whole. A block
	// that remains open across midnight stays one continuous workday here; its
	// check-out is still written to the original block.
	evaluation := activeSvc.EvaluateWorkSessionsLaborTime(sessions, breaksBySession, now)
	result.NetMinutes = evaluation.NetMinutes
	result.BreakMinutes = evaluation.BreakMinutes
	result.RequiredBreakMinutes = evaluation.RequiredBreakMinutes
	result.IsBreakCompliant = evaluation.IsBreakCompliant
	return result, nil
}

// resolveDay returns all work blocks of `day` together with their breaks,
// keyed by session ID, in check-in order.
//
// The day is asked for explicitly instead of going through the open-session
// lookup, which is scoped to the server's current date and would come back
// empty for a session stamped seconds before midnight. This query returns the
// rows whether they are still open or already closed, so open and closed state
// are derived from check_out_time rather than from which read happened to hit.
//
// An open block that has passed its live limit is the one row that must not be
// taken at face value. It reaches into every later day (no check-out means no
// upper bound), so the intersecting query keeps returning it long after the
// open-session lookup has dropped it — and the kiosk would report a shift that
// ended days ago as running, offer a check-out for it, and then fail that
// check-out because it is written against `day` while the row belongs to an
// earlier one. Such a block is cut at the instant it stopped counting as work
// everywhere else (ExpireStaleOpenBlock): it shows up as the closed block it
// effectively is, and drops out entirely once that instant lies before the day
// begins. The row itself is repaired on the next check-in, not here.
func (s *Service) resolveDay(ctx context.Context, staffID int64, day timezone.Date, now time.Time) ([]*activeModels.WorkSession, map[int64][]*activeModels.WorkSessionBreak, error) {
	history, err := s.workSessions.GetHistoryIntersecting(ctx, staffID, day, day)
	if err != nil {
		return nil, nil, fmt.Errorf("load work sessions of the day: %w", err)
	}
	if history == nil || len(history.Sessions) == 0 {
		return nil, nil, nil
	}
	dayStart := day.BerlinMidnight()
	sessions := make([]*activeModels.WorkSession, 0, len(history.Sessions))
	breaksBySession := make(map[int64][]*activeModels.WorkSessionBreak, len(history.Sessions))
	for _, daySession := range history.Sessions {
		session := daySession.WorkSession
		if expired, stale := activeSvc.ExpireStaleOpenBlock(session, now); stale {
			if !expired.CheckOutTime.After(dayStart) {
				continue
			}
			session = expired
		}
		sessions = append(sessions, session)
		breaksBySession[daySession.ID] = daySession.Breaks
	}
	return sessions, breaksBySession, nil
}
