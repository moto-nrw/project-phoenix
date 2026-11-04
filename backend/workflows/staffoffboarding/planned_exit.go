package staffoffboarding

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The planned exit of #4809: the Leitung enters a last working day, and after
// the end of that day (Berlin) the scheduler runs the same offboarding as
// the immediate deletion, in the name of the person who planned it.

var (
	// ErrPlannedExitInPast rejects a last working day before today.
	ErrPlannedExitInPast = errors.New("the last working day must be today or later")
	// ErrPlannedExitStarted rejects changing or withdrawing an exit whose
	// last working day is over: the offboarding is due or already blocked.
	ErrPlannedExitStarted = errors.New("the planned exit is already due")
)

// Retry and reminder timing of the planned exit, fixed by #4809.
const (
	plannedExitRetryInterval = time.Hour
	plannedExitWeekNotice    = 7
)

// inReminderWeek tells whether today is in the seven days before the last
// working day.
func inReminderWeek(today, day calendar.Date) bool {
	return !today.Before(day.AddDays(-plannedExitWeekNotice))
}

// PlannedExits is School Membership's store of the planned exits, part of
// the workflow's Membership port.
type PlannedExits interface {
	FindPlannedExitForMutation(context.Context, int64) (schoolmembership.PlannedExit, error)
	ListPlannedExits(context.Context) ([]schoolmembership.PlannedExit, error)
	SavePlannedExit(context.Context, schoolmembership.PlanExit) (schoolmembership.PlannedExit, error)
	DeletePlannedExit(context.Context, int64) error
	RecordPlannedExitProgress(context.Context, schoolmembership.PlannedExitProgress) error
}

// Change actions of a planned exit in the audit trail.
const (
	ExitPlanned   = "planned"
	ExitMoved     = "moved"
	ExitWithdrawn = "withdrawn"
)

// ExitPlanChange is one planning step for the audit trail. OldDay and NewDay
// are calendar dates; either is empty when there is none.
type ExitPlanChange struct {
	StaffID int64
	Action  string
	OldDay  *calendar.Date
	NewDay  *calendar.Date
}

// PlannerDependencies are the ports of planning beyond the offboarding
// itself: the audit trail of a planning step and the clock that stamps the
// settled week reminder.
type PlannerDependencies struct {
	AppendAudit func(context.Context, Actor, ExitPlanChange) error
	Now         func() time.Time
}

// Planner plans, moves and withdraws exits; DueExits runs them.
type Planner struct {
	workflow *Workflow
	exits    PlannedExits
	deps     PlannerDependencies
}

func NewPlanner(workflow *Workflow, deps PlannerDependencies) (*Planner, error) {
	if workflow == nil || deps.AppendAudit == nil || deps.Now == nil {
		return nil, errors.New("staff offboarding planner: all dependencies are required")
	}
	return &Planner{workflow: workflow, exits: workflow.deps.Membership, deps: deps}, nil
}

// Plan enters or moves the last working day of a live staff member. Only an
// authorized school principal may plan, as for the immediate deletion.
func (p *Planner) Plan(ctx context.Context, staffID int64, day calendar.Date) (plan schoolmembership.PlannedExit, err error) {
	err = p.workflow.run(ctx, "plan_exit", staffID, func(txCtx context.Context, actor Actor) (Result, error) {
		today, err := p.today()
		if err != nil {
			return Result{}, err
		}
		if day.Before(today) {
			return Result{}, ErrPlannedExitInPast
		}
		plan, err = p.plan(txCtx, actor, staffID, day, today)
		return Result{}, err
	})
	if err != nil {
		return schoolmembership.PlannedExit{}, err
	}
	return plan, nil
}

