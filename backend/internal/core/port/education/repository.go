package education

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	domain "github.com/moto-nrw/project-phoenix/internal/core/domain/education"
)

type Group = domain.Group
type GroupTeacher = domain.GroupTeacher
type GroupSubstitution = domain.GroupSubstitution

// GroupRepository defines operations for managing education groups
type GroupRepository interface {
	Create(ctx context.Context, group *Group) error
	FindByID(ctx context.Context, id interface{}) (*Group, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Group, error)
	Update(ctx context.Context, group *Group) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Group, error)
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Group, error)
	FindByName(ctx context.Context, name string) (*Group, error)
	FindByRoom(ctx context.Context, roomID int64) ([]*Group, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*Group, error)
	FindWithRoom(ctx context.Context, groupID int64) (*Group, error)
}

// GroupTeacherRepository defines operations for managing group-teacher relationships
type GroupTeacherRepository interface {
	Create(ctx context.Context, groupTeacher *GroupTeacher) error
	FindByID(ctx context.Context, id interface{}) (*GroupTeacher, error)
	Update(ctx context.Context, groupTeacher *GroupTeacher) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*GroupTeacher, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*GroupTeacher, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*GroupTeacher, error)
}

// GroupSubstitutionRepository defines operations for managing group substitutions (without relation loading).
type GroupSubstitutionRepository interface {
	Create(ctx context.Context, substitution *GroupSubstitution) error
	FindByID(ctx context.Context, id interface{}) (*GroupSubstitution, error)
	Update(ctx context.Context, substitution *GroupSubstitution) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*GroupSubstitution, error)
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*GroupSubstitution, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*GroupSubstitution, error)
	FindByRegularStaff(ctx context.Context, staffID int64) ([]*GroupSubstitution, error)
	FindBySubstituteStaff(ctx context.Context, staffID int64) ([]*GroupSubstitution, error)
	FindActive(ctx context.Context, date time.Time) ([]*GroupSubstitution, error)
	FindActiveBySubstitute(ctx context.Context, substituteStaffID int64, date time.Time) ([]*GroupSubstitution, error)
	FindActiveByGroup(ctx context.Context, groupID int64, date time.Time) ([]*GroupSubstitution, error)
	FindOverlapping(ctx context.Context, staffID int64, startDate time.Time, endDate time.Time) ([]*GroupSubstitution, error)
}

// GroupSubstitutionRelationsRepository defines operations that load related data.
type GroupSubstitutionRelationsRepository interface {
	FindByIDWithRelations(ctx context.Context, id int64) (*GroupSubstitution, error)
	ListWithRelations(ctx context.Context, options *base.QueryOptions) ([]*GroupSubstitution, error)
	FindActiveWithRelations(ctx context.Context, date time.Time) ([]*GroupSubstitution, error)
	FindActiveBySubstituteWithRelations(ctx context.Context, substituteStaffID int64, date time.Time) ([]*GroupSubstitution, error)
	FindActiveByGroupWithRelations(ctx context.Context, groupID int64, date time.Time) ([]*GroupSubstitution, error)
}
