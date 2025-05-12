// Package education provides services for managing educational groups and related entities
package education

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Service defines operations for managing educational groups and their relationships
type Service interface {
	base.TransactionalService
	// Group operations
	GetGroup(ctx context.Context, id int64) (*education.Group, error)
	CreateGroup(ctx context.Context, group *education.Group) error
	UpdateGroup(ctx context.Context, group *education.Group) error
	DeleteGroup(ctx context.Context, id int64) error
	ListGroups(ctx context.Context, options *base.QueryOptions) ([]*education.Group, error)
	FindGroupByName(ctx context.Context, name string) (*education.Group, error)
	FindGroupsByRoom(ctx context.Context, roomID int64) ([]*education.Group, error)
	FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error)
	AssignRoomToGroup(ctx context.Context, groupID, roomID int64) error
	RemoveRoomFromGroup(ctx context.Context, groupID int64) error

	// Group-Teacher operations
	AddTeacherToGroup(ctx context.Context, groupID, teacherID int64) error
	RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error
	GetGroupTeachers(ctx context.Context, groupID int64) ([]*users.Teacher, error)
	GetTeacherGroups(ctx context.Context, teacherID int64) ([]*education.Group, error)

	// Substitution operations
	CreateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error
	UpdateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error
	DeleteSubstitution(ctx context.Context, id int64) error
	GetSubstitution(ctx context.Context, id int64) (*education.GroupSubstitution, error)
	ListSubstitutions(ctx context.Context, options *base.QueryOptions) ([]*education.GroupSubstitution, error)
	GetActiveSubstitutions(ctx context.Context, date time.Time) ([]*education.GroupSubstitution, error)
	GetActiveGroupSubstitutions(ctx context.Context, groupID int64, date time.Time) ([]*education.GroupSubstitution, error)
	GetStaffSubstitutions(ctx context.Context, staffID int64, asRegular bool) ([]*education.GroupSubstitution, error)
	CheckSubstitutionConflicts(ctx context.Context, staffID int64, startDate, endDate time.Time) ([]*education.GroupSubstitution, error)
}
