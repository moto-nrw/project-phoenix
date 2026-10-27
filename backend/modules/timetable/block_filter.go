package timetable

import (
	"context"
	"slices"
)

// BlockFilter narrows the Betreuungsplan to one OGS group, one staff member
// (#4766) and/or one room (#4768). A nil field does not filter; several set
// means all must match. The filter only narrows what the caller may already
// see.
type BlockFilter struct {
	// EducationGroupID keeps the blocks of the group, as BlockGroupQuery
	// reports them.
	EducationGroupID *int64
	// StaffID keeps the blocks the staff member is assigned to, as regular
	// staff or as substitute, absent rows included: the block still shows
	// the person.
	StaffID *int64
	// RoomID keeps the blocks planned in the room: the block's own room, the
	// one the planner and the room's plan show.
	RoomID *int64
}

// Active reports whether the filter narrows anything.
func (f BlockFilter) Active() bool {
	return f.EducationGroupID != nil || f.StaffID != nil || f.RoomID != nil
}

// Matches reports whether a block in the given room, with the given staff
// rows and OGS groups, passes the filter.
func (f BlockFilter) Matches(roomID int64, staff []InstanceStaff, educationGroupIDs []int64) bool {
	if f.RoomID != nil && roomID != *f.RoomID {
		return false
	}
	if f.EducationGroupID != nil && !slices.Contains(educationGroupIDs, *f.EducationGroupID) {
		return false
	}
	if f.StaffID != nil && !slices.ContainsFunc(staff, func(row InstanceStaff) bool { return row.StaffID == *f.StaffID }) {
		return false
	}
	return true
}

// BlockGroupQuery names the OGS groups of planned blocks (#4766).
type BlockGroupQuery interface {
	// BlockEducationGroups maps each block to its OGS groups, ascending: the
	// group its template targets, the groups its template takes as
	// participant source and the current group of every child planned on
	// it. Blocks without a group are absent.
	BlockEducationGroups(ctx context.Context, instanceIDs []int64) (map[int64][]int64, error)
}
