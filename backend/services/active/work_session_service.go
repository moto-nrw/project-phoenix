package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
)

// SessionUpdateRequest defines the structure for updating a work session
type SessionUpdateRequest struct {
	CheckInTime  *time.Time `json:"check_in_time"`
	CheckOutTime *time.Time `json:"check_out_time"`
	BreakMinutes *int       `json:"break_minutes"`
	Status       *string    `json:"status"`
	Notes        *string    `json:"notes"`
}

// SessionResponse wraps a work session with calculated fields
type SessionResponse struct {
	*activeModels.WorkSession
	NetMinutes       int                              `json:"net_minutes"`
	IsOvertime       bool                             `json:"is_overtime"`
	IsBreakCompliant bool                             `json:"is_break_compliant"`
	Breaks           []*activeModels.WorkSessionBreak `json:"breaks"`
}

// WorkSessionService defines operations for staff time tracking
type WorkSessionService interface {
	CheckIn(ctx context.Context, staffID int64, status string) (*activeModels.WorkSession, error)
	CheckOut(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	StartBreak(ctx context.Context, staffID int64) (*activeModels.WorkSessionBreak, error)
	EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	GetSessionBreaks(ctx context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error)
	UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates SessionUpdateRequest) (*activeModels.WorkSession, error)
	GetCurrentSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	GetHistory(ctx context.Context, staffID int64, from, to time.Time) ([]*SessionResponse, error)
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)
	CleanupOpenSessions(ctx context.Context) (int, error)
	EnsureCheckedIn(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
}

// workSessionService implements WorkSessionService
type workSessionService struct {
	repo      activeModels.WorkSessionRepository
	breakRepo activeModels.WorkSessionBreakRepository
}

// NewWorkSessionService creates a new work session service
func NewWorkSessionService(repo activeModels.WorkSessionRepository, breakRepo activeModels.WorkSessionBreakRepository) WorkSessionService {
	return &workSessionService{repo: repo, breakRepo: breakRepo}
}

// CheckIn creates a new work session for the staff member
func (s *workSessionService) CheckIn(ctx context.Context, staffID int64, status string) (*activeModels.WorkSession, error) {
	// Default status to present if empty
	if status == "" {
		status = activeModels.WorkSessionStatusPresent
	}

	// Validate status
	if status != activeModels.WorkSessionStatusPresent && status != activeModels.WorkSessionStatusHomeOffice {
		return nil, fmt.Errorf("status must be 'present' or 'home_office'")
	}

	// Get today's date (truncated to date)
	today := time.Now().Truncate(24 * time.Hour)

	// Check if there's already a session today
	existingSession, err := s.repo.GetByStaffAndDate(ctx, staffID, today)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing session: %w", err)
	}

	if existingSession != nil {
		if existingSession.IsActive() {
			return nil, fmt.Errorf("already checked in")
		}
		// Re-open the checked-out session (accidental checkout recovery)
		return s.reopenSession(ctx, existingSession, staffID, status)
	}

	// Create new session
	now := time.Now()
	session := &activeModels.WorkSession{
		StaffID:      staffID,
		Date:         today,
		Status:       status,
		CheckInTime:  now,
		CheckOutTime: nil,
		BreakMinutes: 0,
		CreatedBy:    staffID,
	}

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session data: %w", err)
	}

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create work session: %w", err)
	}

	return session, nil
}

// reopenSession clears checkout on an existing session so the staff member can continue working
func (s *workSessionService) reopenSession(ctx context.Context, session *activeModels.WorkSession, staffID int64, status string) (*activeModels.WorkSession, error) {
	session.CheckOutTime = nil
	session.AutoCheckedOut = false
	session.Status = status
	session.UpdatedBy = &staffID

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session data: %w", err)
	}

	if err := s.repo.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to reopen session: %w", err)
	}

	return session, nil
}

// CheckOut ends the current work session for the staff member
func (s *workSessionService) CheckOut(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	// Get current active session
	session, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no active session found")
		}
		return nil, fmt.Errorf("failed to get current session: %w", err)
	}

	if session == nil {
		return nil, fmt.Errorf("no active session found")
	}

	// End any active break before checkout
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active break: %w", err)
	}
	if activeBreak != nil {
		now := time.Now()
		duration := int(math.Round(now.Sub(activeBreak.StartedAt).Minutes()))
		if err := s.breakRepo.EndBreak(ctx, activeBreak.ID, now, duration); err != nil {
			return nil, fmt.Errorf("failed to end active break: %w", err)
		}
		// Recalculate break_minutes cache
		if err := s.recalcBreakMinutes(ctx, session.ID); err != nil {
			return nil, fmt.Errorf("failed to update break minutes: %w", err)
		}
	}

	// Close the session using repository method
	now := time.Now()
	if err := s.repo.CloseSession(ctx, session.ID, now, false); err != nil {
		return nil, fmt.Errorf("failed to close session: %w", err)
	}

	// Re-fetch the updated session
	updatedSession, err := s.repo.FindByID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated session: %w", err)
	}

	return updatedSession, nil
}

// StartBreak starts a new break for the current session
func (s *workSessionService) StartBreak(ctx context.Context, staffID int64) (*activeModels.WorkSessionBreak, error) {
	// Get today's active session
	session, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no active session found")
		}
		return nil, fmt.Errorf("failed to get current session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("no active session found")
	}

	// Check no active break exists
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active break: %w", err)
	}
	if activeBreak != nil {
		return nil, fmt.Errorf("break already active")
	}

	// Create a new break
	now := time.Now()
	brk := &activeModels.WorkSessionBreak{
		SessionID: session.ID,
		StartedAt: now,
	}
	brk.CreatedAt = now
	brk.UpdatedAt = now

	if err := s.breakRepo.Create(ctx, brk); err != nil {
		return nil, fmt.Errorf("failed to create break: %w", err)
	}

	return brk, nil
}

