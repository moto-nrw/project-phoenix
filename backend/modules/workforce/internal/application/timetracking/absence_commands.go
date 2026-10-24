package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PreviewCompTimeBalance projects what a comp_time absence over [start, end]
// does to the Stundenkonto (#2873). It replaces the former overdraft
// rejection: the numbers inform the Leitung before the deliberate confirm in
// the frontend, they never block the booking.
func (s *absenceCommands) PreviewCompTimeBalance(
	ctx context.Context,
	staffID int64,
	start, end calendar.Date,
	halfDay bool,
) (value0 *CompTimeBalancePreview, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.PreviewCompTimeBalance(txCtx, staffID, start, end, halfDay)
		return err
	})
	return
}

// CreateAbsence creates a new absence record for the caller themself.
func (s *absenceCommands) CreateAbsence(ctx context.Context, staffID int64, req CreateAbsenceRequest) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CreateAbsence(txCtx, staffID, req)
		return err
	})
	return
}

// CreateOwnAbsence is the self-service entry point. Comp-time absences change
// the Stundenkonto and therefore require the manager-authorized staff route.
func (s *absenceCommands) CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, req CreateAbsenceRequest) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CreateOwnAbsence(txCtx, staffID, actorAccountID, req)
		return err
	})
	return
}

// CreateAbsenceFor creates an absence for subjectStaffID on createdByStaffID's
// behalf (#1843 admin sick reporting; self-service passes the same id twice).
// A full-day sick absence cascades into the plans; the merge path cascades
// over the MERGED range of the primary row, which the idempotency guards in
// the syncer make safe to re-run.
func (s *absenceCommands) CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, req CreateAbsenceRequest) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CreateAbsenceFor(txCtx, subjectStaffID, createdByStaffID, actorAccountID, req)
		return err
	})
	return
}

// UpdateAbsence updates an existing absence record
func (s *absenceCommands) UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, req UpdateAbsenceRequest) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.UpdateAbsence(txCtx, staffID, actorAccountID, absenceID, req)
		return err
	})
	return
}

// DeleteAbsence deletes an absence record (self-service; the caller is the
// absence's owner and the acting person).
func (s *absenceCommands) DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.DeleteAbsence(txCtx, staffID, absenceID)
		return err
	})
	return
}

// DeleteOwnAbsence is the self-service delete path. Manager-created comp-time
// absences remain read-only for the affected staff member.
func (s *absenceCommands) DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.DeleteOwnAbsence(txCtx, staffID, actorAccountID, absenceID)
		return err
	})
	return
}

// DeleteAbsenceFor deletes subjectStaffID's absence on actorStaffID's behalf
// (#1843 admin delete). Deleting a sick report first reverses its plan
// cascade — reactivates the shifts it cancelled and clears the block
// absences it stamped — in the same tenant tx, so a failed reversal aborts
// the delete. CancelAbsence needs no such hook: it only accepts
// requested/approved vacation-flow rows, never a reported sick absence.
func (s *absenceCommands) DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.DeleteAbsenceFor(txCtx, subjectStaffID, actorStaffID, actorAccountID, absenceID)
		return err
	})
	return
}

// ListAbsences returns a staff member's absences for a date range, status, or
// both. The generic repository filter keeps tenant scoping in one place.
func (s *absenceCommands) ListAbsences(ctx context.Context, staffID int64, filter StaffAbsenceListFilter) (value0 []*StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.ListAbsences(txCtx, staffID, filter)
		return err
	})
	return
}

// GetAbsencesForRange preserves the range-only service contract used by staff
// detail views.
func (s *absenceCommands) GetAbsencesForRange(ctx context.Context, staffID int64, from, to calendar.Date) (value0 []*StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetAbsencesForRange(txCtx, staffID, from, to)
		return err
	})
	return
}

// HasAbsenceOnDate checks if a staff member has an absence on a specific date
func (s *absenceCommands) HasAbsenceOnDate(ctx context.Context, staffID int64, date calendar.Date) (value0 bool, value1 *StaffAbsence, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, value1, err = s.commands.HasAbsenceOnDate(txCtx, staffID, date)
		return err
	})
	return
}

func (s *absenceCommands) ListAbsenceRequests(ctx context.Context, req AbsenceRequestListQuery) (value0 []*StaffAbsenceRequestItem, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.ListAbsenceRequests(txCtx, req)
		return err
	})
	return
}

