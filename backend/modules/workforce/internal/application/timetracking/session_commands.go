package timetracking

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// DayExportRows exposes the single-staff export's merged session/absence rows.
// It delegates to the batched path so both export forms use the same loading
// and rendering code.
func (s *workSessionCommands) DayExportRows(ctx context.Context, staffID int64, from, to calendar.Date) (value0 []DayExportRow, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.DayExportRows(txCtx, staffID, from, to)
		return err
	})
	return
}

// DayExportRowsByStaffIDs builds all cross-staff evidence rows with a constant
// number of repository calls. It intentionally skips weekly summaries because
// the export only consumes the per-session cells.
func (s *workSessionCommands) DayExportRowsByStaffIDs(ctx context.Context, staffIDs []int64, from, to calendar.Date) (value0 map[int64][]DayExportRow, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.DayExportRowsByStaffIDs(txCtx, staffIDs, from, to)
		return err
	})
	return
}

// StartBreak starts a new break on the running block, whichever day it was
// opened on (see openBlockDay).
// If plannedDurationMinutes is provided (1-240), sets planned_end_time for auto-end
func (s *workSessionCommands) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (value0 *WorkSessionBreak, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.StartBreak(txCtx, staffID, plannedDurationMinutes)
		return err
	})
	return
}

// StartBreakOn starts a break on the open session of an explicit calendar day.
func (s *workSessionCommands) StartBreakOn(ctx context.Context, staffID int64, day calendar.Date, plannedDurationMinutes *int) (value0 *WorkSessionBreak, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.StartBreakOn(txCtx, staffID, day, plannedDurationMinutes)
		return err
	})
	return
}

// EndBreak ends the active break on the running block, whichever day it was
// opened on (see openBlockDay).
func (s *workSessionCommands) EndBreak(ctx context.Context, staffID int64) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.EndBreak(txCtx, staffID)
		return err
	})
	return
}

// EndBreakOn ends the active break on the open session of an explicit calendar day.
func (s *workSessionCommands) EndBreakOn(ctx context.Context, staffID int64, day calendar.Date) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.EndBreakOn(txCtx, staffID, day)
		return err
	})
	return
}

// GetSessionBreaks returns all breaks for a given session
func (s *workSessionCommands) GetSessionBreaks(ctx context.Context, staffID, sessionID int64) (value0 []*WorkSessionBreak, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetSessionBreaks(txCtx, staffID, sessionID)
		return err
	})
	return
}

// AutoEndExpiredBreaks ends all breaks whose planned_end_time has passed
// Returns the number of breaks that were ended
func (s *workSessionCommands) AutoEndExpiredBreaks(ctx context.Context) (value0 int, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.AutoEndExpiredBreaks(txCtx)
		return err
	})
	return
}

// CheckIn creates a new work session for the staff member.
// Status must be explicitly chosen. Empty values are rejected so the caller
// (HTTP handler or internal worker) cannot accidentally fall back to "present".
func (s *workSessionCommands) CheckIn(ctx context.Context, staffID int64, status, source, reason string) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CheckIn(txCtx, staffID, status, source, reason)
		return err
	})
	return
}

// CheckInOn is the day-pinned entry point of the kiosk, kept symmetric with
// CheckOutOn. The pinned day no longer selects anything: the open-block guard
// in checkIn scans every day (a block opened before Berlin midnight is still
// open after it), and a session that is created fresh is filed on the day its
// own stamp falls on — see checkIn.
func (s *workSessionCommands) CheckInOn(ctx context.Context, staffID int64, pinnedDate calendar.Date, status, source, reason string) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CheckInOn(txCtx, staffID, pinnedDate, status, source, reason)
		return err
	})
	return
}

// EnsureCheckedIn ensures a staff member is checked in, creating a session if needed.
// `source` is forwarded to CheckIn so the channel is recorded faithfully.
func (s *workSessionCommands) EnsureCheckedIn(ctx context.Context, staffID int64, source string) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.EnsureCheckedIn(txCtx, staffID, source)
		return err
	})
	return
}

