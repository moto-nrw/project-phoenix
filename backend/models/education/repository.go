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

	// Exists reports whether a group with the given ID exists in the current
	// tenant (issue #584: moved from api/timetable template validation).
	Exists(ctx context.Context, id int64) (bool, error)
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Group, error)
	FindByName(ctx context.Context, name string) (*Group, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*Group, error)
	FindWithRoom(ctx context.Context, groupID int64) (*Group, error)
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
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
