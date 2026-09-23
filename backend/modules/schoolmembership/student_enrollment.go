package schoolmembership

import (
	"context"
	"strings"
)

// StudentEnrollmentCommands is the lifecycle capability, separate from staff
// and group administration.
//
// Every write that can raise the Kontingentzahl (Enroll, RenewEnrollment,
// SetStatus, ResumeCare, Reactivate) is checked against the school's
// Kinderkontingent in the same transaction and refused with
// ChildQuotaReachedError when it would exceed it (#3567). TransitionStatus
// is the scheduler's pending → active step and is not checked: a pending
// child already counts.
type StudentEnrollmentCommands interface {
	TransitionStatus(context.Context, int64, string, string) (bool, error)
	SetStatus(context.Context, int64, string) (bool, error)
	Enroll(context.Context, StudentEnrollment) (int64, error)
	RenewEnrollment(context.Context, StudentEnrollment) (int64, error)
	AssignGroup(context.Context, int64, *int64) (bool, error)
	LockStudentClassWrites(context.Context, bool) error
	EndCare(context.Context, []int64, string) (int64, error)
	ResumeCare(context.Context, int64, string, string, string) (bool, error)
	Graduate(context.Context, []int64) (int64, error)
	Reactivate(context.Context, []int64, string, ChildQuotaCheck) ([]int64, error)
	ChangeClass(context.Context, []int64, string, string) (int64, error)
}

// LockStudentClassWrites fences membership writers before any student row lock.
// Grade transition takes it exclusively; individual writers share it.
func (m *Module) LockStudentClassWrites(ctx context.Context, exclusive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.engine.LockStudentClassWrites(ctx, exclusive)
}

func (m *Module) Graduate(ctx context.Context, ids []int64) (int64, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	return m.engine.GraduateStudents(ctx, ids)
}

// TransitionStatus is the scheduler's compare-and-set. Graduation and
// reactivation are separate commands and cannot be requested through it.
func (m *Module) TransitionStatus(ctx context.Context, id int64, expected, next string) (bool, error) {
	if id <= 0 {
		return false, invalid("student ID is required")
	}
	for _, value := range []string{expected, next} {
		if value != "active" && value != "pending" && value != "inactive" {
			return false, invalid("status transition requires non-alumni lifecycle states")
		}
	}
	return m.engine.TransitionStudentStatus(ctx, id, expected, next)
}

func (m *Module) SetStatus(ctx context.Context, id int64, status string) (bool, error) {
	if id <= 0 {
		return false, invalid("student ID is required")
	}
	if status != "active" && status != "pending" && status != "inactive" {
		return false, invalid("status change requires a non-alumni lifecycle state")
	}
	return m.engine.SetStudentStatus(ctx, id, status)
}

// Reactivate brings graduated children back. The whole batch is checked
// against the Kinderkontingent as one write unless the caller reverts a grade
// transition and says so.
func (m *Module) Reactivate(ctx context.Context, ids []int64, status string, check ChildQuotaCheck) ([]int64, error) {
	ids = uniquePositive(ids)
	status = strings.TrimSpace(status)
	if status != "active" && status != "pending" && status != "inactive" {
		return nil, invalid("reactivation requires a non-alumni lifecycle status")
	}
	if !check.valid() {
		return nil, invalid("reactivation requires an explicit child quota check")
	}
	if len(ids) == 0 {
		return []int64{}, nil
	}
	return m.engine.ReactivateStudents(ctx, ids, status, check == EnforceChildQuota)
}

func validateStudentIDs(ids []int64) error {
	for _, id := range ids {
		if id <= 0 {
			return invalid("student ID is required")
		}
	}
	return nil
}

func (m *Module) EndCare(ctx context.Context, ids []int64, until string) (int64, error) {
	ids = uniquePositive(ids)
	if err := validateDate(until, "care end"); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return m.engine.EndStudentCare(ctx, ids, until)
}

// ResumeCare changes only an ended, non-alumni care interval. On is the
// caller's frozen calendar day, not the database server's timezone.
func (m *Module) ResumeCare(ctx context.Context, id int64, from, status, on string) (bool, error) {
	if err := validateStudentIDs([]int64{id}); err != nil {
		return false, err
	}
	if from == "" || on == "" {
		return false, invalid("care start and current day are required")
	}
	if err := validateDate(from, "care start"); err != nil {
		return false, err
	}
	if err := validateDate(on, "current day"); err != nil {
		return false, err
	}
	if status != "active" && status != "pending" && status != "inactive" {
		return false, invalid("care resume requires a non-alumni lifecycle status")
	}
	return m.engine.ResumeStudentCare(ctx, id, from, status, on)
}

// ChangeClass compares the current class before changing it, making retry and
// grade-transition reversal conditional on the state the caller observed.
func (m *Module) ChangeClass(ctx context.Context, ids []int64, from, to string) (int64, error) {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return 0, invalid("source and destination class are required")
	}
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	return m.engine.ChangeStudentClass(ctx, ids, from, to)
}