func (s *absenceCommands) ListPendingRequests(ctx context.Context) (value0 []*StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.ListPendingRequests(txCtx)
		return err
	})
	return
}

func (s *absenceCommands) GetVacationQuotaSummary(ctx context.Context, staffID int64, year int) (value0 *VacationQuotaSummary, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetVacationQuotaSummary(txCtx, staffID, year)
		return err
	})
	return
}

func (s *absenceCommands) UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.UpsertVacationQuota(txCtx, staffID, year, entitled, carryover)
		return err
	})
	return
}

func (s *absenceCommands) SetVacationQuota(ctx context.Context, change VacationQuotaChange) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.SetVacationQuota(txCtx, change)
		return err
	})
	return
}

// RebookAbsences changes the type of stored absences (#3258). The rows keep
// their dates, their entry date and their status; the Stundenkonto, the
// allowances and every view follow from the new type because all of them are
// computed on read. A closed month blocks the change, since its frozen
// closing balance would no longer match.
func (s *absenceCommands) RebookAbsences(ctx context.Context, req RebookAbsencesRequest) (value0 *AbsenceRebookingResult, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.RebookAbsences(txCtx, req)
		return err
	})
	return
}

// GetTodayAbsenceMap returns staff ID -> absence type for today.
func (s *absenceCommands) GetTodayAbsenceMap(ctx context.Context) (value0 map[int64]string, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetTodayAbsenceMap(txCtx)
		return err
	})
	return
}

// GetTodayAbsenceLabelMap returns staff ID -> school-defined absence name for
// today.
func (s *absenceCommands) GetTodayAbsenceLabelMap(ctx context.Context) (value0 map[int64]string, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetTodayAbsenceLabelMap(txCtx)
		return err
	})
	return
}

func (s *absenceCommands) RequestVacation(ctx context.Context, staffID int64, req RequestVacationRequest) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.RequestVacation(txCtx, staffID, req)
		return err
	})
	return
}

func (s *absenceCommands) ApproveAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, note string) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.ApproveAbsence(txCtx, absenceID, actorAccountID, decidedByStaffID, note)
		return err
	})
	return
}

func (s *absenceCommands) DenyAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, reason string) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.DenyAbsence(txCtx, absenceID, actorAccountID, decidedByStaffID, reason)
		return err
	})
	return
}

func (s *absenceCommands) QuestionAbsence(ctx context.Context, absenceID int64, actorAccountID int64, note string) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.QuestionAbsence(txCtx, absenceID, actorAccountID, note)
		return err
	})
	return
}

func (s *absenceCommands) ResubmitAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64, note string) (value0 *StaffAbsenceResponse, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.ResubmitAbsence(txCtx, staffID, actorAccountID, absenceID, note)
		return err
	})
	return
}

func (s *absenceCommands) CancelAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.CancelAbsence(txCtx, staffID, actorAccountID, absenceID)
		return err
	})
	return
}

// GetVacationOpening returns the takeover row for a staff/year, or nil.
func (s *absenceCommands) GetVacationOpening(ctx context.Context, staffID int64, year int) (value0 *StaffVacationOpening, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.GetVacationOpening(txCtx, staffID, year)
		return err
	})
	return
}

// SetVacationOpening books the vacation takeover for the Stichtag's year.
func (s *absenceCommands) SetVacationOpening(ctx context.Context, staffID, decidedBy int64, req SetVacationOpeningRequest) (value0 *StaffVacationOpening, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.SetVacationOpening(txCtx, staffID, decidedBy, req)
		return err
	})
	return
}

// ValidateVacationOpeningAbsencesBefore exposes the read-only half of the
// opening guard for import previews. It deliberately takes no lock: imports
// validate rows in file order before their sorted creation pass, and retaining
// per-staff locks here could deadlock concurrent previews or imports. The
// actual SetVacationOpening path still locks through insertion.
func (s *absenceCommands) ValidateVacationOpeningAbsencesBefore(ctx context.Context, staffID int64, effectiveDate calendar.Date) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.ValidateVacationOpeningAbsencesBefore(txCtx, staffID, effectiveDate)
		return err
	})
	return
}

// DeleteVacationOpening removes a takeover row and writes an append-only
// tombstone (audit.time_tracking_deletions) in the same tenant transaction.
func (s *absenceCommands) DeleteVacationOpening(ctx context.Context, staffID, deletedBy int64, year int) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.DeleteVacationOpening(txCtx, staffID, deletedBy, year)
		return err
	})
	return
}
