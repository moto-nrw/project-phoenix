package studentpresence

// The changes Student Presence announces to the live views of the staff
// portals: a child's check-in or check-out and the start or end of a room
// session. Student Presence names them in its own vocabulary; the root shapes
// them into the realtime transport's events.

// LiveAttendanceDetail carries the timetable attendance fields a visit
// announcement may carry (WP-B10). Substatus and Note stay absent when nil.
type LiveAttendanceDetail struct {
	Status    string
	Substatus *string
	Note      *string
}

// LiveVisitChange describes one child's room check-in or check-out. An empty
// ActiveGroupID marks a roomless attendance change. EducationGroupID scopes
// the educational group topic and the client-side invalidation; Source names
// the emitting flow and is omitted when empty.
type LiveVisitChange struct {
	ActiveGroupID    string
	StudentID        string
	EducationGroupID *int64
	Source           string
	Attendance       *LiveAttendanceDetail
}

// LiveSessionChange describes a session start or end for the room roster.
type LiveSessionChange struct {
	ActiveGroupID string
	ActivityName  string
	RoomID        string
	RoomName      string
	SupervisorIDs []string
}
