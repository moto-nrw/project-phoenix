package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrNotFound = errors.New("group not found")

// Group is an education group (Klasse/Gruppe) of education.groups.
type Group struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TenantID  int64     `json:"tenant_id"`
	Name      string    `json:"name"`
	RoomID    *int64    `json:"room_id,omitempty"`

	// Room is the group's room as the Facilities owner resolves it; the
	// room-enriched reads fill it, the others leave it nil.
	Room *GroupRoom `json:"room,omitempty"`
}

// Validate requires a name and trims it.
func (g *Group) Validate() error {
	if g.Name == "" {
		return errors.New("group name is required")
	}
	g.Name = strings.TrimSpace(g.Name)
	return nil
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

// StaffGroupID pairs a staff member with one education group they supervise:
// callers need the IDs, never the group rows themselves.
type StaffGroupID struct {
	StaffID int64
	GroupID int64
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
