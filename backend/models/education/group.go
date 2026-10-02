package education

import (
	"errors"
	"strings"
	"time"
)

// Group represents an educational group/class (education.groups).
type Group struct {
	Model
	TenantModel
	Name   string `bun:"name,notnull" json:"name"`
	RoomID *int64 `bun:"room_id" json:"room_id,omitempty"`

	// Room is the group's room as the Facilities owner resolves it; the
	// room-enriched reads fill it, the others leave it nil.
	Room *GroupRoom `bun:"-" json:"room,omitempty"`
	// Teachers are linked through the GroupTeacher model
	// Students will be a relationship from the Student model
}

// GroupRoom is the Facilities room a group is assigned to, as the
// room-enriched group reads carry it (#2665). facilities.rooms belongs to
// that owner; School Structure only keeps this projection.
type GroupRoom struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
	Building  string    `json:"building,omitempty"`
	Floor     *int      `json:"floor,omitempty"`
	Capacity  *int      `json:"capacity,omitempty"`
	Category  *string   `json:"category,omitempty"`
	Color     *string   `json:"color,omitempty"`
}

// GroupListQuery is the bounded read shape for the group overview. It exposes
// only the filters and ordering that overview callers use.
type GroupListQuery struct {
	Name         string
	NameContains string
	RoomID       *int64
	Limit        int
	Offset       int
	SortByName   bool
	Descending   bool
}

// Validate ensures group data is valid
func (g *Group) Validate() error {
	if g.Name == "" {
		return errors.New("group name is required")
	}

	// Trim spaces from name
	g.Name = strings.TrimSpace(g.Name)

	return nil
}

// HasRoom checks if the group has a room assigned
func (g *Group) HasRoom() bool {
	return g.RoomID != nil && *g.RoomID > 0
}

// StaffGroupID pairs a staff member with one education group they supervise:
// callers need the IDs, never the group rows themselves.
type StaffGroupID struct {
	StaffID int64 `bun:"staff_id"`
	GroupID int64 `bun:"group_id"`
}
