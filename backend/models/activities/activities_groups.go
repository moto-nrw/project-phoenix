package activities

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// Group represents an activity group
type Group struct {
	base.Model
	Name            string    `bun:"name,notnull" json:"name"`
	MaxParticipants int       `bun:"max_participants,notnull" json:"max_participants"`
	IsOpen          bool      `bun:"is_open,notnull,default:false" json:"is_open"`
	CategoryID      int64     `bun:"category_id,notnull" json:"category_id"`
	SupervisorID    int64     `bun:"supervisor_id" json:"supervisor_id,omitempty"` // TODO: double check
	PlanedRoomID    *int64    `bun:"planed_room_id" json:"planed_room_id,omitempty"`
	CreatedAt       time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt       time.Time `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`

	// Relations (not stored in the database table)
	Category     *Category        `bun:"-" json:"category,omitempty"`
	Supervisor   *auth.Account    `bun:"-" json:"supervisor,omitempty"`
	PlanedRoom   *facilities.Room `bun:"-" json:"planed_room,omitempty"`
	Participants []*auth.Account  `bun:"-" json:"participants,omitempty"`
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

	// Trim spaces from name
	g.Name = strings.TrimSpace(g.Name)

	// Validate max participants is positive
	if g.MaxParticipants <= 0 {
		return errors.New("max participants must be positive")
	}

	// Validate that category ID is set
	if g.CategoryID == 0 {
		return errors.New("category is required")
	}

	return nil
}

// IsFull checks if the group has reached its maximum number of participants
func (g *Group) IsFull() bool {
	if g.Participants == nil {
		return false
	}
	return len(g.Participants) >= g.MaxParticipants
}

// HasRoom checks if the group has a planned room assigned
func (g *Group) HasRoom() bool {
	return g.PlanedRoomID != nil && *g.PlanedRoomID > 0
}

// AddParticipant adds a participant to the group if it's not full
func (g *Group) AddParticipant(participant *auth.Account) error {
	// Initialize participants slice if nil
	if g.Participants == nil {
		g.Participants = make([]*auth.Account, 0)
	}

	// Check if group is full
	if g.IsFull() {
		return errors.New("group is full")
	}

	// Check if participant already exists
	for _, p := range g.Participants {
		if p.ID == participant.ID {
			return errors.New("participant already in group")
		}
	}

	g.Participants = append(g.Participants, participant)
	return nil
}

// RemoveParticipant removes a participant from the group
func (g *Group) RemoveParticipant(participantID int64) bool {
	if g.Participants == nil {
		return false
	}

	for i, p := range g.Participants {
		if p.ID == participantID {
			// Remove participant by slicing
			g.Participants = append(g.Participants[:i], g.Participants[i+1:]...)
			return true
		}
	}

	return false
}

// HasSupervisor checks if the group has a supervisor
func (g *Group) HasSupervisor() bool {
	return g.SupervisorID != 0
}

// SetSupervisor sets the supervisor for the group
func (g *Group) SetSupervisor(supervisor *auth.Account) {
	g.SupervisorID = supervisor.ID
	g.Supervisor = supervisor
}

// SetPlanedRoom sets the planned room for the group
func (g *Group) SetPlanedRoom(room *facilities.Room) error {
	// Check if the room has enough capacity
	if room != nil && !room.IsAvailable(g.MaxParticipants) {
		return errors.New("room capacity is not sufficient for the group")
	}

	if room == nil {
		g.PlanedRoomID = nil
	} else {
		g.PlanedRoomID = &room.ID
	}
	g.PlanedRoom = room
	return nil
}
