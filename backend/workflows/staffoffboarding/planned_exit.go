package staffoffboarding

import (
	"context"
	"errors"
	"fmt"
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
	PlannedExitRetryInterval = time.Hour
	plannedExitWeekNotice    = 7
)

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
	OldDay  string
	NewDay  string
}

// Notice kinds of a planned exit.
const (
	NoticeWeekBefore = "week_before"
	NoticeExitDay    = "exit_day"
	NoticeBlocked    = "blocked"
)

// ExitNotice tells the Leitung about a planned exit. It names the staff
// member by ID only; the recipients open the profile for the name.
type ExitNotice struct {
	Kind           string
	TenantID       int64
	StaffID        int64
	LastWorkingDay string
}

// PlannerDependencies are the ports of the planned exit beyond the
// offboarding itself. Notify reports whether the notice went out; a notice
// the school's delivery settings hold back now (paused, outside the active
// hours) is sent on a later run. Only RunDue notifies, so a planner that
// serves requests alone may leave Notify nil. Now is the scheduler's clock.
type PlannerDependencies struct {
	AppendAudit func(context.Context, Actor, ExitPlanChange) error
	Notify      func(context.Context, ExitNotice) (bool, error)
	Now         func() time.Time
}

// Planner plans, moves and withdraws exits and runs the due ones.
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
func (p *Planner) Plan(ctx context.Context, staffID int64, lastWorkingDay string) (plan schoolmembership.PlannedExit, err error) {
	day, err := calendar.ParseDate(lastWorkingDay)
	if err != nil {
		return schoolmembership.PlannedExit{}, &schoolmembership.InvalidMembershipError{Reason: "last working day must be a date (YYYY-MM-DD)"}
	}
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
		StaffID: staffID, LastWorkingDay: day.String(),
		PlannedByAccountID: actor.AccountID, PlannedByStaffID: actor.StaffID, PlannedByName: actor.Username,
	})
	if err != nil || previous == plan.LastWorkingDay {
		return plan, err
	}
	if err := p.settleWeekNotice(ctx, plan, today, day); err != nil {
		return schoolmembership.PlannedExit{}, err
	}
	change := ExitPlanChange{StaffID: staffID, Action: ExitPlanned, OldDay: previous, NewDay: plan.LastWorkingDay}
	if previous != "" {
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
		if previous == "" {
			return Result{}, schoolmembership.ErrPlannedExitNotFound
		}
		if err := p.exits.DeletePlannedExit(txCtx, staffID); err != nil {
			return Result{}, err
		}
		return Result{}, p.deps.AppendAudit(txCtx, actor, ExitPlanChange{StaffID: staffID, Action: ExitWithdrawn, OldDay: previous})
	})
}

