package models

import (
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"
)

// RoomOccupancy represents the current occupation of a room by an activity group or student group
type RoomOccupancy struct {
	ID         int64     `json:"id" bun:"id,pk,autoincrement"`
	DeviceID   string    `json:"device_id" bun:"device_id,notnull,unique"`
	RoomID     int64     `json:"room_id" bun:"room_id,notnull"`
	AgID       *int64    `json:"ag_id,omitempty" bun:"ag_id"`
	GroupID    *int64    `json:"group_id,omitempty" bun:"group_id"`
	TimespanID int64     `json:"timespan_id" bun:"timespan_id,notnull"`
	CreatedAt  time.Time `json:"created_at" bun:"created_at,notnull"`
	ModifiedAt time.Time `json:"modified_at" bun:"modified_at,notnull"`

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
	ID              int64     `json:"id" bun:"id,pk,autoincrement"`
	RoomOccupancyID int64     `json:"room_occupancy_id" bun:"room_occupancy_id,notnull"`
	SupervisorID    int64     `json:"supervisor_id" bun:"supervisor_id,notnull"`
	CreatedAt       time.Time `json:"created_at" bun:"created_at,notnull"`

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
