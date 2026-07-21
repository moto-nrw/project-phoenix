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

type workSessionService interface {
	CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*activeModels.WorkSession, error)
	CheckOut(ctx context.Context, staffID int64, reason string) (*activeModels.WorkSession, error)
	StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error)
	EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	GetSessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*activeModels.WorkSessionBreak, error)
	UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error)
	GetCurrentSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
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
	return s.loadState(ctx, person, staff)
}

// Execute performs one explicit stamp action with source=nfc, then reloads
// state so the kiosk never has to predict the result locally.
func (s *Service) Execute(ctx context.Context, command Command) (*State, error) {
	person, staff, err := s.resolveStaff(ctx, command.RFIDTag)
	if err != nil {
		return nil, err
	}

	switch command.Action {
	case ActionCheckIn:
		err = s.checkIn(ctx, staff.ID, command)
	case ActionCheckOut:
		_, err = s.workSessions.CheckOut(ctx, staff.ID, command.Reason)
	case ActionBreakStart:
		_, err = s.workSessions.StartBreak(ctx, staff.ID, command.PlannedDurationMinutes)
	case ActionBreakEnd:
		_, err = s.workSessions.EndBreak(ctx, staff.ID)
	default:
		return nil, ErrInvalidAction
	}
	if err != nil {
		return nil, err
	}

	return s.loadState(ctx, person, staff)
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
func (s *Service) checkIn(ctx context.Context, staffID int64, command Command) error {
	if command.Status == "" {
		return ErrStatusRequired
	}
	if command.Status != activeModels.WorkSessionStatusPresent && command.Status != activeModels.WorkSessionStatusHomeOffice {
		return fmt.Errorf("%w: status must be 'present' or 'home_office'", ErrStatusRequired)
	}

	_, err := s.workSessions.CheckIn(ctx, staffID, command.Status, activeModels.WorkSessionSourceNFC, command.Reason)
	var conflict *activeSvc.ReopenStatusConflictError
	if errors.As(err, &conflict) {
		if strings.TrimSpace(command.Reason) == "" {
			return conflict
		}
		if _, err = s.workSessions.CheckIn(ctx, staffID, conflict.ExistingStatus, activeModels.WorkSessionSourceNFC, command.Reason); err != nil {
			return s.classifyCheckInError(ctx, fmt.Errorf("reopen session with existing status: %w", err))
		}
		reason := strings.TrimSpace(command.Reason)
		status := command.Status
		if _, err = s.workSessions.UpdateSession(ctx, staffID, conflict.SessionID, activeSvc.SessionUpdateRequest{Status: &status, Notes: &reason}); err != nil {
			// The request transaction must roll back the successful reopen if
			// this audit-bearing status update cannot be persisted.
			return fmt.Errorf("update reopened session status: %w", err)
		}
		return nil
	}
	return s.classifyCheckInError(ctx, err)
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

func (s *Service) loadState(ctx context.Context, person *userModels.Person, staff *userModels.Staff) (*State, error) {
	result := &State{
		StaffID:          staff.ID,
		StaffName:        person.GetFullName(),
		State:            StateCheckedOut,
		AllowedActions:   []string{ActionCheckIn},
		IsBreakCompliant: true,
	}
	session, err := s.workSessions.GetCurrentSession(ctx, staff.ID)
	if err != nil {
		return nil, fmt.Errorf("load current work session: %w", err)
	}
	if session == nil {
		today := timezone.DateFromTime(s.currentTime())
		history, historyErr := s.workSessions.GetHistory(ctx, staff.ID, today, today)
		if historyErr != nil {
			return nil, fmt.Errorf("load today's work session: %w", historyErr)
		}
		if history != nil && len(history.Sessions) > 0 {
			todaySession := history.Sessions[0]
			result.Session = newSession(todaySession.WorkSession)
			result.NetMinutes = todaySession.NetMinutes
			result.BreakMinutes = todaySession.BreakMinutes
			result.IsBreakCompliant = todaySession.IsBreakCompliant
			result.RequiredBreakMinutes = activeSvc.EvaluateLaborTime(
				todaySession.WorkSession,
				todaySession.Breaks,
				s.currentTime(),
			).RequiredBreakMinutes
		}
		return result, nil
	}

	breaks, err := s.workSessions.GetSessionBreaks(ctx, staff.ID, session.ID)
	if err != nil {
		return nil, fmt.Errorf("load work session breaks: %w", err)
	}
	result.Session = newSession(session)
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

	evaluation := activeSvc.EvaluateLaborTime(session, breaks, s.currentTime())
	result.NetMinutes = evaluation.NetMinutes
	result.BreakMinutes = evaluation.BreakMinutes
	result.RequiredBreakMinutes = evaluation.RequiredBreakMinutes
	result.IsBreakCompliant = evaluation.IsBreakCompliant
	return result, nil
}
