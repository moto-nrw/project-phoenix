// ShiftPlanSyncer is the #1843 bridge between a sick report
// (active.staff_absences) and the planning layer (schedule.staff_shifts +
// schedule.instance_staff). The absence lifecycle asks it for the plan
// changes of a write; in production a sickCascadePlanner records them into
// the workforce.SickCascadePlan the write returns, and the shift-plan-sync
// workflow applies that plan in the same tenant transaction (#4825). Unlike
// the AttendanceSyncer the contract is FAIL-CLOSED: the linkage is the
// feature, so an error must abort the surrounding absence write.
package timetracking

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// SickCascadeInput identifies one sick report and the range it covers. Both
// cascade directions take the same shape; the dates are calendar days in
// workforce.DateLayout.
type SickCascadeInput = workforce.SickCascadeInput

// ShiftPlanSyncer receives the plan changes of one absence write, in the
// order the write asks for them.
type ShiftPlanSyncer interface {
	MarkSickForRange(ctx context.Context, in SickCascadeInput) error
	ClearSickForRange(ctx context.Context, in SickCascadeInput) error
	ReconcileSickRange(ctx context.Context, before, after SickCascadeInput) error
	ReassignSickStamps(ctx context.Context, fromAbsenceID, toAbsenceID int64) error
}

// errUnplannedSickCascade refuses a plan change outside a planned absence
// write: no sick report may commit without its plan effects.
var errUnplannedSickCascade = errors.New("staff absence: sick cascade requested outside a planned absence write")

// unplannedSickCascade is the syncer of a service whose writes nobody plans.
type unplannedSickCascade struct{}

func (unplannedSickCascade) MarkSickForRange(context.Context, SickCascadeInput) error {
	return errUnplannedSickCascade
}

func (unplannedSickCascade) ClearSickForRange(context.Context, SickCascadeInput) error {
	return errUnplannedSickCascade
}

func (unplannedSickCascade) ReconcileSickRange(context.Context, SickCascadeInput, SickCascadeInput) error {
	return errUnplannedSickCascade
}

func (unplannedSickCascade) ReassignSickStamps(context.Context, int64, int64) error {
	return errUnplannedSickCascade
}

// sickCascadePlanner records the plan changes of one absence write.
type sickCascadePlanner struct {
	plan *workforce.SickCascadePlan
}

func (p sickCascadePlanner) MarkSickForRange(_ context.Context, in SickCascadeInput) error {
	*p.plan = append(*p.plan, workforce.SickCascadeStep{Kind: workforce.SickCascadeMark, Report: in})
	return nil
}

func (p sickCascadePlanner) ClearSickForRange(_ context.Context, in SickCascadeInput) error {
	*p.plan = append(*p.plan, workforce.SickCascadeStep{Kind: workforce.SickCascadeClear, Report: in})
	return nil
}

func (p sickCascadePlanner) ReconcileSickRange(_ context.Context, before, after SickCascadeInput) error {
	*p.plan = append(*p.plan, workforce.SickCascadeStep{Kind: workforce.SickCascadeReconcile, Before: before, Report: after})
	return nil
}

func (p sickCascadePlanner) ReassignSickStamps(_ context.Context, fromAbsenceID, toAbsenceID int64) error {
	*p.plan = append(*p.plan, workforce.SickCascadeStep{Kind: workforce.SickCascadeReassign, FromAbsenceID: fromAbsenceID, ToAbsenceID: toAbsenceID})
	return nil
}

// sickCascadePlanning is a service that can run one write with a planner.
type sickCascadePlanning interface {
	withSickCascadePlan() (StaffAbsenceService, *workforce.SickCascadePlan)
}

// withSickCascadePlan is this service for one write whose plan changes are
// recorded into the returned plan. The copy is shallow on purpose: the
// service holds only its collaborators and settings, no per-write state, so
// the copy differs from s in the syncer alone.
func (s *staffAbsenceService) withSickCascadePlan() (StaffAbsenceService, *workforce.SickCascadePlan) {
	plan := &workforce.SickCascadePlan{}
	planned := *s
	planned.shiftPlanSyncer = sickCascadePlanner{plan: plan}
	return &planned, plan
}

// withSickCascadePlan keeps the command boundary around the planned write.
func (c *absenceCommands) withSickCascadePlan() (StaffAbsenceService, *workforce.SickCascadePlan) {
	planned, plan := c.commands.withSickCascadePlan()
	service := planned.(*staffAbsenceService)
	return &absenceCommands{StaffAbsenceService: service, commands: service}, plan
}

// planSickCascade returns absences for one write and the plan it records.
func planSickCascade(absences StaffAbsenceService) (StaffAbsenceService, *workforce.SickCascadePlan, error) {
	planning, ok := absences.(sickCascadePlanning)
	if !ok {
		return nil, nil, errors.New("staff absence: the absence service cannot plan the sick cascade")
	}
	planned, plan := planning.withSickCascadePlan()
	return planned, plan, nil
}
