package application

import (
	"context"
	"fmt"
	"log/slog"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// planClassSwitch stores the class switch an approval planned for the request
// child, or clears it when change is nil or keeps the class (#3917).
func (d *Decisions) planClassSwitch(ctx context.Context, requestChildID int64, change *enrollment.ClassSwitch) error {
	if change != nil && change.From == change.To {
		change = nil
	}
	if err := d.deps.Children.SetChildClassSwitch(ctx, requestChildID, change); err != nil {
		return fmt.Errorf("decision: plan class switch: %w", err)
	}
	return nil
}

// replanApprovedChildClass re-derives a still planned class switch after a
// confirmed edit of the approved child. The child keeps its running class;
// only the planned target follows the edit. ok is false when no switch is
// planned for a future day, so the caller writes the class at once.
func (d *Decisions) replanApprovedChildClass(ctx context.Context, run *approvedChildSync) (bool, error) {
	planned, err := d.deps.Children.ChildClassSwitch(ctx, run.child.ID)
	if err != nil {
		return false, fmt.Errorf("decision: read planned class switch: %w", err)
	}
	if planned == nil {
		return false, nil
	}
	if !calendar.Date(planned.On).After(d.todayDate()) {
		// The switch day has come but the tick has not run yet: the edit
		// writes the class now, which makes the plan obsolete.
		return false, d.planClassSwitch(ctx, run.child.ID, nil)
	}
	current := run.student.SchoolClass
	replanned := &enrollment.ClassSwitch{
		RequestChildID: run.child.ID, From: current, To: resolveRolloverSchoolClass(run.child, current), On: planned.On,
	}
	return true, d.planClassSwitch(ctx, run.child.ID, replanned)
}

// ApplyDueClassSwitches moves the children whose planned class switch is due
// into their new class and clears the plans (#3917). The rollover tick runs it
// in the tenant transaction. A switch applies only while the request child is
// still approved and the child is still enrolled in the class it had at
// approval: a manual class edit or a grade transition in between wins and the
// plan is dropped. Like an approval that changes a class, it takes the
// class-writes and recurrence gates first and resyncs the offering-sourced
// templates from asOf, so the Jahrgang-filtered rosters follow the new class.
func (d *Decisions) ApplyDueClassSwitches(ctx context.Context, asOf calendar.Date) (int, error) {
	due, err := d.deps.Children.DueClassSwitches(ctx, enrollment.Date(asOf.String()))
	if err != nil {
		return 0, fmt.Errorf("class switch: list due switches: %w", err)
	}
	if len(due) == 0 {
		return 0, nil
	}
	if err := d.lockExistingStudentGates(ctx); err != nil {
		return 0, err
	}
	applied := 0
	for _, change := range due {
		ok, err := d.applyClassSwitch(ctx, change)
		if err != nil {
			return applied, err
		}
		if ok {
			applied++
		}
		if err := d.deps.Children.SetChildClassSwitch(ctx, change.RequestChildID, nil); err != nil {
			return applied, fmt.Errorf("class switch: clear request child %d: %w", change.RequestChildID, err)
		}
	}
	if applied > 0 {
		if err := d.resyncOfferingSourcedTemplates(ctx, asOf); err != nil {
			return applied, fmt.Errorf("class switch: resync sourced templates: %w", err)
		}
	}
	return applied, nil
}

func (d *Decisions) applyClassSwitch(ctx context.Context, change enrollment.DueClassSwitch) (bool, error) {
	if change.ChildStatus != enrollmentModels.ChildStatusApproved || change.CreatedStudentID == nil {
		return false, nil
	}
	student, err := d.readEnrollmentStudent(ctx, *change.CreatedStudentID, "update")
	if err != nil && d.deps.Runtime.NotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("class switch: load student %d: %w", *change.CreatedStudentID, err)
	}
	if student == nil || (student.Status != studentStatusActive && student.Status != studentStatusPending) {
		return false, nil
	}
	if student.SchoolClass != change.From {
		d.logger().Info("class switch skipped: class changed since approval",
			slog.Int64("request_child_id", change.RequestChildID),
			slog.Int64("student_id", student.ID))
		return false, nil
	}
	student.SchoolClass = change.To
	if err := d.deps.StudentEnrollment.RenewEnrollmentStudent(ctx, student.ID, enrollmentStudentInput(student)); err != nil {
		return false, fmt.Errorf("class switch: update student %d: %w", student.ID, err)
	}
	return true, nil
}
