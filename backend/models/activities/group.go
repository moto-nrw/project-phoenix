package activities

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Group type constants
const (
	GroupTypeActivity = "activity"
	GroupTypeCare     = "care"
	GroupTypeExternal = "external"
)

// Group represents an activity group
type Group struct {
	base.Model `bun:"schema:activities,table:groups"`
	base.TenantModel
	Name             string     `bun:"name,notnull" json:"name"`
	MaxParticipants  int        `bun:"max_participants,notnull" json:"max_participants"`
	IsOpen           bool       `bun:"is_open,notnull,default:false" json:"is_open"`
	CategoryID       int64      `bun:"category_id,notnull" json:"category_id"`
	PlannedRoomID    *int64     `bun:"planned_room_id" json:"planned_room_id,omitempty"`
	CreatedBy        *int64     `bun:"created_by" json:"created_by"`
	Type             string     `bun:"type,notnull,default:'activity'" json:"type"`
	EducationGroupID *int64     `bun:"education_group_id" json:"education_group_id,omitempty"`
	IsTemplate       bool       `bun:"is_template,notnull,default:false" json:"is_template"`
	IsSystem         bool       `bun:"is_system,notnull,default:false" json:"is_system"`
	ArchivedAt       *time.Time `bun:"archived_at" json:"archived_at,omitempty"`

	// Relations - populated when using the ORM's relations
	Category       *Category            `bun:"rel:belongs-to,join:category_id=id" json:"category,omitempty"`
	PlannedRoom    *facilities.Room     `bun:"rel:belongs-to,join:planned_room_id=id" json:"planned_room,omitempty"`
	CreatedByStaff *users.Staff         `bun:"rel:belongs-to,join:created_by=id" json:"created_by_staff,omitempty"`
	Supervisors    []*SupervisorPlanned `bun:"rel:has-many,join:id=group_id" json:"supervisors,omitempty"`
	Schedules      []*Schedule          `bun:"rel:has-many,join:id=activity_group_id" json:"schedules,omitempty"`
	Enrollments    []*StudentEnrollment `bun:"rel:has-many,join:id=activity_group_id" json:"enrollments,omitempty"`
}

func (g *Group) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`activities.groups AS "group"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`activities.groups AS "group"`)
	}
	return nil
}

// GetID returns the entity's ID
func (g *Group) GetID() interface{} {
	return g.ID
}

// GetCreatedAt returns the creation timestamp
func (g *Group) GetCreatedAt() time.Time {
	return g.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (g *Group) GetUpdatedAt() time.Time {
	return g.UpdatedAt
}

// TableName returns the database table name
func (g *Group) TableName() string {
	return "activities.groups"
}

// Validate ensures group data is valid
func (g *Group) Validate() error {
	if g.Name == "" {
		return errors.New("group name is required")
	}

	if g.MaxParticipants <= 0 {
		return errors.New("max participants must be greater than zero")
	}

	if g.CategoryID <= 0 {
		return errors.New("category ID is required")
	}

	return nil
}

// IsOwnedBy checks if the group was created by the given staff member
func (g *Group) IsOwnedBy(staffID int64) bool {
	return g.CreatedBy != nil && *g.CreatedBy == staffID
}

// IsSupervisedBy checks if the given staff member is a supervisor of this group
func (g *Group) IsSupervisedBy(staffID int64) bool {
	for _, supervisor := range g.Supervisors {
		if supervisor != nil && supervisor.StaffID == staffID {
			return true
		}
	}
	return false
}

// HasAvailableSpots checks if the group has available spots based on current enrollment count
func (g *Group) HasAvailableSpots(currentEnrollmentCount int) bool {
	return g.MaxParticipants > currentEnrollmentCount
}
