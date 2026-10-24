package schoolmembership

import "context"

// StudentEnrollmentCommands is the lifecycle capability, separate from staff
// and group administration.
//
// Every write that can raise the Kontingentzahl (Enroll, RenewEnrollment,
// SetStatus, ResumeCare, Reactivate) is checked against the school's
// Kinderkontingent in the same transaction and refused with
// ChildQuotaReachedError when it would exceed it (#3567). TransitionStatus
// checks the scheduler's pending → active step as well when a started
// pending child becomes counted.
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
