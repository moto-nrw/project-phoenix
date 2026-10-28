package staffoffboarding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Notice kinds of a planned exit.
const (
	NoticeWeekBefore = "week_before"
	NoticeExitDay    = "exit_day"
	NoticeBlocked    = "blocked"
)

// ExitNotice tells the Leitung about a planned exit. It lands on a lock
// screen, so the text names no person; DeepLink opens the profile.
type ExitNotice struct {
	Kind           string
	TenantID       int64
	StaffID        int64
	LastWorkingDay calendar.Date
	Title          string
	Body           string
	DeepLink       string
	// IdempotencyKey is the same for every attempt at one notice.
	IdempotencyKey string
}

// DueExitDependencies are the ports of the scheduler's run. Notify reports
// whether the notice went out; a notice the school's delivery settings hold
// back now (paused, outside the active hours) is sent on a later run.
// Isolate runs one planned exit in a savepoint of the caller's transaction:
// failed is that exit's own error, rolled back; err means the transaction
// itself can no longer be trusted. Now is the scheduler's clock.
type DueExitDependencies struct {
	Notify  func(context.Context, ExitNotice) (bool, error)
	Isolate func(context.Context, func(context.Context) error) (failed error, err error)
	Now     func() time.Time
}

// DueExits sends the reminders of the planned exits and runs the due ones.
type DueExits struct {
	planner *Planner
	deps    DueExitDependencies
}

func NewDueExits(planner *Planner, deps DueExitDependencies) (*DueExits, error) {
	if planner == nil || deps.Notify == nil || deps.Isolate == nil || deps.Now == nil {
		return nil, errors.New("staff offboarding due exits: all dependencies are required")
	}
	return &DueExits{planner: planner, deps: deps}, nil
}

// DueRun reports the planned exits whose run failed, by staff ID. Each
// failure was rolled back on its own; the rest of the run commits.
type DueRun struct {
	Failed map[int64]error
}

// RunDue sends the reminders and runs the due exits of the school in ctx,
// inside the caller's transaction. Every planned exit runs in a savepoint, so
// one failing exit neither blocks the others nor undoes their notices.
func (d *DueExits) RunDue(ctx context.Context) (DueRun, error) {
	run := DueRun{Failed: map[int64]error{}}
	today, err := d.planner.today()
	if err != nil {
		return run, err
	}
	plans, err := d.planner.exits.ListPlannedExits(ctx)
	if err != nil {
		return run, err
	}
	for _, plan := range plans {
		var exitErr error
		failed, err := d.deps.Isolate(ctx, func(spCtx context.Context) (err error) {
			exitErr, err = d.runPlan(spCtx, plan, today)
			return err
		})
		if err != nil {
			return run, fmt.Errorf("planned exit of staff %d: %w", plan.StaffID, err)
		}
		if failed = errors.Join(failed, exitErr); failed != nil {
			run.Failed[plan.StaffID] = failed
		}
	}
	return run, nil
}

// runPlan does what is due for one planned exit. exitErr is a failed
// offboarding whose closed sign-in and next attempt are kept; err rolls the
// planned exit's savepoint back.
func (d *DueExits) runPlan(ctx context.Context, plan schoolmembership.PlannedExit, today calendar.Date) (exitErr, err error) {
	day := plan.LastWorkingDay
	switch {
	case day.After(today):
		if !inReminderWeek(today, day) {
			return nil, nil
		}
		return nil, d.remindOnce(ctx, plan, NoticeWeekBefore, plan.WeekNoticeSentAt, func(progress *schoolmembership.PlannedExitProgress, at *time.Time) {
			progress.WeekNoticeSentAt = at
		})
	case day == today:
		return nil, d.remindOnce(ctx, plan, NoticeExitDay, plan.DayNoticeSentAt, func(progress *schoolmembership.PlannedExitProgress, at *time.Time) {
			progress.DayNoticeSentAt = at
		})
	default:
		return d.runExit(ctx, plan, today)
	}
}

func (d *DueExits) remindOnce(ctx context.Context, plan schoolmembership.PlannedExit, kind string, sentAt *time.Time, mark func(*schoolmembership.PlannedExitProgress, *time.Time)) error {
	if sentAt != nil {
		return nil
	}
	sent, err := d.deps.Notify(ctx, exitNotice(kind, plan))
	if err != nil || !sent {
		return err
	}
	now := d.deps.Now()
	progress := schoolmembership.PlannedExitProgress{StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay, NextAttemptAt: plan.NextAttemptAt}
	mark(&progress, &now)
	return d.planner.exits.RecordPlannedExitProgress(ctx, progress)
}

