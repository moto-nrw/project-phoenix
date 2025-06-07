package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Group represents an active group session in a room
type Group struct {
	base.Model `bun:"schema:active,table:groups"`
	StartTime  time.Time  `bun:"start_time,notnull" json:"start_time"`
	EndTime    *time.Time `bun:"end_time" json:"end_time,omitempty"`
	GroupID    int64      `bun:"group_id,notnull" json:"group_id"`
	DeviceID   *int64     `bun:"device_id" json:"device_id,omitempty"`      // Optional for RFID system
	RoomID     int64      `bun:"room_id,notnull" json:"room_id"`

	// Relations - these would be populated when using the ORM's relations
	ActualGroup *activities.Group  `bun:"rel:belongs-to,join:group_id=id" json:"actual_group,omitempty"`
	Device      *iot.Device        `bun:"rel:belongs-to,join:device_id=id" json:"device,omitempty"`
	Room        *facilities.Room   `bun:"rel:belongs-to,join:room_id=id" json:"room,omitempty"`
	Visits      []*Visit           `bun:"rel:has-many,join:id=active_group_id" json:"visits,omitempty"`
	Supervisors []*GroupSupervisor `bun:"rel:has-many,join:id=group_id" json:"supervisors,omitempty"`
}

// BeforeAppendModel is commented out to let the repository control the table expression
// func (g *Group) BeforeAppendModel(query any) error {
// 	switch q := query.(type) {
// 	case *bun.SelectQuery:
// 		q.ModelTableExpr("active.groups")
// 	case *bun.InsertQuery:
// 		q.ModelTableExpr("active.groups")
// 	case *bun.UpdateQuery:
// 		q.ModelTableExpr("active.groups")
// 	case *bun.DeleteQuery:
// 		q.ModelTableExpr("active.groups")
// 	}
// 	return nil
// }

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
	return "active.groups"
}

// Validate ensures active group data is valid
func (g *Group) Validate() error {
	if g.StartTime.IsZero() {
		return errors.New("start time is required")
	}

	if g.EndTime != nil && g.StartTime.After(*g.EndTime) {
		return errors.New("start time must be before end time")
	}

	if g.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	// DeviceID is now optional for RFID system
	// No validation needed for DeviceID

	if g.RoomID <= 0 {
		return errors.New("room ID is required")
	}

	return nil
}

// IsActive returns whether this active group session is currently active
func (g *Group) IsActive() bool {
	return g.EndTime == nil
}

// EndSession sets the end time to the current time
func (g *Group) EndSession() {
	now := time.Now()
	g.EndTime = &now
}

// SetEndTime explicitly sets the end time
func (g *Group) SetEndTime(endTime time.Time) error {
	if g.StartTime.After(endTime) {
		return errors.New("end time cannot be before start time")
	}
	g.EndTime = &endTime
	return nil
}

// GetDuration returns the duration of the active group session
func (g *Group) GetDuration() time.Duration {
	if g.EndTime == nil {
		return time.Since(g.StartTime)
	}
	return g.EndTime.Sub(g.StartTime)
}
