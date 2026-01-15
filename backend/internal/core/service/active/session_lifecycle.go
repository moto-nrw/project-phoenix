package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/uptrace/bun"
)

// Session Lifecycle Management
// This file contains methods for managing session timeouts, cleanup, and daily session endings.

// visitSSEData holds data needed for SSE broadcasts after a visit is ended
type visitSSEData struct {
	VisitID   int64
	StudentID int64
	Name      string
	Student   *userModels.Student
}

// ProcessSessionTimeout handles device-triggered session timeout
func (s *service) ProcessSessionTimeout(ctx context.Context, deviceID int64) (*TimeoutResult, error) {
	// Validate device has active session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeout", Err: ErrNoActiveSession}
	}

	// Delegate to ProcessSessionTimeoutByID with the session ID
	return s.ProcessSessionTimeoutByID(ctx, session.ID)
}

// validateSessionForTimeout validates that a session exists and is still active.
// Returns the session if valid, or an error if not found or already ended.
func (s *service) validateSessionForTimeout(ctx context.Context, sessionID int64) (*active.Group, error) {
	session, err := s.groupRepo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: ErrActiveGroupNotFound}
	}

	if !session.IsActive() {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: ErrActiveGroupAlreadyEnded}
	}

	return session, nil
}

// checkoutActiveVisits ends all active visits for a session and returns the count of students checked out.
func (s *service) checkoutActiveVisits(ctx context.Context, sessionID int64) (int, error) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	studentsCheckedOut := 0
	for _, visit := range visits {
		if !visit.IsActive() {
			continue
		}
		if err := s.visitRepo.EndVisit(ctx, visit.ID); err != nil {
			return 0, err
		}
		studentsCheckedOut++
	}

	return studentsCheckedOut, nil
}

// collectActiveVisitsForSSE gathers visit and student data needed for SSE broadcasts
func (s *service) collectActiveVisitsForSSE(ctx context.Context, sessionID int64) ([]visitSSEData, error) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var result []visitSSEData
	for _, visit := range visits {
		if !visit.IsActive() {
			continue
		}
		data := visitSSEData{
			VisitID:   visit.ID,
			StudentID: visit.StudentID,
		}
		// Query student name for SSE event
		if student, err := s.studentRepo.FindByID(ctx, visit.StudentID); err == nil && student != nil {
			data.Student = student
			if person, err := s.personRepo.FindByID(ctx, student.PersonID); err == nil && person != nil {
				data.Name = fmt.Sprintf("%s %s", person.FirstName, person.LastName)
			}
		}
		result = append(result, data)
	}

	return result, nil
}

// ProcessSessionTimeoutByID handles session timeout by session ID directly.
// This is the preferred method for cleanup operations to avoid TOCTOU race conditions.
// It verifies the session is still active before ending it.
func (s *service) ProcessSessionTimeoutByID(ctx context.Context, sessionID int64) (*TimeoutResult, error) {
	// Collect visit data BEFORE transaction for SSE broadcasts
	visitsToNotify, err := s.collectActiveVisitsForSSE(ctx, sessionID)
	if err != nil {
		// Non-fatal: continue without SSE data
		visitsToNotify = nil
	}

	var result *TimeoutResult
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*service)

		session, err := txService.validateSessionForTimeout(ctx, sessionID)
		if err != nil {
			return err
		}

		studentsCheckedOut, err := txService.checkoutActiveVisits(ctx, sessionID)
		if err != nil {
			return err
		}

		if err := txService.groupRepo.EndSession(ctx, sessionID); err != nil {
			return err
		}

		result = &TimeoutResult{
			SessionID:          sessionID,
			ActivityID:         session.GroupID,
			StudentsCheckedOut: studentsCheckedOut,
			TimeoutAt:          time.Now(),
		}
		return nil
	})

	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return nil, activeErr
		}
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: err}
	}

	// Broadcast SSE events (fire-and-forget, outside transaction)
	if s.broadcaster != nil && result != nil {
		sessionIDStr := fmt.Sprintf("%d", sessionID)
		s.broadcastStudentCheckoutEvents(sessionIDStr, visitsToNotify)
		s.broadcastActivityEndEvent(ctx, sessionID, sessionIDStr)
	}

	return result, nil
}

// UpdateSessionActivity updates the last activity timestamp for a session
func (s *service) UpdateSessionActivity(ctx context.Context, activeGroupID int64) error {
	// Get the current session to validate it exists and is active
	session, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "UpdateSessionActivity", Err: err}
	}

	if session == nil {
		return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupNotFound}
	}

	if !session.IsActive() {
		return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupAlreadyEnded}
	}

	// Update last activity timestamp
	return s.groupRepo.UpdateLastActivity(ctx, activeGroupID, time.Now())
}

// ValidateSessionTimeout validates if a timeout request is valid
func (s *service) ValidateSessionTimeout(ctx context.Context, deviceID int64, timeoutMinutes int) error {
	// Validate device has active session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return &ActiveError{Op: "ValidateSessionTimeout", Err: err}
	}

	// Validate timeout parameters
	if timeoutMinutes <= 0 || timeoutMinutes > 480 { // Max 8 hours
		return &ActiveError{Op: "ValidateSessionTimeout", Err: fmt.Errorf("invalid timeout minutes: %d", timeoutMinutes)}
	}

	// Check if session is actually timed out based on inactivity
	timeoutDuration := time.Duration(timeoutMinutes) * time.Minute
	inactivityDuration := time.Since(session.LastActivity)

	if inactivityDuration < timeoutDuration {
		return &ActiveError{Op: "ValidateSessionTimeout", Err: fmt.Errorf("session not yet timed out: %v remaining", timeoutDuration-inactivityDuration)}
	}

	return nil
}

