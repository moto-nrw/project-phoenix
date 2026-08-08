// Package education provides services for managing educational groups and related entities
package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Service defines operations for managing educational groups and their relationships
type Service interface {
	// Group operations
	GetGroup(ctx context.Context, id int64) (*education.Group, error)
	GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error)
	CreateGroup(ctx context.Context, group *education.Group) error
	UpdateGroup(ctx context.Context, group *education.Group) error
	DeleteGroup(ctx context.Context, id int64) error
	ListGroups(ctx context.Context, options *base.QueryOptions) ([]*education.Group, error)
	CountGroups(ctx context.Context, options *base.QueryOptions) (int, error)
	FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error)
	// GetGroupsWithRoomsByIDs bulk-loads groups with their room relation in
	// one query — use instead of per-group FindGroupWithRoom loops.
	GetGroupsWithRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error)

	// Group-Teacher operations
	RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error
	UpdateGroupTeachers(ctx context.Context, groupID int64, teacherIDs []int64) error
	GetGroupTeachers(ctx context.Context, groupID int64) ([]*users.Teacher, error)
	GetTeachersForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*users.Teacher, error)
	GetTeacherGroups(ctx context.Context, teacherID int64) ([]*education.Group, error)

	// Class-Teacher operations (#1772): staff-to-school-class assignments
	// that scope the Lehrkraft day view. Classes are free-text strings
	// matched via schoolclass.Normalize; there is no class entity. changedBy
	// is the authenticated account ID for the audit trail.
	GetStaffSchoolClasses(ctx context.Context, staffID int64) ([]string, error)
	SetStaffSchoolClasses(ctx context.Context, staffID int64, classes []string, changedBy int64) error

	// Substitution operations
	CreateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error

	// CreateGroupTransfer persists a group-transfer substitution WITHOUT the
	// overlap conflict check: group transfers deliberately allow staff to
	// hold multiple groups at once (issue #584: moved verbatim from
	// api/groups, including that documented bypass).
	CreateGroupTransfer(ctx context.Context, substitution *education.GroupSubstitution) error
	UpdateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error
	DeleteSubstitution(ctx context.Context, id int64) error
	GetSubstitution(ctx context.Context, id int64) (*education.GroupSubstitution, error)
	ListSubstitutions(ctx context.Context, options *base.QueryOptions) ([]*education.GroupSubstitution, error)
	GetActiveSubstitutions(ctx context.Context, date timezone.Date) ([]*education.GroupSubstitution, error)
	GetActiveGroupSubstitutions(ctx context.Context, groupID int64, date timezone.Date) ([]*education.GroupSubstitution, error)
	GetStaffSubstitutions(ctx context.Context, staffID int64, asRegular bool) ([]*education.GroupSubstitution, error)
	CheckSubstitutionConflicts(ctx context.Context, staffID int64, startDate, endDate timezone.Date) ([]*education.GroupSubstitution, error)
}
