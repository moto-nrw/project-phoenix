package careexit

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RestoredPlanResync re-plans the offering-sourced roster rows of children
// whose removed plan the care lifecycle put back. Care Plan decides the rows'
// planning period; this workflow only decides when it runs. An empty list
// re-plans nothing.
type RestoredPlanResync interface {
	ResyncStudentSourcedRosters(ctx context.Context, studentIDs []int64, effectiveFrom calendar.Date) error
}

// RestoringCareExits are the Care Plan lifecycle commands that put a removed
// plan back before they return.
type RestoringCareExits interface {
	Confirm(ctx context.Context, token string, input careplan.CareExitInput, actorAccountID int64) (*careplan.CareExitResult, error)
	ConfirmWithdrawalCareEnd(ctx context.Context, completionID int64, token string, input careplan.CareExitInput, actorAccountID int64) (*careplan.CareExitResult, error)
	Cancel(ctx context.Context, studentIDs []int64, actorAccountID int64) (int, error)
}

// PlanRestoration runs the restoring care-exit commands and re-plans the
// restored children against the current source configuration in the same
// unit of work (#5235). Cancelling a planned exit, and confirming a changed
// one, put the removed rows back verbatim, with the planning period recorded
// when they were removed. A template whose schedules have meanwhile moved to
// another period would leave the child off its new occurrences until some
// unrelated resync. The re-plan runs after the complete restore, including
// the care boundary, and from today on: history stays as it was.
type PlanRestoration struct {
	exits   RestoringCareExits
	rosters RestoredPlanResync
	unit    func(context.Context, func(context.Context) error) error
	today   func() calendar.Date
}

// NewPlanRestoration binds the lifecycle commands, Care Plan's roster resync and the
// outer unit the lifecycle's own transaction joins.
func NewPlanRestoration(exits RestoringCareExits, rosters RestoredPlanResync,
	unit func(context.Context, func(context.Context) error) error, today func() calendar.Date,
) *PlanRestoration {
	if exits == nil || rosters == nil || unit == nil {
		panic("care plan restoration: care exits, roster resync and unit of work are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &PlanRestoration{exits: exits, rosters: rosters, unit: unit, today: today}
}

// Cancel withdraws the planned exits and re-plans every cancelled child.
func (p *PlanRestoration) Cancel(ctx context.Context, studentIDs []int64, actorAccountID int64) (cancelled int, err error) {
	err = p.unit(ctx, func(txCtx context.Context) error {
		cancelled, err = p.exits.Cancel(txCtx, studentIDs, actorAccountID)
		if err != nil {
			return err
		}
		return p.rosters.ResyncStudentSourcedRosters(txCtx, uniqueSortedIDs(studentIDs), p.today())
	})
	if err != nil {
		return 0, err
	}
	return cancelled, nil
}

// Confirm ends the care and re-plans the children whose earlier exit it
// replaced before applying the new last care day.
func (p *PlanRestoration) Confirm(ctx context.Context, token string, input careplan.CareExitInput, actorAccountID int64) (result *careplan.CareExitResult, err error) {
	err = p.unit(ctx, func(txCtx context.Context) error {
		result, err = p.exits.Confirm(txCtx, token, input, actorAccountID)
		if err != nil {
			return err
		}
		return p.rosters.ResyncStudentSourcedRosters(txCtx, result.ReappliedStudentIDs, p.today())
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ConfirmWithdrawalCareEnd is Confirm for the guided withdrawal close-out.
func (p *PlanRestoration) ConfirmWithdrawalCareEnd(
	ctx context.Context, completionID int64, token string, input careplan.CareExitInput, actorAccountID int64,
) (result *careplan.CareExitResult, err error) {
	err = p.unit(ctx, func(txCtx context.Context) error {
		result, err = p.exits.ConfirmWithdrawalCareEnd(txCtx, completionID, token, input, actorAccountID)
		if err != nil {
			return err
		}
		return p.rosters.ResyncStudentSourcedRosters(txCtx, result.ReappliedStudentIDs, p.today())
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
