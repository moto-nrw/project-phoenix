package education

import (
	"context"
	"time"
)

// GroupRepository defines operations for managing education groups
type GroupRepository interface {
	Create(ctx context.Context, group *Group) error
	FindByID(ctx context.Context, id interface{}) (*Group, error)
	Update(ctx context.Context, group *Group) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Group, error)
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

// GroupSubstitutionRepository defines operations for managing group substitutions
type GroupSubstitutionRepository interface {
	Create(ctx context.Context, substitution *GroupSubstitution) error
	FindByID(ctx context.Context, id interface{}) (*GroupSubstitution, error)
	Update(ctx context.Context, substitution *GroupSubstitution) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*GroupSubstitution, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*GroupSubstitution, error)
	FindByRegularStaff(ctx context.Context, staffID int64) ([]*GroupSubstitution, error)
	FindBySubstituteStaff(ctx context.Context, staffID int64) ([]*GroupSubstitution, error)
	FindActive(ctx context.Context, date time.Time) ([]*GroupSubstitution, error)
	FindActiveByGroup(ctx context.Context, groupID int64, date time.Time) ([]*GroupSubstitution, error)
	FindOverlapping(ctx context.Context, staffID int64, startDate time.Time, endDate time.Time) ([]*GroupSubstitution, error)
}