// CheckOut ends the running work session of the staff member, whichever day it
// was opened on. Resolving the day from the clock instead would strand a block
// that crossed Berlin midnight: check-in refuses while it is open, so a
// today-only checkout would leave no way to close it.
func (s *workSessionCommands) CheckOut(ctx context.Context, staffID int64, reason string) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CheckOut(txCtx, staffID, reason)
		return err
	})
	return
}

// CheckOutOn ends the open session of an explicit calendar day.
func (s *workSessionCommands) CheckOutOn(ctx context.Context, staffID int64, day calendar.Date, reason string) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CheckOutOn(txCtx, staffID, day, reason)
		return err
	})
	return
}

// ExportSessions renders the single-staff export of work sessions and
// absences. CSV and XLSX stay plain data files (shared writers below);
// PDF renders through the listexport design (#1568) as the print artifact.
func (s *workSessionCommands) ExportSessions(ctx context.Context, staffID int64, from, to calendar.Date, format string) (value0 *ExportFile, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.ExportSessions(txCtx, staffID, from, to, format)
		return err
	})
	return
}

// GetCurrentSession returns the current active session for a staff member
func (s *workSessionCommands) GetCurrentSession(ctx context.Context, staffID int64) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetCurrentSession(txCtx, staffID)
		return err
	})
	return
}

// GetLatestOpenSession returns the most recent still-running session of a staff
// member across all days, or nil when none is open. It answers "is this person
// clocked in right now" without assuming the session was opened today — a night
// stamp survives the Berlin midnight rollover.
func (s *workSessionCommands) GetLatestOpenSession(ctx context.Context, staffID int64) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetLatestOpenSession(txCtx, staffID)
		return err
	})
	return
}

// GetHistory returns work sessions for a staff member in a date range with weekly aggregation
func (s *workSessionCommands) GetHistory(ctx context.Context, staffID int64, from, to calendar.Date) (value0 *HistoryResponse, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetHistory(txCtx, staffID, from, to)
		return err
	})
	return
}

// GetHistoryIntersecting returns blocks that overlap a calendar range. Every
// consumer that lays minutes out on calendar days uses it: the history tables,
// the kiosk day state, the balances. GetHistory keeps the stored-date contract
// for the export, where a block is a single row under its start day.
func (s *workSessionCommands) GetHistoryIntersecting(ctx context.Context, staffID int64, from, to calendar.Date) (value0 *HistoryResponse, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetHistoryIntersecting(txCtx, staffID, from, to)
		return err
	})
	return
}

// GetSessionEdits returns the audit trail for a work session
func (s *workSessionCommands) GetSessionEdits(ctx context.Context, staffID, sessionID int64) (value0 []*WorkSessionEditView, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetSessionEdits(txCtx, staffID, sessionID)
		return err
	})
	return
}

// GetSessionEditsForStaff is the admin-facing counterpart. The session must
// belong to the staff member named in the URL. Without that check an admin
// could load any session's edits by guessing IDs (within the tenant, RLS
// still applies). Permission gating happens at the route level.
func (s *workSessionCommands) GetSessionEditsForStaff(ctx context.Context, staffID, sessionID int64) (value0 []*WorkSessionEditView, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetSessionEditsForStaff(txCtx, staffID, sessionID)
		return err
	})
	return
}

// GetTodayPresenceMap returns a map of staff IDs to their work status for today
func (s *workSessionCommands) GetTodayPresenceMap(ctx context.Context) (value0 map[int64]string, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetTodayPresenceMap(txCtx)
		return err
	})
	return
}

// CleanupOpenSessions closes sessions still open from before yesterday. This
// leaves a full following calendar day for legitimate night blocks to be
// checked out normally before they become stale.
func (s *workSessionCommands) CleanupOpenSessions(ctx context.Context) (value0 int, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CleanupOpenSessions(txCtx)
		return err
	})
	return
}

