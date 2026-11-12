package staffoffboarding

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// ErrExitLocked reports a planned exit whose locks another transaction held
// longer than the run waits for them. The exit gives way and runs on a later
// run, so it holds up neither the holder nor the school's other exits.
var ErrExitLocked = errors.New("staff offboarding: planned exit is locked by another transaction")

// DueExitDependencies are the ports of the scheduler's run. Notify reports
// whether the notice went out; a notice the school's delivery settings hold
// back now (paused, outside the active hours) is sent on a later run.
//
// Within runs one planned exit in a transaction of its own of the school in
// ctx, detached from the caller's: failed is that exit's own error, rolled
// back; err means the run can no longer trust the database (no transaction
// started, unknown commit outcome, failed savepoint control, cancelled run)
// and stops. BoundLockWaits runs fn so that a wait for a lock another
// transaction holds ends after a bound with an error wrapping ErrExitLocked.
// It covers the whole exit, including its execution and progress writes.
// Isolate runs one offboarding in a savepoint of the exit's transaction,
// with failed and err as in Within. Now is the scheduler's clock; Logger
// takes one result line per planned exit (slog.Default when nil).
type DueExitDependencies struct {
	Notify         func(context.Context, ExitNotice) (bool, error)
	Within         func(context.Context, func(context.Context) error) (failed error, err error)
	BoundLockWaits func(context.Context, func(context.Context) error) error
	Isolate        func(context.Context, func(context.Context) error) (failed error, err error)
	Now            func() time.Time
	Logger         *slog.Logger
}

// DueExits sends the reminders of the planned exits and runs the due ones.
type DueExits struct {
	planner *Planner
	deps    DueExitDependencies
}

func NewDueExits(planner *Planner, deps DueExitDependencies) (*DueExits, error) {
	if planner == nil || deps.Notify == nil || deps.Within == nil || deps.BoundLockWaits == nil || deps.Isolate == nil || deps.Now == nil {
		return nil, errors.New("staff offboarding due exits: all dependencies are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &DueExits{planner: planner, deps: deps}, nil
}

// Outcomes of one planned exit in a run. Succeeded committed what was due;
// deferred committed nothing or kept the exit for a later run (its locks
// were held, or the offboarding is blocked and retried hourly); failed rolled
// the exit back or recorded a failed offboarding; aborted stopped the run.
const (
	ExitSucceeded = "succeeded"
	ExitDeferred  = "deferred"
	ExitFailed    = "failed"
	ExitAborted   = "aborted"
)

// DueExitResult is the outcome of one planned exit the run acted on.
type DueExitResult struct {
	StaffID int64
	Outcome string
	Err     error
}

// DueRun reports every planned exit the run acted on, in the run's order.
type DueRun struct {
	Exits []DueExitResult
}

// RunDue sends the reminders and runs the due exits of the school in ctx.
// Every planned exit with work due is one unit: its own transaction takes
// the exit's locks, bounded, and does its work (#6703). A failing or locked
// exit neither blocks the others nor undoes their work, and no transaction
// holds the locks of two people, so the lock order (#6583, #6702) holds
// within each. The caller's transaction only lists the plans.
func (d *DueExits) RunDue(ctx context.Context) (DueRun, error) {
	var run DueRun
	today, err := d.planner.today()
	if err != nil {
		return run, err
	}
	plans, err := d.planner.exits.ListPlannedExits(ctx)
	if err != nil {
		return run, err
	}
	now := d.deps.Now()
	for _, plan := range plans {
		if !workDue(plan, today, now) {
			continue
		}
		result, err := d.runUnit(ctx, plan, today)
		d.logResult(plan, result)
		run.Exits = append(run.Exits, result)
		if err != nil {
			return run, fmt.Errorf("planned exit of staff %d: %w", plan.StaffID, err)
		}
	}
	return run, nil
}

// workDue reports from the listed snapshot whether the plan may have work
// due: a reminder not yet sent, or an offboarding whose attempt is due. The
// unit decides again on the locked current plan; a plan that changed after
// the listing is the next run's.
func workDue(plan schoolmembership.PlannedExit, today calendar.Date, now time.Time) bool {
	day := plan.LastWorkingDay
	switch {
	case day.After(today):
		return inReminderWeek(today, day) && plan.WeekNoticeSentAt == nil
	case day == today:
		return plan.DayNoticeSentAt == nil
	default:
		return plan.NextAttemptAt == nil || !now.Before(*plan.NextAttemptAt)
	}
}

// runUnit runs one planned exit in its own transaction. err aborts the run;
// the result carries it as aborted.
func (d *DueExits) runUnit(ctx context.Context, plan schoolmembership.PlannedExit, today calendar.Date) (DueExitResult, error) {
	result := DueExitResult{StaffID: plan.StaffID, Outcome: ExitSucceeded}
	var exitErr error
	failed, err := d.deps.Within(ctx, func(txCtx context.Context) error {
		exitErr = nil
		return d.deps.BoundLockWaits(txCtx, func(lockCtx context.Context) error {
			if err := d.lockPlan(lockCtx, plan.StaffID); err != nil {
				return err
			}
			var err error
			exitErr, err = d.runPlan(lockCtx, plan, today)
			return err
		})
	})
	switch {
	case err != nil:
		result.Outcome, result.Err = ExitAborted, err
		return result, err
	case errors.Is(failed, ErrExitLocked):
		result.Outcome, result.Err = ExitDeferred, failed
	case failed != nil:
		result.Outcome, result.Err = ExitFailed, failed
	case errors.Is(exitErr, ErrInUse):
		result.Outcome, result.Err = ExitDeferred, exitErr
	case exitErr != nil:
		result.Outcome, result.Err = ExitFailed, exitErr
	}
	return result, nil
}

// logResult writes the one result line of a planned exit. It names the
// school and the staff member by ID only.
func (d *DueExits) logResult(plan schoolmembership.PlannedExit, result DueExitResult) {
	level := slog.LevelInfo
	switch result.Outcome {
	case ExitDeferred:
		level = slog.LevelWarn
	case ExitFailed, ExitAborted:
		level = slog.LevelError
	}
	attrs := []slog.Attr{
		slog.Int64("tenant_id", plan.TenantID),
		slog.Int64("staff_id", plan.StaffID),
		slog.String("outcome", result.Outcome),
	}
	if result.Err != nil {
		attrs = append(attrs, slog.String("error", result.Err.Error()))
	}
	d.deps.Logger.LogAttrs(context.Background(), level, "planned staff exit", attrs...)
}

// lockPlan takes the locks of one planned exit before its first write, in
// the order its offboarding takes them (Konto → Schule → Profil, #6702): the
// member's sign-in account, Workforce's absence and staff-shift gates, then
// the membership row. The exit's transaction keeps them until its commit and
// holds no other person's locks, so an offboarding that takes a gate never
// follows another person's staff row (tenant.LockStaffRows, #6583).
func (d *DueExits) lockPlan(ctx context.Context, staffID int64) error {
	if err := d.lockAccount(ctx, staffID); err != nil {
		return err
	}
	if err := d.planner.workflow.deps.Workforce.Lock(ctx, staffID); err != nil {
		return err
	}
	_, err := d.planner.exits.FindPlannedExitForMutation(ctx, staffID)
	if err != nil && !errors.Is(err, schoolmembership.ErrPlannedExitNotFound) && !errors.Is(err, schoolmembership.ErrStaffNotFound) {
		return err
	}
	return nil
}

// lockAccount locks the sign-in account of the exit before its gates. An
// exit's offboarding and a closed sign-in lock the account; an immediate
// offboarding locks it before its gates too, so an exit holding gates and
// the staff row never waits for an account that offboarding holds while it
// waits for those gates.
func (d *DueExits) lockAccount(ctx context.Context, staffID int64) error {
	deps := d.planner.workflow.deps
	// A retired or missing member is no exit any more; runPlan skips it.
	staff, err := deps.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{staffID}, MembershipOnly: true})
	if err != nil || len(staff) == 0 {
		return err
	}
	people, err := deps.People.ListPersonsByID(ctx, []int64{staff[0].PersonID})
	if err != nil {
		return err
	}
	accounts := make([]int64, 0, len(people))
	for _, person := range people {
		if person.AccountID != nil {
			accounts = append(accounts, *person.AccountID)
		}
	}
	if len(accounts) == 0 {
		return nil
	}
	return deps.LockAccounts(ctx, accounts)
}

