package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// TeacherGroupID pairs a teacher profile with one education group they are
// assigned to. The supervision rule resolves the teacher to a staff member.
type TeacherGroupID struct {
	TeacherID int64
	GroupID   int64
}

// GroupMembershipPairs are the raw supervision references of a set of
// education groups: the teacher assignments, unresolved, plus the groups and
// day the substitutions must be read for. The supervision rule turns them
// into (staff, group) pairs through School Membership and Workforce, dropping
// references to offboarded teachers and staff (#2667, #2688).
type GroupMembershipPairs struct {
	// Assigned pairs a group with the teacher assigned to it.
	Assigned []TeacherGroupID
	// GroupIDs and On name the substitutions to add: whoever substitutes in
	// one of these groups on that day. education.group_substitution belongs
	// to Workforce, so School Structure does not read it.
	GroupIDs []int64
	On       calendar.Date
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
