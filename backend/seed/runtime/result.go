package runtime

import (
	"github.com/moto-nrw/project-phoenix/models/active"
)

// Result contains all created runtime state
type Result struct {
	// Active sessions
	ActiveGroups   []*active.Group
	CombinedGroups []*active.CombinedGroup
	GroupMappings  []GroupMapping

	// Visit tracking
	Visits     []*active.Visit
	Attendance []*active.Attendance

	// Supervisor assignments
	Supervisors     []*active.GroupSupervisor
	SupervisorCount int

	// Statistics
	StudentsCheckedIn  int
	StudentsInRooms    map[int64]int // room_id -> count
	ActiveGroupsByRoom map[int64]*active.Group
}

// GroupMapping represents active.group_mappings join table
type GroupMapping struct {
	CombinedGroupID int64
	ActiveGroupID   int64
}

// NewResult creates a new runtime result
func NewResult() *Result {
	return &Result{
		StudentsInRooms:    make(map[int64]int),
		ActiveGroupsByRoom: make(map[int64]*active.Group),
	}
}