// GetSessionTimeoutInfo provides comprehensive timeout information for a device session
func (s *service) GetSessionTimeoutInfo(ctx context.Context, deviceID int64) (*SessionTimeoutInfo, error) {
	// Get current session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "GetSessionTimeoutInfo", Err: err}
	}

	// Count active students in the session
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, session.ID)
	if err != nil {
		return nil, &ActiveError{Op: "GetSessionTimeoutInfo", Err: err}
	}

	activeStudentCount := 0
	for _, visit := range visits {
		if visit.IsActive() {
			activeStudentCount++
		}
	}

	info := &SessionTimeoutInfo{
		SessionID:          session.ID,
		ActivityID:         session.GroupID,
		StartTime:          session.StartTime,
		LastActivity:       session.LastActivity,
		TimeoutMinutes:     session.TimeoutMinutes,
		InactivityDuration: session.GetInactivityDuration(),
		TimeUntilTimeout:   session.GetTimeUntilTimeout(),
		IsTimedOut:         session.IsTimedOut(),
		ActiveStudentCount: activeStudentCount,
	}

	return info, nil
}

// CleanupAbandonedSessions cleans up sessions that have been abandoned for longer than the specified duration.
// A session is considered abandoned if:
// 1. No activity (RFID scans or device pings) for longer than the threshold, AND
// 2. The device is offline (not pinging)
// This ensures sessions stay alive if either there's activity OR the device is still online.
func (s *service) CleanupAbandonedSessions(ctx context.Context, threshold time.Duration) (int, error) {
	// Find sessions with no activity since the threshold
	cutoffTime := time.Now().Add(-threshold)
	sessions, err := s.groupRepo.FindActiveSessionsOlderThan(ctx, cutoffTime)
	if err != nil {
		return 0, &ActiveError{Op: "CleanupAbandonedSessions", Err: err}
	}

	cleanedCount := 0
	for _, session := range sessions {
		// Session is abandoned only if BOTH conditions are true:
		// 1. No recent activity (already filtered by query)
		// 2. Device is offline (not pinging)
		deviceOnline := session.Device != nil && session.Device.IsOnline()
		if deviceOnline {
			// Device is still pinging - session stays alive
			continue
		}

		// Both conditions met: no activity AND device offline - clean up
		// Use ProcessSessionTimeoutByID with the session ID directly to prevent TOCTOU race condition
		// This ensures we end the exact session we identified as abandoned, not whatever
		// session happens to be current for the device at cleanup time
		_, err := s.ProcessSessionTimeoutByID(ctx, session.ID)
		if err != nil {
			// Log error but continue with other sessions
			// Note: ErrActiveGroupAlreadyEnded is expected if session was ended between
			// identification and cleanup - this is the race condition we're protecting against
			continue
		}
		cleanedCount++
	}

	return cleanedCount, nil
}

// EndDailySessions ends all active sessions at the end of the day
func (s *service) EndDailySessions(ctx context.Context) (*DailySessionCleanupResult, error) {
	result := &DailySessionCleanupResult{
		ExecutedAt: time.Now(),
		Success:    true,
		Errors:     make([]string, 0),
	}

	activeGroups, err := s.groupRepo.List(ctx, nil)
	if err != nil {
		result.Success = false
		return result, &ActiveError{Op: "EndDailySessions", Err: ErrDatabaseOperation}
	}

	for _, group := range activeGroups {
		if group.IsActive() {
			s.processGroupForDailyCleanup(ctx, group, result)
		}
	}

	return result, nil
}

// processGroupForDailyCleanup processes a single group for daily cleanup
func (s *service) processGroupForDailyCleanup(ctx context.Context, group *active.Group, result *DailySessionCleanupResult) {
	// Track error count before visit cleanup
	errorCountBefore := len(result.Errors)

	s.endActiveVisitsForGroup(ctx, group.ID, result)

	// If visit cleanup failed (e.g., database error fetching visits),
	// skip session and supervisor cleanup to maintain data consistency.
	// We don't want to end the session/supervisors while visits remain active.
	if len(result.Errors) > errorCountBefore {
		return
	}

	s.endGroupSession(ctx, group, result)
	s.endActiveSupervisorsForGroup(ctx, group.ID, result)
}

// endActiveVisitsForGroup ends all active visits for a group
func (s *service) endActiveVisitsForGroup(ctx context.Context, groupID int64, result *DailySessionCleanupResult) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, groupID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get visits for group %d: %v", groupID, err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
		return
	}

	for _, visit := range visits {
		if !visit.IsActive() {
			continue
		}

		visit.EndVisit()
		if err := s.visitRepo.Update(ctx, visit); err != nil {
			errMsg := fmt.Sprintf("Failed to end visit %d: %v", visit.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
		} else {
			result.VisitsEnded++
		}
	}
}

// endGroupSession ends a group session
func (s *service) endGroupSession(ctx context.Context, group *active.Group, result *DailySessionCleanupResult) {
	group.EndSession()
	if err := s.groupRepo.Update(ctx, group); err != nil {
		errMsg := fmt.Sprintf("Failed to end group session %d: %v", group.ID, err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
	} else {
		result.SessionsEnded++
	}
}

// endActiveSupervisorsForGroup ends all active supervisors for a group
func (s *service) endActiveSupervisorsForGroup(ctx context.Context, groupID int64, result *DailySessionCleanupResult) {
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, groupID, true)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get supervisors for group %d: %v", groupID, err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
		return
	}

	now := time.Now()
	for _, supervisor := range supervisors {
		supervisor.EndDate = &now
		if err := s.supervisorRepo.Update(ctx, supervisor); err != nil {
			errMsg := fmt.Sprintf("Failed to end supervisor %d: %v", supervisor.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
		} else {
			result.SupervisorsEnded++
		}
	}
}
