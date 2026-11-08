package application

import (
	"context"
	"fmt"
	"log/slog"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// targetedFieldScope selects the targets one dispatch writes.
type targetedFieldScope int

const (
	// allTargets dispatches every targeted field.
	allTargets targetedFieldScope = iota
	// withoutWeeklyPlan dispatches everything except the weekly plan.
	withoutWeeklyPlan
	// weeklyPlanOnly dispatches only the weekly plan.
	weeklyPlanOnly
)

// isWeeklyPlanTarget reports whether a target belongs to the child's weekly
// plan: the weekly pickup times, the care days and the Heimweg rules. They
// describe how the care runs from a day on, so a renewal approved before its
// care starts applies them only at the start (#6043). Every other target is
// a fact about the child (health, contacts, consent) and applies at approval.
func isWeeklyPlanTarget(target string) bool {
	switch target {
	case enrollmentNative.TargetSchedulePickup,
		enrollmentNative.TargetScheduleArrival,
		enrollmentNative.TargetStudentDeparture,
		enrollmentNative.TargetStudentAllowedDepartureModes,
		enrollmentNative.TargetStudentBusDays,
		enrollmentNative.TargetStudentBus,
		enrollmentNative.TargetStudentPickupStatus:
		return true
	}
	return false
}

func (s targetedFieldScope) includes(target string) bool {
	switch s {
	case withoutWeeklyPlan:
		return !isWeeklyPlanTarget(target)
	case weeklyPlanOnly:
		return isWeeklyPlanTarget(target)
	default:
		return true
	}
}

// renewalWeeklyPlanDay is the day a renewal's weekly plan applies when the
// approval must not apply it at once, or nil. A child the school still cares
// for keeps its running weekly plan until the renewed care starts (#6043);
// a child without running care gets the plan at once, as before.
func (d *Decisions) renewalWeeklyPlanDay(phase *enrollmentNative.Phase, careRunning bool) *enrollmentNative.Date {
	start := calendar.Date(phase.ServiceStartDate)
	if !careRunning || !start.After(d.todayDate()) {
		return nil
	}
	on := phase.ServiceStartDate
	return &on
}

// planWeeklyPlan stores the day the request child's weekly plan applies, or
// clears the plan when on is nil. The approval plans it like the approval of
// a renewal whose care has started: a field without a value keeps its
// target. The reviewer's staff membership is resolved now and stored as the
// author of the rows the start writes, so the start does not depend on that
// account months later.
func (d *Decisions) planWeeklyPlan(ctx context.Context, requestChildID int64, on *enrollmentNative.Date, reviewedBy int64) error {
	var plan *enrollmentNative.WeeklyPlan
	if on != nil {
		plan = &enrollmentNative.WeeklyPlan{RequestChildID: requestChildID, On: *on, AuthorStaffID: d.weeklyPlanAuthor(ctx, reviewedBy)}
	}
	return d.storeWeeklyPlan(ctx, requestChildID, plan)
}

// weeklyPlanAuthor is the staff membership of accountID, or 0 when the
// account has none. A missing author only matters once the start writes
// weekly rows; the tick then reports the failure and keeps the plan due.
func (d *Decisions) weeklyPlanAuthor(ctx context.Context, accountID int64) int64 {
	if accountID <= 0 {
		return 0
	}
	staffID, err := d.resolveReviewerStaffID(ctx, accountID)
	if err != nil {
		return 0
	}
	return staffID
}

func (d *Decisions) storeWeeklyPlan(ctx context.Context, requestChildID int64, plan *enrollmentNative.WeeklyPlan) error {
	if err := d.deps.Children.SetChildWeeklyPlan(ctx, requestChildID, plan); err != nil {
		return fmt.Errorf("decision: plan weekly plan: %w", err)
	}
	return nil
}

// approvedChildTargetScope is the scope of a confirmed edit of an approved
// child. While the weekly plan is still planned for a future day, the edit
// changes what the start applies, not the running plan; a replacing edit
// makes the start apply the submission as the complete weekly plan, so a
// field the edit removed clears its target then, as it would at once. A plan
// whose day has come but which the tick has not applied yet is applied by
// the edit itself.
func (d *Decisions) approvedChildTargetScope(ctx context.Context, run *approvedChildSync) (targetedFieldScope, error) {
	requestChildID := run.child.ID
	planned, err := d.deps.Children.ChildWeeklyPlan(ctx, requestChildID)
	if err != nil {
		return allTargets, fmt.Errorf("decision: read planned weekly plan: %w", err)
	}
	if planned == nil {
		return allTargets, nil
	}
	updated := *planned
	updated.Replace = planned.Replace || run.input.ReplaceTargetedData
	if author := d.weeklyPlanAuthor(ctx, run.input.ActorAccountID); author > 0 {
		updated.AuthorStaffID = author
	}
	if !calendar.Date(planned.On).After(d.todayDate()) {
		return withoutWeeklyPlan, d.applyPlannedWeeklyFields(ctx, run.request, run.child, run.student, run.input.ActorAccountID, updated)
	}
	if updated != *planned {
		if err := d.storeWeeklyPlan(ctx, requestChildID, &updated); err != nil {
			return allTargets, err
		}
	}
	return withoutWeeklyPlan, nil
}

// ApplyDueWeeklyPlans replaces the running weekly plan of every approved
// re-enrollment whose planned day has come with the approved submission and
// clears the plans (#6043). The rollover tick runs it in the tenant
// transaction. Connected companion plans share a savepoint and a final stranding
// verdict. A failure keeps that group due without blocking unrelated plans.
func (d *Decisions) ApplyDueWeeklyPlans(ctx context.Context, asOf calendar.Date) (int, error) {
	// The gates before the plans and the student rows, as an approval takes
	// them: a phase edit reschedules pending plans under the recurrence gate.
	if err := d.LockApprovalGates(ctx); err != nil {
		return 0, err
	}
	due, err := d.deps.Children.DueWeeklyPlans(ctx, enrollmentNative.Date(asOf.String()))
	if err != nil {
		return 0, fmt.Errorf("weekly plan: list due plans: %w", err)
	}
	groups, err := d.weeklyPlanGroups(ctx, due)
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, group := range groups {
		var count int
		apply := func(applyCtx context.Context) error {
			var applyErr error
			count, applyErr = d.applyWeeklyPlanGroup(applyCtx, group)
			return applyErr
		}
		if d.deps.Runtime.InTransaction(ctx) {
			err = d.deps.Runtime.Savepoint(ctx, apply)
		} else {
			err = apply(ctx)
		}
		if err != nil && d.deps.Runtime.IsSavepointControl(err) {
			return applied, fmt.Errorf("weekly plan: savepoint for request child %d: %w", group.plans[0].RequestChildID, err)
		}
		if err != nil {
			d.logger().Error("weekly plan: apply failed, the running plan stays",
				slog.Int64("request_child_id", group.plans[0].RequestChildID),
				slog.String("error", err.Error()))
			continue
		}
		applied += count
	}
	return applied, nil
}

// applyWeeklyPlan applies one due plan and clears it. A plan whose child is
// no longer approved, cared for or authorized is dropped without applying.
func (d *Decisions) applyWeeklyPlan(ctx context.Context, plan enrollmentNative.WeeklyPlan) (bool, error) {
	requestChildID := plan.RequestChildID
	child, err := decodedChildByID(ctx, d.deps.Children, requestChildID)
	if err != nil {
		return false, fmt.Errorf("weekly plan: load request child: %w", err)
	}
	student, err := d.weeklyPlanStudent(ctx, child)
	if err != nil || student == nil {
		return false, d.finishWeeklyPlan(ctx, requestChildID, err)
	}
	request, err := decodedRequestByID(ctx, d.deps.Requests, child.RequestID, false)
	if err != nil {
		return false, fmt.Errorf("weekly plan: load request: %w", err)
	}
	enrollmentOnly, err := d.childEnrollmentOnly(ctx, request, child)
	if err != nil || enrollmentOnly {
		return false, d.finishWeeklyPlan(ctx, requestChildID, err)
	}
	// The tick is the system actor; the stored staff ID authors weekly rows.
	// A plan stored without an author (the reviewer had no staff membership
	// then) falls back to the reviewer as it is now.
	if plan.AuthorStaffID == 0 && child.ReviewedBy != nil {
		plan.AuthorStaffID = d.weeklyPlanAuthor(ctx, *child.ReviewedBy)
	}
	if err := d.applyPlannedWeeklyFields(ctx, request, child, student, 0, plan); err != nil {
		return false, err
	}
	return true, nil
}

// applyPlannedWeeklyFields shares activation semantics between the tick and
// a correction made after the plan became due. Use the caller's student so
// the correction's remaining fields cannot overwrite the activated plan.
func (d *Decisions) applyPlannedWeeklyFields(ctx context.Context, request *Request, child *RequestChild, student *Student, actorAccountID int64, plan enrollmentNative.WeeklyPlan) error {
	before := *student
	planSynced, err := d.applyTargetedFields(ctx, request, child, student, nil, actorAccountID, targetedFieldSyncOptions{
		Replace:               plan.Replace,
		ReplaceSchedules:      true,
		Scope:                 weeklyPlanOnly,
		ScheduleAuthorStaffID: plan.AuthorStaffID,
	})
	if err != nil {
		return fmt.Errorf("weekly plan: apply: %w", err)
	}
	if d.deps.StudentAudit != nil {
		var auditErr error
		if actorAccountID == 0 {
			auditErr = d.deps.StudentAudit.RecordSystemChanges(ctx, &before, student)
		} else {
			auditErr = d.deps.StudentAudit.RecordChangesForActor(ctx, &before, student, actorAccountID)
		}
		if auditErr != nil {
			return fmt.Errorf("weekly plan: audit departure change: %w", auditErr)
		}
	}
	if err := d.finishWeeklyPlan(ctx, plan.RequestChildID, nil); err != nil {
		return err
	}
	d.deferStudentPlanBroadcasts(ctx, student.ID, planSynced)
	return nil
}

// weeklyPlanStudent locks the student a due plan applies to; nil when the
// plan no longer applies.
func (d *Decisions) weeklyPlanStudent(ctx context.Context, child *RequestChild) (*Student, error) {
	if child == nil || child.Status != enrollmentNative.ChildStatusApproved || child.CreatedStudentID == nil {
		return nil, nil
	}
	student, err := d.readEnrollmentStudent(ctx, *child.CreatedStudentID, "update")
	if err != nil && d.deps.Runtime.NotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("weekly plan: load student %d: %w", *child.CreatedStudentID, err)
	}
	if student == nil || (student.Status != studentStatusActive && student.Status != studentStatusPending) {
		return nil, nil
	}
	return student, nil
}

// finishWeeklyPlan clears a handled plan; a failure keeps it due.
func (d *Decisions) finishWeeklyPlan(ctx context.Context, requestChildID int64, err error) error {
	if err != nil {
		return err
	}
	if err := d.deps.Children.SetChildWeeklyPlan(ctx, requestChildID, nil); err != nil {
		return fmt.Errorf("weekly plan: clear request child %d: %w", requestChildID, err)
	}
	return nil
}
