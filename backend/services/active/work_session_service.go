package active

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/xuri/excelize/v2"
)

// BreakDurationUpdate represents an update to a single break's duration
type BreakDurationUpdate struct {
	ID              int64 `json:"id"`
	DurationMinutes int   `json:"duration_minutes"`
}

// SessionUpdateRequest defines the structure for updating a work session
type SessionUpdateRequest struct {
	CheckInTime  *time.Time            `json:"check_in_time"`
	CheckOutTime *time.Time            `json:"check_out_time"`
	BreakMinutes *int                  `json:"break_minutes"`
	Status       *string               `json:"status"`
	Notes        *string               `json:"notes"`
	Breaks       []BreakDurationUpdate `json:"breaks"`
}

// SessionResponse wraps a work session with calculated fields
type SessionResponse struct {
	*activeModels.WorkSession
	NetMinutes       int                              `json:"net_minutes"`
	IsOvertime       bool                             `json:"is_overtime"`
	IsBreakCompliant bool                             `json:"is_break_compliant"`
	Breaks           []*activeModels.WorkSessionBreak `json:"breaks"`
	EditCount        int                              `json:"edit_count"`
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
	GetSessionEdits(ctx context.Context, sessionID int64) ([]*auditModels.WorkSessionEdit, error)
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)
	CleanupOpenSessions(ctx context.Context) (int, error)
	EnsureCheckedIn(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	ExportSessions(ctx context.Context, staffID int64, from, to time.Time, format string) ([]byte, string, error)
}

// workSessionService implements WorkSessionService
type workSessionService struct {
	repo      activeModels.WorkSessionRepository
	breakRepo activeModels.WorkSessionBreakRepository
	auditRepo auditModels.WorkSessionEditRepository
}

// NewWorkSessionService creates a new work session service
func NewWorkSessionService(repo activeModels.WorkSessionRepository, breakRepo activeModels.WorkSessionBreakRepository, auditRepo auditModels.WorkSessionEditRepository) WorkSessionService {
	return &workSessionService{repo: repo, breakRepo: breakRepo, auditRepo: auditRepo}
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

// UpdateSession updates a work session with the provided fields and creates audit entries
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

	// Snapshot old values before applying updates
	now := time.Now()
	var auditEdits []*auditModels.WorkSessionEdit

	makeEdit := func(field string, oldVal, newVal *string) {
		auditEdits = append(auditEdits, &auditModels.WorkSessionEdit{
			SessionID: sessionID,
			StaffID:   session.StaffID,
			EditedBy:  staffID,
			FieldName: field,
			OldValue:  oldVal,
			NewValue:  newVal,
			Notes:     updates.Notes,
			CreatedAt: now,
		})
	}

	strPtr := func(s string) *string { return &s }

	if updates.CheckInTime != nil {
		oldVal := session.CheckInTime.Format(time.RFC3339)
		newVal := updates.CheckInTime.Format(time.RFC3339)
		if oldVal != newVal {
			makeEdit(auditModels.FieldCheckInTime, strPtr(oldVal), strPtr(newVal))
		}
		session.CheckInTime = *updates.CheckInTime
	}
	if updates.CheckOutTime != nil {
		var oldVal string
		if session.CheckOutTime != nil {
			oldVal = session.CheckOutTime.Format(time.RFC3339)
		}
		newVal := updates.CheckOutTime.Format(time.RFC3339)
		if oldVal != newVal {
			makeEdit(auditModels.FieldCheckOutTime, strPtr(oldVal), strPtr(newVal))
		}
		session.CheckOutTime = updates.CheckOutTime
	}
	// If individual break updates are provided, process them instead of BreakMinutes
	if len(updates.Breaks) > 0 {
		// Load all breaks for this session
		sessionBreaks, err := s.breakRepo.GetBySessionID(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to load session breaks: %w", err)
		}

		// Build lookup map
		breakMap := make(map[int64]*activeModels.WorkSessionBreak, len(sessionBreaks))
		for _, b := range sessionBreaks {
			breakMap[b.ID] = b
		}

		for _, bu := range updates.Breaks {
			brk, ok := breakMap[bu.ID]
			if !ok {
				return nil, fmt.Errorf("break %d does not belong to this session", bu.ID)
			}
			if brk.EndedAt == nil {
				return nil, fmt.Errorf("cannot edit duration of an active break")
			}

			oldDuration := brk.DurationMinutes
			if oldDuration != bu.DurationMinutes {
				// Calculate new ended_at from started_at + new duration
				newEndedAt := brk.StartedAt.Add(time.Duration(bu.DurationMinutes) * time.Minute)

				if err := s.breakRepo.UpdateDuration(ctx, bu.ID, bu.DurationMinutes, newEndedAt); err != nil {
					return nil, fmt.Errorf("failed to update break %d: %w", bu.ID, err)
				}

				// Audit entry for this break change
				oldVal := strconv.Itoa(oldDuration)
				newVal := strconv.Itoa(bu.DurationMinutes)
				makeEdit(auditModels.FieldBreakDuration, strPtr(oldVal), strPtr(newVal))
			}
		}

		// Recalculate session break_minutes cache from updated breaks
		if err := s.recalcBreakMinutes(ctx, sessionID); err != nil {
			return nil, fmt.Errorf("failed to recalculate break minutes: %w", err)
		}
		// Re-fetch session to get updated break_minutes
		session, err = s.repo.FindByID(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to re-fetch session: %w", err)
		}
	} else if updates.BreakMinutes != nil {
		oldVal := strconv.Itoa(session.BreakMinutes)
		newVal := strconv.Itoa(*updates.BreakMinutes)
		if oldVal != newVal {
			makeEdit(auditModels.FieldBreakMinutes, strPtr(oldVal), strPtr(newVal))
		}
		session.BreakMinutes = *updates.BreakMinutes
	}
	if updates.Status != nil {
		if session.Status != *updates.Status {
			makeEdit(auditModels.FieldStatus, strPtr(session.Status), updates.Status)
		}
		session.Status = *updates.Status
	}
	if updates.Notes != nil {
		if session.Notes != *updates.Notes {
			makeEdit(auditModels.FieldNotes, strPtr(session.Notes), updates.Notes)
		}
		session.Notes = *updates.Notes
	}

	session.UpdatedBy = &staffID
	session.UpdatedAt = now

	// Validate the updated session
	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session data: %w", err)
	}

	// Update in database
	if err := s.repo.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	// Create audit entries for changed fields
	if len(auditEdits) > 0 {
		if err := s.auditRepo.CreateBatch(ctx, auditEdits); err != nil {
			return nil, fmt.Errorf("failed to create audit entries: %w", err)
		}
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

	// Collect session IDs for batch edit count query
	sessionIDs := make([]int64, len(sessions))
	for i, session := range sessions {
		sessionIDs[i] = session.ID
	}

	// Batch fetch edit counts
	editCounts, err := s.auditRepo.CountBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get edit counts: %w", err)
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
			EditCount:        editCounts[session.ID],
		}
	}

	return responses, nil
}

