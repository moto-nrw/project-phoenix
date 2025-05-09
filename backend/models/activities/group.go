package activities

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// Group represents an activity group
type Group struct {
	base.Model
	Name            string `bun:"name,notnull" json:"name"`
	MaxParticipants int    `bun:"max_participants,notnull" json:"max_participants"`
	IsOpen          bool   `bun:"is_open,notnull,default:false" json:"is_open"`
	CategoryID      int64  `bun:"category_id,notnull" json:"category_id"`
	PlannedRoomID   *int64 `bun:"planned_room_id" json:"planned_room_id,omitempty"`

	// Relations - populated when using the ORM's relations
	Category    *Category            `bun:"rel:belongs-to,join:category_id=id" json:"category,omitempty"`
	PlannedRoom *facilities.Room     `bun:"rel:belongs-to,join:planned_room_id=id" json:"planned_room,omitempty"`
	Supervisors []*SupervisorPlanned `bun:"rel:has-many,join:id=group_id" json:"supervisors,omitempty"`
	Schedules   []*Schedule          `bun:"rel:has-many,join:id=activity_group_id" json:"schedules,omitempty"`
	Enrollments []*StudentEnrollment `bun:"rel:has-many,join:id=activity_group_id" json:"enrollments,omitempty"`
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

// HasAvailableSpots checks if the group has available spots based on current enrollment count
func (g *Group) HasAvailableSpots(currentEnrollmentCount int) bool {
	return g.MaxParticipants > currentEnrollmentCount
}

// CanJoin determines if a student can join this group
func (g *Group) CanJoin(currentEnrollmentCount int) bool {
	return g.IsOpen && g.HasAvailableSpots(currentEnrollmentCount)
}
