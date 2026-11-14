package timetable

import "context"

// RoomReferences counts the Timetable rows that still point at one room. The
// Facilities room removal shows them before a room is deleted or archived
// (#6845): live rows keep a room planned and block the removal, history rows
// only decide between deleting and archiving.
type RoomReferences struct {
	// ActivityGroups counts the activities and templates that plan the room
	// and are not archived.
	ActivityGroups int
	// UpcomingBlocks counts planned or running blocks in the room from the
	// given day on.
	UpcomingBlocks int
	// PastBlocks counts every other block in the room: earlier days and
	// cancelled or completed ones.
	PastBlocks int
	// ArchivedActivityGroups counts archived activities and templates that
	// still name the room.
	ArchivedActivityGroups int
}

// RoomReferenceQuery counts the Timetable rows that reference one room.
type RoomReferenceQuery interface {
	CountRoomReferences(ctx context.Context, roomID int64) (RoomReferences, error)
}
