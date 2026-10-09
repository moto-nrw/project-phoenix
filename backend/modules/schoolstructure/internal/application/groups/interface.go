// Package education provides services for managing educational groups and related entities
package groups

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/education"
)

// Service defines operations for managing educational groups and their relationships
type Service interface {
	// Group operations
	CreateGroup(ctx context.Context, group *education.Group) error
	UpdateGroup(ctx context.Context, group *education.Group) error
	DeleteGroup(ctx context.Context, id int64) error

	// Group-Teacher operations
	RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error
	UpdateGroupTeachers(ctx context.Context, groupID int64, teacherIDs []int64) error
	GetGroupTeachers(ctx context.Context, groupID int64) ([]*Teacher, error)
	GetTeachersForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*Teacher, error)
	GetTeacherGroups(ctx context.Context, teacherID int64) ([]*education.Group, error)

	// Class-Teacher operations (#1772): staff-to-school-class assignments
	// that scope the Lehrkraft day view. Classes are free-text strings
	// matched via schoolclass.Normalize; there is no class entity. changedBy
	// is the authenticated account ID for the audit trail.
	GetStaffSchoolClasses(ctx context.Context, staffID int64) ([]string, error)
	SetStaffSchoolClasses(ctx context.Context, staffID int64, classes []string, changedBy int64) error
}