// runExit offboards the member through the immediate deletion's execution
// path, acting as the planner, once the planned exit is due. The plan is
// locked and read again first: a move or withdrawal that committed after
// the listing wins. A failed offboarding closes the member's sign-in at
// this school anyway and is retried every hour until it succeeds; a blocked
// one also tells the Leitung once.
func (d *DueExits) runExit(ctx context.Context, listed schoolmembership.PlannedExit, today calendar.Date) (exitErr, err error) {
	plan, err := d.planner.exits.FindPlannedExitForMutation(ctx, listed.StaffID)
	if errors.Is(err, schoolmembership.ErrPlannedExitNotFound) || errors.Is(err, schoolmembership.ErrStaffNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := d.deps.Now()
	if !plan.LastWorkingDay.Before(today) ||
		(plan.NextAttemptAt != nil && now.Before(*plan.NextAttemptAt)) {
		return nil, err
	}
	asPlanner := plannerActor(plan)
	execErr, err := d.deps.Isolate(ctx, func(spCtx context.Context) error {
		_, err := d.planner.workflow.executeAs(spCtx, "planned_exit", plan.StaffID, "", asPlanner)
		return err
	})
	if err != nil || execErr == nil {
		return nil, err
	}
	if err := d.recordFailedExit(ctx, plan, now, errors.Is(execErr, ErrInUse), asPlanner); err != nil {
		return nil, err
	}
	if errors.Is(execErr, ErrInUse) {
		return nil, nil
	}
	return execErr, nil
}

// recordFailedExit closes the sign-in once, schedules the next attempt and
// tells the Leitung once about a blocked exit.
func (d *DueExits) recordFailedExit(ctx context.Context, plan schoolmembership.PlannedExit, now time.Time, blocked bool, asPlanner func(context.Context) (Actor, error)) error {
	next := now.Add(plannedExitRetryInterval)
	progress := schoolmembership.PlannedExitProgress{StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay, NextAttemptAt: &next}
	if plan.AccessLockedAt == nil {
		if err := d.planner.workflow.closeAccess(ctx, plan.StaffID, asPlanner); err != nil {
			return err
		}
		progress.AccessLockedAt = &now
	}
	if blocked && plan.BlockedNoticeSentAt == nil {
		sent, err := d.deps.Notify(ctx, exitNotice(NoticeBlocked, plan))
		if err != nil {
			return err
		}
		if sent {
			progress.BlockedNoticeSentAt = &now
		}
	}
	return d.planner.exits.RecordPlannedExitProgress(ctx, progress)
}

// plannerActor is the person who planned the exit: the scheduled offboarding
// runs in their name.
func plannerActor(plan schoolmembership.PlannedExit) func(context.Context) (Actor, error) {
	actor := Actor{
		TenantID: plan.TenantID, AccountID: plan.PlannedByAccountID, StaffID: plan.PlannedByStaffID,
		Username: plan.PlannedByName, PlannedExitDay: &plan.LastWorkingDay,
	}
	return func(context.Context) (Actor, error) { return actor, nil }
}

// exitNotice is the notice text of #4809 in moto's plain German.
func exitNotice(kind string, plan schoolmembership.PlannedExit) ExitNotice {
	notice := ExitNotice{
		Kind: kind, TenantID: plan.TenantID, StaffID: plan.StaffID, LastWorkingDay: plan.LastWorkingDay,
		DeepLink:       fmt.Sprintf("/staff/%d", plan.StaffID),
		IdempotencyKey: fmt.Sprintf("staff-exit-%d-%s-%s", plan.StaffID, plan.LastWorkingDay, kind),
	}
	day := plan.LastWorkingDay.Format("02.01.2006")
	switch kind {
	case NoticeWeekBefore:
		notice.Title = "Austritt in einer Woche"
		notice.Body = "Am " + day + " hat eine Person aus dem Team ihren letzten Arbeitstag."
	case NoticeExitDay:
		notice.Title = "Heute letzter Arbeitstag"
		notice.Body = "Heute hat eine Person aus dem Team ihren letzten Arbeitstag. Heute Nacht endet ihr Zugang."
	default:
		notice.Title = "Austritt wartet noch"
		notice.Body = "Die Person führt noch eine Aufsicht oder hat eine Gruppe übernommen. Ihre Anmeldung ist gesperrt."
	}
	return notice
}