// runPlan does what is due for one planned exit. exitErr is a failed or
// blocked (ErrInUse) offboarding whose closed sign-in and next attempt are
// kept; err rolls the planned exit's transaction back.
func (d *DueExits) runPlan(ctx context.Context, listed schoolmembership.PlannedExit, today calendar.Date) (exitErr, err error) {
	day := listed.LastWorkingDay
	if day.After(today) && !inReminderWeek(today, day) {
		return nil, nil
	}

	// ListPlannedExits only finds candidates. The membership lock makes the
	// decision, its progress claim and the notification outbox one unit: a
	// move or withdrawal that won the lock is never notified from this
	// snapshot, and a later move waits until the current notice commits.
	plan, err := d.planner.exits.FindPlannedExitForMutation(ctx, listed.StaffID)
	if errors.Is(err, schoolmembership.ErrPlannedExitNotFound) || errors.Is(err, schoolmembership.ErrStaffNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	day = plan.LastWorkingDay
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

// runExit offboards the locked current plan through the immediate deletion's
// execution path, acting as its planner, once it is due. A failed offboarding
// closes the member's sign-in at this school anyway and is retried every hour
// until it succeeds; a blocked one also tells the Leitung once.
func (d *DueExits) runExit(ctx context.Context, plan schoolmembership.PlannedExit, today calendar.Date) (exitErr, err error) {
	now := d.deps.Now()
	if !plan.LastWorkingDay.Before(today) ||
		(plan.NextAttemptAt != nil && now.Before(*plan.NextAttemptAt)) {
		return nil, err
	}
	asPlanner := plannerActor(plan)
	execErr, err := d.deps.Isolate(ctx, func(spCtx context.Context) error {
		// Classify lock timeouts before the savepoint rollback consumes the
		// execution error, so contention never records a failed offboarding.
		return d.deps.BoundLockWaits(spCtx, func(lockCtx context.Context) error {
			_, err := d.planner.workflow.executeAs(lockCtx, "planned_exit", plan.StaffID, "", asPlanner)
			return err
		})
	})
	if err != nil || execErr == nil {
		return nil, err
	}
	if errors.Is(execErr, ErrExitLocked) {
		return nil, execErr
	}
	if err := d.recordFailedExit(ctx, plan, now, errors.Is(execErr, ErrInUse), asPlanner); err != nil {
		return nil, err
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
