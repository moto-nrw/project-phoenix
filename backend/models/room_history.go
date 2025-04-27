// Package models contains application specific entities.
package models

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"
)

// RoomHistory records historical usage of rooms for reporting and analysis
type RoomHistory struct {
	ID             int64                  `json:"id" bun:"id,pk,autoincrement"`
	RoomID         int64                  `json:"room_id" bun:"room_id,notnull"`
	Room           *Room                  `json:"room,omitempty" bun:"rel:belongs-to,join:room_id=id"`
	AgName         string                 `json:"ag_name" bun:"ag_name"`
	Day            time.Time              `json:"day" bun:"day,notnull"`
	TimespanID     int64                  `json:"timespan_id" bun:"timespan_id,notnull"`
	Timespan       *Timespan              `json:"timespan,omitempty" bun:"rel:belongs-to,join:timespan_id=id"`
	AgCategoryID   *int64                 `json:"ag_category_id,omitempty" bun:"ag_category_id"`
	AgCategory     *AgCategory            `json:"ag_category,omitempty" bun:"rel:belongs-to,join:ag_category_id=id"`
	SupervisorID   int64                  `json:"supervisor_id" bun:"supervisor_id,notnull"`
	Supervisor     *PedagogicalSpecialist `json:"supervisor,omitempty" bun:"rel:belongs-to,join:supervisor_id=id"`
	MaxParticipant int                    `json:"max_participant" bun:"max_participant,default:0"`
	GroupID        *int64                 `json:"group_id,omitempty" bun:"group_id"`
	Group          *Group                 `json:"group,omitempty" bun:"rel:belongs-to,join:group_id=id"`
	CreatedAt      time.Time              `json:"created_at" bun:"created_at,notnull"`

	bun.BaseModel `bun:"table:room_history"`
}

// BeforeInsert hook executed before database insert operation
func (rh *RoomHistory) BeforeInsert(db *bun.DB) error {
	rh.CreatedAt = time.Now()
	return rh.Validate()
}

// Validate validates RoomHistory struct and returns validation errors
func (rh *RoomHistory) Validate() error {
	return validation.ValidateStruct(rh,
		validation.Field(&rh.RoomID, validation.Required),
		validation.Field(&rh.Day, validation.Required),
		validation.Field(&rh.TimespanID, validation.Required),
		validation.Field(&rh.SupervisorID, validation.Required),
	)
}

// CreateFromRoomOccupancy creates a RoomHistory entry from a RoomOccupancy
func CreateFromRoomOccupancy(occupancy *RoomOccupancy, timespan *Timespan, room *Room, day time.Time) (*RoomHistory, error) {
	history := &RoomHistory{
		RoomID:     occupancy.RoomID,
		Day:        day,
		TimespanID: occupancy.TimespanID,
	}

	// Add group or AG information based on what's available
	if occupancy.GroupID != nil && occupancy.Group != nil {
		history.GroupID = occupancy.GroupID
		history.AgName = occupancy.Group.Name
	}

	if occupancy.AgID != nil && occupancy.Ag != nil {
		history.AgName = occupancy.Ag.Name
		history.MaxParticipant = occupancy.Ag.MaxParticipant
		history.AgCategoryID = &occupancy.Ag.AgCategoryID
	}

	// Add the first supervisor if available
	if len(occupancy.Supervisors) > 0 {
		history.SupervisorID = occupancy.Supervisors[0].ID
	}

	return history, nil
}
