package studentpresence

import (
	"context"
	"time"
)

// VisitGroup describes the session that locates a visit. Names remain owned
// by their directory capabilities and are not copied into presence storage.
type VisitGroup struct {
	ID, RoomID                                    int64
	CreatedAt, UpdatedAt, StartTime, LastActivity time.Time
	EndTime                                       *time.Time
	TemplateID, DeviceID                          *int64
	TimeoutMinutes                                int
}

type VisitLocation struct {
	Visit Visit
	Group *VisitGroup
}

type VisitLocationFilter struct {
	VisitFilter
	RunningGroupsOnly bool
	LatestPerStudent  bool
}

// CurrentRoomVisitsFilter selects each student's latest open visit inside a
// running session: the visit whose session room locates the student now.
func CurrentRoomVisitsFilter(studentIDs []int64) VisitLocationFilter {
	return VisitLocationFilter{
		VisitFilter:       VisitFilter{StudentIDs: studentIDs, OpenOnly: true},
		RunningGroupsOnly: true,
		LatestPerStudent:  true,
	}
}

type VisitLocationQuery interface {
	ListVisitLocations(context.Context, VisitLocationFilter) ([]VisitLocation, error)
	CountOpenVisitsInGroup(context.Context, int64) (int, error)
	// CountOpenVisitsInGroupWithRevision returns the open visit count of a
	// session and its visit revision from one snapshot (#5128). Every
	// committed change of the count raises the revision; one revision of a
	// session always names the same count.
	CountOpenVisitsInGroupWithRevision(context.Context, int64) (int, int64, error)
	CountOpenVisitsInRoom(context.Context, int64) (int, error)
	// CountOpenVisitsInRoomWithRevision returns the open visit count of a
	// room's running sessions and the room's visit revision from one
	// snapshot (#5205). Every committed change of the count raises the
	// revision, also a session's end, reopening, deletion or room change.
	// Room revisions compare only within one room, never with a session's.
	CountOpenVisitsInRoomWithRevision(context.Context, int64) (int, int64, error)
	ListOpenVisitRooms(context.Context, int64) ([]OpenVisitRoom, error)
}

// OpenVisitRoom is distinct per student and room in the whole-school snapshot
// (room filter zero). A specific room retains one row per open visit.
type OpenVisitRoom struct{ StudentID, RoomID int64 }