// lockOpenPlan locks the staff member's planned exit and returns its day, or
// "" without one. An exit whose day is over can no longer change.
func (p *Planner) lockOpenPlan(ctx context.Context, staffID int64, today calendar.Date) (string, error) {
	existing, err := p.exits.FindPlannedExitForMutation(ctx, staffID)
	if errors.Is(err, schoolmembership.ErrPlannedExitNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	day, err := calendar.ParseDate(existing.LastWorkingDay)
	if err != nil {
		return "", err
	}
	if day.Before(today) {
		return "", ErrPlannedExitStarted
	}
	return existing.LastWorkingDay, nil
}

func (p *Planner) today() (calendar.Date, error) {
	return calendar.ParseDate(p.workflow.deps.Today())
}

// RunDue sends the reminders and runs the due exits of the school in ctx.
// The caller holds the school's transaction; a failure other than a blocked
// offboarding rolls the whole run back for the next attempt.
func (p *Planner) RunDue(ctx context.Context) error {
	if p.deps.Notify == nil {
		return errors.New("staff offboarding planner: the due run needs a notifier")
	}
	today, err := p.today()
	if err != nil {
		return err
	}
	plans, err := p.exits.ListPlannedExits(ctx)
	if err != nil {
		return err
	}
	now := p.deps.Now()
	for _, plan := range plans {
		day, err := calendar.ParseDate(plan.LastWorkingDay)
		if err != nil {
			return err
		}
		switch {
		case day.After(today):
			err = p.remindWeekBefore(ctx, plan, today, day)
		case day == today:
			err = p.remindOnce(ctx, plan, NoticeExitDay, plan.DayNoticeSentAt, func(progress *schoolmembership.PlannedExitProgress, at *time.Time) {
				progress.DayNoticeSentAt = at
			})
		case plan.NextAttemptAt == nil || !now.Before(*plan.NextAttemptAt):
			err = p.runExit(ctx, plan, now)
		}
		if err != nil {
			return fmt.Errorf("planned exit of staff %d: %w", plan.StaffID, err)
		}
	}
	return nil
}

// settleWeekNotice marks the week-before reminder as done for an exit
// planned inside that week: the planner just entered it.
func (p *Planner) settleWeekNotice(ctx context.Context, plan schoolmembership.PlannedExit, today, day calendar.Date) error {
	if today.Before(day.AddDays(-plannedExitWeekNotice)) {
		return nil
	}
	now := p.deps.Now()
	return p.exits.RecordPlannedExitProgress(ctx, schoolmembership.PlannedExitProgress{StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay, WeekNoticeSentAt: &now})
}

// remindWeekBefore sends the reminder from seven days before the last
// working day on.
func (p *Planner) remindWeekBefore(ctx context.Context, plan schoolmembership.PlannedExit, today, day calendar.Date) error {
	if today.Before(day.AddDays(-plannedExitWeekNotice)) {
		return nil
	}
	return p.remindOnce(ctx, plan, NoticeWeekBefore, plan.WeekNoticeSentAt, func(progress *schoolmembership.PlannedExitProgress, at *time.Time) {
		progress.WeekNoticeSentAt = at
	})
}

func (p *Planner) remindOnce(ctx context.Context, plan schoolmembership.PlannedExit, kind string, sentAt *time.Time, mark func(*schoolmembership.PlannedExitProgress, *time.Time)) error {
	if sentAt != nil {
		return nil
	}
	sent, err := p.deps.Notify(ctx, ExitNotice{Kind: kind, TenantID: plan.TenantID, StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay})
	if err != nil || !sent {
		return err
	}
	now := p.deps.Now()
	progress := schoolmembership.PlannedExitProgress{StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay, NextAttemptAt: plan.NextAttemptAt}
	mark(&progress, &now)
	return p.exits.RecordPlannedExitProgress(ctx, progress)
}

// runExit offboards the member through the immediate deletion's execution
// path, acting as the planner. A blocked offboarding closes the member's
// sign-in at this school anyway, tells the Leitung once and is retried every
// hour until it succeeds.
func (p *Planner) runExit(ctx context.Context, plan schoolmembership.PlannedExit, now time.Time) error {
	actor := Actor{
		TenantID: plan.TenantID, AccountID: plan.PlannedByAccountID, StaffID: plan.PlannedByStaffID,
		Username: plan.PlannedByName, PlannedExitDay: plan.LastWorkingDay,
	}
	asPlanner := func(context.Context) (Actor, error) { return actor, nil }
	_, err := p.workflow.executeAs(ctx, "planned_exit", plan.StaffID, "", asPlanner)
	if !errors.Is(err, ErrInUse) {
		return err
	}
	next := now.Add(PlannedExitRetryInterval)
	progress := schoolmembership.PlannedExitProgress{StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay, NextAttemptAt: &next}
	if plan.AccessLockedAt == nil {
		if err := p.workflow.closeAccess(ctx, plan.StaffID, asPlanner); err != nil {
			return err
		}
		progress.AccessLockedAt = &now
	}
	if plan.BlockedNoticeSentAt == nil {
		sent, err := p.deps.Notify(ctx, ExitNotice{Kind: NoticeBlocked, TenantID: plan.TenantID, StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay})
		if err != nil {
			return err
		}
		if sent {
			progress.BlockedNoticeSentAt = &now
		}
	}
	return p.exits.RecordPlannedExitProgress(ctx, progress)
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