// EndBreak ends the current active break for the staff member's session
func (s *workSessionService) EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	// Get today's active session
	session, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no active session found")
		}
		return nil, fmt.Errorf("failed to get current session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("no active session found")
	}

	// Find active break
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active break: %w", err)
	}
	if activeBreak == nil {
		return nil, fmt.Errorf("no active break found")
	}

	// End the break
	now := time.Now()
	duration := int(math.Round(now.Sub(activeBreak.StartedAt).Minutes()))
	if err := s.breakRepo.EndBreak(ctx, activeBreak.ID, now, duration); err != nil {
		return nil, fmt.Errorf("failed to end break: %w", err)
	}

	// Recalculate break_minutes cache on session
	if err := s.recalcBreakMinutes(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("failed to update break minutes: %w", err)
	}

	// Re-fetch updated session
	updatedSession, err := s.repo.FindByID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated session: %w", err)
	}

	return updatedSession, nil
}

// GetSessionBreaks returns all breaks for a given session
func (s *workSessionService) GetSessionBreaks(ctx context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
	breaks, err := s.breakRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session breaks: %w", err)
	}
	return breaks, nil
}

// recalcBreakMinutes sums all break durations for a session and updates the cache
func (s *workSessionService) recalcBreakMinutes(ctx context.Context, sessionID int64) error {
	breaks, err := s.breakRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get breaks for recalc: %w", err)
	}

	totalMinutes := 0
	for _, brk := range breaks {
		if brk.EndedAt != nil {
			totalMinutes += brk.DurationMinutes
		} else {
			// Active break: compute live duration
			totalMinutes += int(math.Round(time.Since(brk.StartedAt).Minutes()))
		}
	}

	return s.repo.UpdateBreakMinutes(ctx, sessionID, totalMinutes)
}

// UpdateSession updates a work session with the provided fields
func (s *workSessionService) UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates SessionUpdateRequest) (*activeModels.WorkSession, error) {
	// Get the session
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Verify ownership
	if session.StaffID != staffID {
		return nil, fmt.Errorf("can only update own sessions")
	}

	// Apply updates (only non-nil fields)
	if updates.CheckInTime != nil {
		session.CheckInTime = *updates.CheckInTime
	}
	if updates.CheckOutTime != nil {
		session.CheckOutTime = updates.CheckOutTime
	}
	if updates.BreakMinutes != nil {
		session.BreakMinutes = *updates.BreakMinutes
	}
	if updates.Status != nil {
		session.Status = *updates.Status
	}
	if updates.Notes != nil {
		session.Notes = *updates.Notes
	}

	session.UpdatedBy = &staffID

	// Validate the updated session
	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session data: %w", err)
	}

	// Update in database
	if err := s.repo.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return session, nil
}

// GetCurrentSession returns the current active session for a staff member
func (s *workSessionService) GetCurrentSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	session, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get current session: %w", err)
	}

	return session, nil
}

// GetHistory returns work sessions for a staff member in a date range
func (s *workSessionService) GetHistory(ctx context.Context, staffID int64, from, to time.Time) ([]*SessionResponse, error) {
	sessions, err := s.repo.GetHistoryByStaffID(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get session history: %w", err)
	}

	// Wrap each session in SessionResponse with calculated fields and breaks
	responses := make([]*SessionResponse, len(sessions))
	for i, session := range sessions {
		breaks, err := s.breakRepo.GetBySessionID(ctx, session.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get breaks for session %d: %w", session.ID, err)
		}

		responses[i] = &SessionResponse{
			WorkSession:      session,
			NetMinutes:       session.NetMinutes(),
			IsOvertime:       session.IsOvertime(),
			IsBreakCompliant: session.IsBreakCompliant(),
			Breaks:           breaks,
		}
	}

	return responses, nil
}

// GetTodayPresenceMap returns a map of staff IDs to their work status for today
func (s *workSessionService) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	presenceMap, err := s.repo.GetTodayPresenceMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get today's presence map: %w", err)
	}

	return presenceMap, nil
}

// CleanupOpenSessions closes all sessions that are still open before today
func (s *workSessionService) CleanupOpenSessions(ctx context.Context) (int, error) {
	// Get today's date (truncated)
	today := time.Now().Truncate(24 * time.Hour)

	// Get all open sessions before today
	openSessions, err := s.repo.GetOpenSessions(ctx, today)
	if err != nil {
		return 0, fmt.Errorf("failed to get open sessions: %w", err)
	}

	count := 0
	for _, session := range openSessions {
		// Set check-out time to 23:59:59 of the session date
		endOfDay := session.Date.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

		if err := s.repo.CloseSession(ctx, session.ID, endOfDay, true); err != nil {
			return count, fmt.Errorf("failed to close session %d: %w", session.ID, err)
		}
		count++
	}

	return count, nil
}

// EnsureCheckedIn ensures a staff member is checked in, creating a session if needed
func (s *workSessionService) EnsureCheckedIn(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	// Check if already checked in
	currentSession, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check current session: %w", err)
	}

	if currentSession != nil && currentSession.IsActive() {
		return currentSession, nil
	}

	// Check if there's already a checked-out session today
	today := time.Now().Truncate(24 * time.Hour)
	todaySession, err := s.repo.GetByStaffAndDate(ctx, staffID, today)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check today's session: %w", err)
	}

	if todaySession != nil {
		// Already checked out today, don't re-check-in
		return nil, nil
	}

	// No session today, create one
	return s.CheckIn(ctx, staffID, activeModels.WorkSessionStatusPresent)
}