// plan saves the day in the authorized unit and audits a change.
func (p *Planner) plan(ctx context.Context, actor Actor, staffID int64, day, today calendar.Date) (schoolmembership.PlannedExit, error) {
	if _, err := p.workflow.deps.FindStaff(ctx, staffID); err != nil {
		return schoolmembership.PlannedExit{}, err
	}
	previous, err := p.lockOpenPlan(ctx, staffID, today)
	if err != nil {
		return schoolmembership.PlannedExit{}, err
	}
	plan, err := p.exits.SavePlannedExit(ctx, schoolmembership.PlanExit{
		StaffID: staffID, LastWorkingDay: day,
		PlannedByAccountID: actor.AccountID, PlannedByStaffID: actor.StaffID, PlannedByName: actor.Username,
	})
	if err != nil || (previous != nil && *previous == plan.LastWorkingDay) {
		return plan, err
	}
	if err := p.settleWeekNotice(ctx, plan, today, day); err != nil {
		return schoolmembership.PlannedExit{}, err
	}
	change := ExitPlanChange{StaffID: staffID, Action: ExitPlanned, OldDay: previous, NewDay: &plan.LastWorkingDay}
	if previous != nil {
		change.Action = ExitMoved
	}
	return plan, p.deps.AppendAudit(ctx, actor, change)
}

// Withdraw takes a planned exit back before its last working day is over.
func (p *Planner) Withdraw(ctx context.Context, staffID int64) error {
	return p.workflow.run(ctx, "withdraw_exit", staffID, func(txCtx context.Context, actor Actor) (Result, error) {
		today, err := p.today()
		if err != nil {
			return Result{}, err
		}
		previous, err := p.lockOpenPlan(txCtx, staffID, today)
		if err != nil {
			return Result{}, err
		}
		if previous == nil {
			return Result{}, schoolmembership.ErrPlannedExitNotFound
		}
		if err := p.exits.DeletePlannedExit(txCtx, staffID); err != nil {
			return Result{}, err
		}
		return Result{}, p.deps.AppendAudit(txCtx, actor, ExitPlanChange{StaffID: staffID, Action: ExitWithdrawn, OldDay: previous})
	})
}

// lockOpenPlan locks the staff member's planned exit and returns its day, or
// nil without one. An exit whose day is over can no longer change.
func (p *Planner) lockOpenPlan(ctx context.Context, staffID int64, today calendar.Date) (*calendar.Date, error) {
	existing, err := p.exits.FindPlannedExitForMutation(ctx, staffID)
	if errors.Is(err, schoolmembership.ErrPlannedExitNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if existing.LastWorkingDay.Before(today) {
		return nil, ErrPlannedExitStarted
	}
	return &existing.LastWorkingDay, nil
}

func (p *Planner) today() (calendar.Date, error) {
	return calendar.ParseDate(p.workflow.deps.Today())
}

// settleWeekNotice marks the week-before reminder as done for an exit
// planned inside that week: the planner just entered it.
func (p *Planner) settleWeekNotice(ctx context.Context, plan schoolmembership.PlannedExit, today, day calendar.Date) error {
	if !inReminderWeek(today, day) {
		return nil
	}
	now := p.deps.Now()
	return p.exits.RecordPlannedExitProgress(ctx, schoolmembership.PlannedExitProgress{StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay, WeekNoticeSentAt: &now})
}

// closeAccess runs only the Identity & Access step of the offboarding: the
// member's roles at this school and their sessions are revoked, so neither a
// new sign-in nor a session renewal reaches the school, while the blocked
// rest of the offboarding waits. A later execution finds the access already
// closed and completes the remaining owners.
func (w *Workflow) closeAccess(ctx context.Context, staffID int64, authorize func(context.Context) (Actor, error)) error {
	return w.runAs(ctx, "close_access", staffID, authorize, func(txCtx context.Context, _ Actor) (Result, error) {
		staff, err := w.deps.FindStaff(txCtx, staffID)
		if err != nil {
			return Result{}, err
		}
		person, err := w.deps.People.FindPersonForMutation(txCtx, staff.PersonID)
		if err != nil || person.AccountID == nil {
			return Result{}, err
		}
		preview, err := w.deps.PreviewAccess(txCtx, *person.AccountID)
		if err != nil {
			return Result{}, err
		}
		_, err = w.deps.ExecuteAccess(txCtx, *person.AccountID, preview.Revision)
		return Result{}, err
	})
}
