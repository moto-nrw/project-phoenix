// Package models contains application specific entities.
package models

import (
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"
)

// RoomOccupancy represents the current occupation of a room by an activity group or student group
type RoomOccupancy struct {
	ID          int64                    `json:"id" bun:"id,pk,autoincrement"`
	DeviceID    string                   `json:"device_id" bun:"device_id,notnull,unique"`
	RoomID      int64                    `json:"room_id" bun:"room_id,notnull"`
	Room        *Room                    `json:"room,omitempty" bun:"rel:belongs-to,join:room_id=id"`
	AgID        *int64                   `json:"ag_id,omitempty" bun:"ag_id"`
	Ag          *Ag                      `json:"ag,omitempty" bun:"rel:belongs-to,join:ag_id=id"`
	GroupID     *int64                   `json:"group_id,omitempty" bun:"group_id"`
	Group       *Group                   `json:"group,omitempty" bun:"rel:belongs-to,join:group_id=id"`
	TimespanID  int64                    `json:"timespan_id" bun:"timespan_id,notnull"`
	Timespan    *Timespan                `json:"timespan,omitempty" bun:"rel:belongs-to,join:timespan_id=id"`
	Supervisors []*PedagogicalSpecialist `json:"supervisors,omitempty" bun:"m2m:room_occupancy_supervisors,join:RoomOccupancy=Supervisor"`
	CreatedAt   time.Time                `json:"created_at" bun:"created_at,notnull"`
	ModifiedAt  time.Time                `json:"modified_at" bun:"modified_at,notnull"`

	bun.BaseModel `bun:"table:room_occupancies"`
}

// BeforeInsert hook executed before database insert operation
func (ro *RoomOccupancy) BeforeInsert(db *bun.DB) error {
	now := time.Now()
	ro.CreatedAt = now
	ro.ModifiedAt = now
	return ro.Validate()
}

// BeforeUpdate hook executed before database update operation
func (ro *RoomOccupancy) BeforeUpdate(db *bun.DB) error {
	ro.ModifiedAt = time.Now()
	return ro.Validate()
}

// Validate validates RoomOccupancy struct and returns validation errors
func (ro *RoomOccupancy) Validate() error {
	// Basic field validation
	err := validation.ValidateStruct(ro,
		validation.Field(&ro.DeviceID, validation.Required),
		validation.Field(&ro.RoomID, validation.Required),
		validation.Field(&ro.TimespanID, validation.Required),
	)
	if err != nil {
		return err
	}

	// Custom validation to ensure at least one of AgID or GroupID is set
	if ro.AgID == nil && ro.GroupID == nil {
		return errors.New("at least one of AgID or GroupID must be set")
	}

	return nil
}

// RoomOccupancySupervisor represents the junction table between RoomOccupancy and PedagogicalSpecialist
type RoomOccupancySupervisor struct {
	ID              int64                  `json:"id" bun:"id,pk,autoincrement"`
	RoomOccupancyID int64                  `json:"room_occupancy_id" bun:"room_occupancy_id,notnull"`
	RoomOccupancy   *RoomOccupancy         `json:"-" bun:"rel:belongs-to,join:room_occupancy_id=id"`
	SupervisorID    int64                  `json:"supervisor_id" bun:"supervisor_id,notnull"`
	Supervisor      *PedagogicalSpecialist `json:"-" bun:"rel:belongs-to,join:supervisor_id=id"`
	CreatedAt       time.Time              `json:"created_at" bun:"created_at,notnull"`

	bun.BaseModel `bun:"table:room_occupancy_supervisors"`
}

// BeforeInsert hook executed before database insert operation
func (ros *RoomOccupancySupervisor) BeforeInsert(db *bun.DB) error {
	ros.CreatedAt = time.Now()
	return nil
}

// RoomOccupancyDetail is a simplified view of room occupancy data
// used for API responses and certain queries
type RoomOccupancyDetail struct {
	RoomID     int64 `json:"room_id"`
	TimespanID int64 `json:"timespan_id"`
}

// IsActive checks if the room occupancy is currently active
func (ro *RoomOccupancy) IsActive() bool {
	if ro.Timespan == nil {
		return false
	}
	return ro.Timespan.IsActive()
}

// String returns a string representation of the room occupancy
func (ro *RoomOccupancy) String() string {
	roomName := "Unknown room"
	if ro.Room != nil {
		roomName = ro.Room.RoomName
	}

	if ro.Ag != nil {
		return roomName + " - " + ro.Ag.Name
	} else if ro.Group != nil {
		return roomName + " - " + ro.Group.Name
	}

	return roomName + " - Unspecified activity"
}

// AddSupervisor adds a supervisor to the room occupancy
func (ro *RoomOccupancy) AddSupervisor(db *bun.DB, supervisorID int64) error {
	junction := &RoomOccupancySupervisor{
		RoomOccupancyID: ro.ID,
		SupervisorID:    supervisorID,
	}

	_, err := db.NewInsert().Model(junction).Exec(nil)
	return err
}

// RemoveSupervisor removes a supervisor from the room occupancy
func (ro *RoomOccupancy) RemoveSupervisor(db *bun.DB, supervisorID int64) error {
	_, err := db.NewDelete().
		Model((*RoomOccupancySupervisor)(nil)).
		Where("room_occupancy_id = ? AND supervisor_id = ?", ro.ID, supervisorID).
		Exec(nil)
	return err
}
