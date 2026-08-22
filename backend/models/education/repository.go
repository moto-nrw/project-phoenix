package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// GroupRepository defines operations for managing education groups
type GroupRepository interface {
	base.CRUDRepository[*Group]
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Group, error)
	// FindByIDsWithRooms is the bulk sibling of FindWithRoom: one LEFT JOIN
	// resolves every group's room relation (#2094 review).
	FindByIDsWithRooms(ctx context.Context, ids []int64) (map[int64]*Group, error)

	// Exists reports whether a group with the given ID exists in the current
	// tenant (issue #584: moved from api/timetable template validation).
	Exists(ctx context.Context, id int64) (bool, error)
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Group, error)
	FindByName(ctx context.Context, name string) (*Group, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*Group, error)
	FindWithRoom(ctx context.Context, groupID int64) (*Group, error)
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)

	// ListStaffIDsByEducationGroupIDs answers who supervises these groups on
	// the given day. Producers use it to find the people responsible for one
	// child. It mirrors usercontext.GetMyGroups from the group side, so the
	// two must agree on the join shape.
	ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on timezone.Date) ([]StaffGroupID, error)
}

// StaffGroupID pairs a staff member with one education group they supervise:
// callers need the IDs, never the group rows themselves.
type StaffGroupID struct {
	StaffID int64 `bun:"staff_id"`
	GroupID int64 `bun:"group_id"`
}

// GroupTeacherRepository defines operations for managing group-teacher relationships
type GroupTeacherRepository interface {
	base.CRUDRepository[*GroupTeacher]
	FindByGroup(ctx context.Context, groupID int64) ([]*GroupTeacher, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*GroupTeacher, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*GroupTeacher, error)
	// DeleteByTeacherID removes all group assignments for a teacher
	// (staff offboarding cleanup).
	DeleteByTeacherID(ctx context.Context, teacherID int64) (int64, error)
	// ListGroupTeacherBlockers returns group assignments as
	// caregiver-capability blocker rows.
	ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]users.BlockerGroup, error)
}

// ClassTeacherRepository defines operations for managing staff-to-school-class
// assignments (#1772). School classes are free-text strings; every class
// comparison uses LOWER(BTRIM(...)) — see models/education.ClassTeacher.
type ClassTeacherRepository interface {
	base.CRUDRepository[*ClassTeacher]
	// FindByStaff returns the class assignments of one staff member.
	FindByStaff(ctx context.Context, staffID int64) ([]*ClassTeacher, error)
	// DeleteByStaffID removes all class assignments for a staff member
	// (staff offboarding cleanup — staff rows are only soft-deleted, so the
	// FK cascade never fires).
	DeleteByStaffID(ctx context.Context, staffID int64) (int64, error)
}

// GroupSubstitutionRepository defines operations for managing group substitutions
type GroupSubstitutionRepository interface {
	base.CRUDRepository[*GroupSubstitution]
	// ListActiveSubstitutionBlockers returns current/upcoming substitutions
	// as caregiver-capability blocker rows.
	ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]users.BlockerSubstitution, error)
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*GroupSubstitution, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*GroupSubstitution, error)
	FindByRegularStaff(ctx context.Context, staffID int64) ([]*GroupSubstitution, error)
	FindBySubstituteStaff(ctx context.Context, staffID int64) ([]*GroupSubstitution, error)
	FindActive(ctx context.Context, date timezone.Date) ([]*GroupSubstitution, error)
	FindActiveBySubstitute(ctx context.Context, substituteStaffID int64, date timezone.Date) ([]*GroupSubstitution, error)
	FindOverlapping(ctx context.Context, staffID int64, startDate timezone.Date, endDate timezone.Date) ([]*GroupSubstitution, error)
	// DeleteActiveOrFutureByStaffID removes substitutions involving the staff
	// member (as regular or substitute) that have not ended before the given
	// date. Past substitutions are kept as history (staff offboarding cleanup).
	DeleteActiveOrFutureByStaffID(ctx context.Context, staffID int64, from timezone.Date) (int64, error)

	// Methods with related data loading
	FindByIDWithRelations(ctx context.Context, id int64) (*GroupSubstitution, error)
	ListWithRelations(ctx context.Context, options *base.QueryOptions) ([]*GroupSubstitution, error)
	FindActiveWithRelations(ctx context.Context, date timezone.Date) ([]*GroupSubstitution, error)
	FindActiveBySubstituteWithRelations(ctx context.Context, substituteStaffID int64, date timezone.Date) ([]*GroupSubstitution, error)
	FindActiveByGroupWithRelations(ctx context.Context, groupID int64, date timezone.Date) ([]*GroupSubstitution, error)
}

// ClassArrivalTimeRepository is the data access boundary for class arrival
// times.
type ClassArrivalTimeRepository interface {
	base.CRUDRepository[*ClassArrivalTime]

	// FindByClasses returns the rows for the given school classes, matched
	// case-insensitively on the normalized class. Classes without a row are
	// simply absent from the result.
	FindByClasses(ctx context.Context, classes []string) ([]*ClassArrivalTime, error)

	// Upsert stores the weekday map for one class, replacing what was there.
	Upsert(ctx context.Context, row *ClassArrivalTime) error

	// DeleteByClass removes the row for one class, if any.
	DeleteByClass(ctx context.Context, class string) error
}