// AutoCheckoutDueSessions closes open sessions at their planned shift end (#1798).
func (s *workSessionCommands) AutoCheckoutDueSessions(ctx context.Context, grace time.Duration) (value0 int, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.AutoCheckoutDueSessions(txCtx, grace)
		return err
	})
	return
}

// GetStaffIDsWithSupervisionToday returns staff IDs with supervision activity today.
func (s *workSessionCommands) GetStaffIDsWithSupervisionToday(ctx context.Context) (value0 []int64, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetStaffIDsWithSupervisionToday(txCtx)
		return err
	})
	return
}

// AssignScheduleTemplate snapshots the template's entries as the staff
// member's schedule and binds the template to the staff row.
func (s *workSessionCommands) AssignScheduleTemplate(ctx context.Context, staff *StaffScheduleBinding, modelID int64) (err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		err = s.commands.AssignScheduleTemplate(txCtx, staff, modelID)
		return err
	})
	return
}

// ApplyCustomScheduleRows replaces the schedule with custom rows and unbinds
// any assigned template.
func (s *workSessionCommands) ApplyCustomScheduleRows(ctx context.Context, staff *StaffScheduleBinding, entries []*WorkScheduleRow, anchor, validFrom calendar.Date) (err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		err = s.commands.ApplyCustomScheduleRows(txCtx, staff, entries, anchor, validFrom)
		return err
	})
	return
}

// SaveCustomScheduleAsTemplate persists the rows as a new reusable work time
// model and binds it to the staff member.
func (s *workSessionCommands) SaveCustomScheduleAsTemplate(ctx context.Context, staff *StaffScheduleBinding, name string, rotation int, anchor calendar.Date, entries []*WorkTimeTemplateEntry) (err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		err = s.commands.SaveCustomScheduleAsTemplate(txCtx, staff, name, rotation, anchor, entries)
		return err
	})
	return
}

// UpdateSchedule resolves the requested mode and applies the schedule change.
func (s *workSessionCommands) UpdateSchedule(ctx context.Context, staff *StaffScheduleBinding, in ScheduleUpdateInput) (err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		err = s.commands.UpdateSchedule(txCtx, staff, in)
		return err
	})
	return
}

// UpdateSession updates a work session with the provided fields and creates
// audit entries. Self-edit path: the requesting staff must own the session.
// Notes are only required when changing status (Vor Ort ↔ Homeoffice).
func (s *workSessionCommands) UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates SessionUpdateRequest) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.UpdateSession(txCtx, staffID, sessionID, updates)
		return err
	})
	return
}

// UpdateSessionAsAdmin applies an admin correction. editorStaffID is the
// admin doing the edit (goes into edited_by). targetStaffID is the owner of
// the session we expect to mutate, verified against session.StaffID so the
// route can't be used to leak edits across staff in the same tenant.
//
// Notes are unconditionally required: BAG demands "Verlässlichkeit" of the
// audit trail, and any foreign edit needs a reason. We're stricter than
// self-edit on purpose.
func (s *workSessionCommands) UpdateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID, sessionID int64, updates SessionUpdateRequest) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.UpdateSessionAsAdmin(txCtx, editorStaffID, targetStaffID, sessionID, updates)
		return err
	})
	return
}

// CreateSessionAsAdmin records a "nachgetragene" session for another staff
// member. Each field that ends up on the new row is also captured as an
// audit edit (old_value = NULL, new_value = field) so the MA-side audit log
// shows the create as a series of explicit "field war leer → field ist jetzt
// X" entries. Notes are stored both on the session and on every audit row
// (consistent with edit semantics).
func (s *workSessionCommands) CreateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID int64, req AdminCreateSessionRequest) (value0 *WorkSession, err error) {
	err = s.commands.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CreateSessionAsAdmin(txCtx, editorStaffID, targetStaffID, req)
		return err
	})
	return
}
