package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	NetMinutes       int  `json:"net_minutes"`
	IsOvertime       bool `json:"is_overtime"`
	IsBreakCompliant bool `json:"is_break_compliant"`
}

// WorkSessionService defines operations for staff time tracking
type WorkSessionService interface {
	CheckIn(ctx context.Context, staffID int64, status string) (*activeModels.WorkSession, error)
	CheckOut(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	UpdateBreakMinutes(ctx context.Context, staffID int64, minutes int) (*activeModels.WorkSession, error)
	UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates SessionUpdateRequest) (*activeModels.WorkSession, error)
	GetCurrentSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	GetHistory(ctx context.Context, staffID int64, from, to time.Time) ([]*SessionResponse, error)
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)
	CleanupOpenSessions(ctx context.Context) (int, error)
	EnsureCheckedIn(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
}

// workSessionService implements WorkSessionService
type workSessionService struct {
	repo activeModels.WorkSessionRepository
}

// NewWorkSessionService creates a new work session service
func NewWorkSessionService(repo activeModels.WorkSessionRepository) WorkSessionService {
	return &workSessionService{repo: repo}
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

// UpdateBreakMinutes updates the break minutes for the current session
func (s *workSessionService) UpdateBreakMinutes(ctx context.Context, staffID int64, minutes int) (*activeModels.WorkSession, error) {
	if minutes < 0 {
		return nil, fmt.Errorf("break minutes cannot be negative")
	}

	// Get today's session
	today := time.Now().Truncate(24 * time.Hour)
	session, err := s.repo.GetByStaffAndDate(ctx, staffID, today)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no session found for today")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session == nil {
		return nil, fmt.Errorf("no session found for today")
	}

	// Update break minutes
	session.BreakMinutes = minutes
	session.UpdatedBy = &staffID

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session data: %w", err)
	}

	if err := s.repo.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update break minutes: %w", err)
	}

	return session, nil
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

	// Wrap each session in SessionResponse with calculated fields
	responses := make([]*SessionResponse, len(sessions))
	for i, session := range sessions {
		responses[i] = &SessionResponse{
			WorkSession:      session,
			NetMinutes:       session.NetMinutes(),
			IsOvertime:       session.IsOvertime(),
			IsBreakCompliant: session.IsBreakCompliant(),
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