// GetSessionEdits returns the audit trail for a work session
func (s *workSessionService) GetSessionEdits(ctx context.Context, sessionID int64) ([]*auditModels.WorkSessionEdit, error) {
	edits, err := s.auditRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session edits: %w", err)
	}
	return edits, nil
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

// German weekday names for export
var germanWeekdays = [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

// ExportSessions generates a CSV or XLSX export of work sessions
func (s *workSessionService) ExportSessions(ctx context.Context, staffID int64, from, to time.Time, format string) ([]byte, string, error) {
	sessions, err := s.GetHistory(ctx, staffID, from, to)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get sessions for export: %w", err)
	}

	fromStr := from.Format("2006-01-02")
	toStr := to.Format("2006-01-02")

	switch format {
	case "xlsx":
		data, err := s.exportXLSX(sessions)
		if err != nil {
			return nil, "", err
		}
		return data, fmt.Sprintf("zeiterfassung_%s_%s.xlsx", fromStr, toStr), nil
	default:
		data, err := s.exportCSV(sessions)
		if err != nil {
			return nil, "", err
		}
		return data, fmt.Sprintf("zeiterfassung_%s_%s.csv", fromStr, toStr), nil
	}
}

func (s *workSessionService) exportCSV(sessions []*SessionResponse) ([]byte, error) {
	var buf bytes.Buffer

	// UTF-8 BOM for Excel compatibility
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	w.Comma = ';'

	// Header
	if err := w.Write([]string{"Datum", "Wochentag", "Start", "Ende", "Pause (Min)", "Netto (Std)", "Ort", "Bemerkungen"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, sr := range sessions {
		row := s.sessionToRow(sr)
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("CSV write error: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *workSessionService) exportXLSX(sessions []*SessionResponse) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := "Zeiterfassung"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	// Remove default "Sheet1" if it exists and is different
	if sheet != "Sheet1" {
		_ = f.DeleteSheet("Sheet1")
	}

	headers := []string{"Datum", "Wochentag", "Start", "Ende", "Pause (Min)", "Netto (Std)", "Ort", "Bemerkungen"}

	// Header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1},
	})

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	// Data rows
	for rowIdx, sr := range sessions {
		row := s.sessionToRow(sr)
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheet, cell, val)
		}
	}

	// Auto-width columns
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, 16)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to write XLSX: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *workSessionService) sessionToRow(sr *SessionResponse) []string {
	sess := sr.WorkSession

	datum := sess.Date.Format("02.01.2006")
	wochentag := germanWeekdays[sess.Date.Weekday()]

	start := sess.CheckInTime.Format("15:04")

	ende := ""
	if sess.CheckOutTime != nil {
		ende = sess.CheckOutTime.Format("15:04")
	}

	pauseMin := strconv.Itoa(sess.BreakMinutes)

	// Net as "Xh YYmin"
	netMins := sr.NetMinutes
	h := netMins / 60
	m := netMins % 60
	netto := fmt.Sprintf("%dh %02dmin", h, m)

	ort := "In der OGS"
	if sess.Status == activeModels.WorkSessionStatusHomeOffice {
		ort = "Homeoffice"
	}

	return []string{datum, wochentag, start, ende, pauseMin, netto, ort, sess.Notes}
}
