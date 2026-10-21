package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	activityrecords "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/activityrecords"
)

// GroupRows are the owner's activity group and target reads and writes.
type GroupRows interface {
	timetable.ActivityGroupQuery
	timetable.ActivityGroupCommand
}

type PlannedSupervisorRecords interface {
	Create(context.Context, *activityContract.SupervisorPlanned) error
	FindByID(context.Context, int64) (*activityContract.SupervisorPlanned, error)
	Update(context.Context, *activityContract.SupervisorPlanned) error
	Delete(context.Context, int64) error

	// ListPlannedSupervisionBlockers returns planned activity supervisions
	// as caregiver-capability blocker rows.
	ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]activityrecords.PlannedSupervisionBlocker, error)

	// CloseOpenByGroupAndPeriod closes the open planned supervisions of a
	// group for the given calendar period (NULL period matches rows without
	// one) by setting valid_until.
	CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom activityContract.Date) error

	// FindByStaffID finds all supervisions for a specific staff member
	FindByStaffID(ctx context.Context, staffID int64) ([]*activityContract.SupervisorPlanned, error)

	// FindByGroupID finds all supervisors for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.SupervisorPlanned, error)

	// FindByGroupIDs finds all supervisors for multiple groups in a single query
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activityContract.SupervisorPlanned, error)

	// SetPrimary sets a supervisor as the primary supervisor for a group
	SetPrimary(ctx context.Context, id int64) error

	// CapActiveByGroup caps open supervision rows (valid_until IS NULL).
	// Rows starting on/after the cap are deleted because they have no interval
	// left; begun rows are ended at validUntil. Bounded rows are left to their
	// owning workflow. Returns the total number changed.
	CapActiveByGroup(ctx context.Context, groupID int64, validUntil activityContract.Date) (int64, error)

	// SetValidUntilByID closes exactly one supervision without writing the
	// rest of a potentially partial read model back to the database.
	SetValidUntilByID(ctx context.Context, id int64, validUntil activityContract.Date) error
}

type ActivityEnrollmentRecords interface {
	FindByStudentIDs(context.Context, []int64) ([]*activityContract.StudentEnrollment, error)
	Create(context.Context, *activityContract.StudentEnrollment) error
	FindByID(context.Context, int64) (*activityContract.StudentEnrollment, error)
	Update(context.Context, *activityContract.StudentEnrollment) error
	Delete(context.Context, int64) error

	// CloseOpenByGroupAndPeriod closes the open enrollments of a group for
	// the given calendar period (NULL period matches rows without one) by
	// setting valid_until.
	CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom activityContract.Date) error

	// FindByStudentID finds all enrollments for a specific student
	FindByStudentID(ctx context.Context, studentID int64) ([]*activityContract.StudentEnrollment, error)

	// FindActiveByStudentIDs finds active enrollments for a batch of students
	// on the given date. valid_until is exclusive.
	FindActiveByStudentIDs(ctx context.Context, studentIDs []int64, onDate activityContract.Date) ([]*activityContract.StudentEnrollment, error)

	// FindByGroupID finds all enrollments for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.StudentEnrollment, error)

	// CapActiveByGroup caps open enrollment rows (valid_until IS NULL). Rows
	// starting on/after the cap are deleted because they have no interval left;
	// begun rows are ended at validUntil. Bounded rows are left to their owning
	// workflow. Returns the total number changed.
	CapActiveByGroup(ctx context.Context, groupID int64, validUntil activityContract.Date) (int64, error)

	// SetValidUntilByID closes exactly one enrollment without writing the
	// rest of a potentially partial read model back to the database.
	SetValidUntilByID(ctx context.Context, id int64, validUntil activityContract.Date) error
}
