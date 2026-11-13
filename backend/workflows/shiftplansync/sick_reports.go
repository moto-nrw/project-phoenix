package shiftplansync

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// SickReports is the entry of the absence writes that may cascade an absence
// into the plans (#1843, #4825); since #6847 every absence type in force
// cascades, a sick report as well as an approved vacation. Each write runs Workforce's absence command and the
// plan it returns in one tenant transaction, and a failed plan step aborts
// the absence write, so a half-cascaded sick report never commits. Before
// the write the entry takes every lock the write and its cascade need, in
// the canonical order (SickCascade.LockSickReportWrite, #6232): the absence
// gate, the staff-shift gates, the days; Workforce's staff row comes after.
type SickReports struct {
	absences workforce.SickReportWrites
	cascade  SickCascade
	runInTx  func(context.Context, func(context.Context) error) error
}

// NewSickReports binds Workforce's sick-report writes to the cascade, run in
// the unit of work runInTx: the caller's tenant transaction, or one for the
// context's school (#4825).
func NewSickReports(absences workforce.SickReportWrites, cascade SickCascade, runInTx func(context.Context, func(context.Context) error) error) (*SickReports, error) {
	if absences == nil || cascade == nil || runInTx == nil {
		return nil, errors.New("shift plan sync: sick reports need the absence writes, the cascade and the unit of work")
	}
	return &SickReports{absences: absences, cascade: cascade, runInTx: runInTx}, nil
}

// ApproveAbsence approves a request; the approved absence cascades into the
// plans (#6847). The request names no subject, so the lock reads it from the
// stored absence.
func (r *SickReports) ApproveAbsence(ctx context.Context, absenceID, actorAccountID, decidedByStaffID int64, note string) (*workforce.StaffAbsenceResponse, error) {
	return r.absence(ctx, r.lockFor(0, absenceID, "", ""), func(ctx context.Context) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
		return r.absences.ApproveAbsence(ctx, absenceID, actorAccountID, decidedByStaffID, note)
	})
}

// CancelAbsence withdraws the staff member's request; withdrawing an approved
// one reverses its plan cascade (#6847).
func (r *SickReports) CancelAbsence(ctx context.Context, staffID, actorAccountID, absenceID int64) error {
	return r.deletion(ctx, r.lockFor(staffID, absenceID, "", ""), func(ctx context.Context) (workforce.SickCascadePlan, error) {
		return r.absences.CancelAbsence(ctx, staffID, actorAccountID, absenceID)
	})
}

// CreateOwnAbsence files the staff member's own absence.
func (r *SickReports) CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	return r.absence(ctx, r.lockFor(staffID, 0, request.DateStart, request.DateEnd), func(ctx context.Context) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
		return r.absences.CreateOwnAbsence(ctx, staffID, actorAccountID, request)
	})
}

// CreateAbsenceFor files an absence for subjectStaffID on createdByStaffID's
// behalf; the merge path cascades over the merged range of the primary row,
// which the cascade's idempotency guards make safe to re-run.
func (r *SickReports) CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	return r.absence(ctx, r.lockFor(subjectStaffID, 0, request.DateStart, request.DateEnd), func(ctx context.Context) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
		return r.absences.CreateAbsenceFor(ctx, subjectStaffID, createdByStaffID, actorAccountID, request)
	})
}

// UpdateAbsence edits the staff member's absence; a sick report's range edit
// reconciles its plan-day difference.
func (r *SickReports) UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, request workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	return r.absence(ctx, r.lockFor(staffID, absenceID, optionalDate(request.DateStart), optionalDate(request.DateEnd)), func(ctx context.Context) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
		return r.absences.UpdateAbsence(ctx, staffID, actorAccountID, absenceID, request)
	})
}

// DeleteOwnAbsence deletes the staff member's own absence and reverses its
// plan cascade.
func (r *SickReports) DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) error {
	return r.deletion(ctx, r.lockFor(staffID, absenceID, "", ""), func(ctx context.Context) (workforce.SickCascadePlan, error) {
		return r.absences.DeleteOwnAbsence(ctx, staffID, actorAccountID, absenceID)
	})
}

// DeleteAbsenceFor deletes subjectStaffID's absence on actorStaffID's behalf
// and reverses its plan cascade.
func (r *SickReports) DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) error {
	return r.deletion(ctx, r.lockFor(subjectStaffID, absenceID, "", ""), func(ctx context.Context) (workforce.SickCascadePlan, error) {
		return r.absences.DeleteAbsenceFor(ctx, subjectStaffID, actorStaffID, actorAccountID, absenceID)
	})
}

// lockFor takes the write's locks before Workforce writes (#6232).
func (r *SickReports) lockFor(staffID, absenceID int64, dateStart, dateEnd string) func(context.Context) error {
	return func(ctx context.Context) error {
		return r.cascade.LockSickReportWrite(ctx, staffID, absenceID, dateStart, dateEnd)
	}
}

func optionalDate(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *SickReports) absence(ctx context.Context, lock func(context.Context) error, write func(context.Context) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error)) (*workforce.StaffAbsenceResponse, error) {
	var result *workforce.StaffAbsenceResponse
	err := r.runInTx(ctx, func(ctx context.Context) error {
		if err := lock(ctx); err != nil {
			return err
		}
		value, plan, err := write(ctx)
		if err != nil {
			return err
		}
		if err := r.apply(ctx, plan); err != nil {
			return err
		}
		result = value
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *SickReports) deletion(ctx context.Context, lock func(context.Context) error, write func(context.Context) (workforce.SickCascadePlan, error)) error {
	return r.runInTx(ctx, func(ctx context.Context) error {
		if err := lock(ctx); err != nil {
			return err
		}
		plan, err := write(ctx)
		if err != nil {
			return err
		}
		return r.apply(ctx, plan)
	})
}

// apply runs the plan's steps in order; the first failure aborts the write.
func (r *SickReports) apply(ctx context.Context, plan workforce.SickCascadePlan) error {
	for _, step := range plan {
		if err := r.applyStep(ctx, step); err != nil {
			return err
		}
	}
	return nil
}

func (r *SickReports) applyStep(ctx context.Context, step workforce.SickCascadeStep) error {
	switch step.Kind {
	case workforce.SickCascadeMark:
		if err := r.cascade.MarkSickForRange(ctx, step.Report); err != nil {
			return fmt.Errorf("absence saved nothing — plan cascade failed: %w", err)
		}
	case workforce.SickCascadeClear:
		if err := r.cascade.ClearSickForRange(ctx, step.Report); err != nil {
			return fmt.Errorf("absence not deleted — plan cascade reversal failed: %w", err)
		}
	case workforce.SickCascadeReconcile:
		if err := r.cascade.ReconcileSickRange(ctx, step.Before, step.Report); err != nil {
			return fmt.Errorf("absence update saved nothing — plan cascade reconciliation failed: %w", err)
		}
	case workforce.SickCascadeReassign:
		if err := r.cascade.ReassignSickStamps(ctx, step.FromAbsenceID, step.ToAbsenceID); err != nil {
			return fmt.Errorf("failed to merge absence — plan stamp reassignment failed: %w", err)
		}
	default:
		return fmt.Errorf("sick cascade: unknown plan step %q", step.Kind)
	}
	return nil
}
