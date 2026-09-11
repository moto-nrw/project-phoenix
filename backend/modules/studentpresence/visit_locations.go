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

type VisitLocationQuery interface {
	ListVisitLocations(context.Context, VisitLocationFilter) ([]VisitLocation, error)
	CountOpenVisitsInGroup(context.Context, int64) (int, error)
	CountOpenVisitsInRoom(context.Context, int64) (int, error)
	ListOpenVisitRooms(context.Context, int64) ([]OpenVisitRoom, error)
}

// OpenVisitRoom is distinct per student and room in the whole-school snapshot
// (room filter zero). A specific room retains one row per open visit.
type OpenVisitRoom struct{ StudentID, RoomID int64 }

func (m *Module) ListVisitLocations(ctx context.Context, filter VisitLocationFilter) ([]VisitLocation, error) {
	return m.engine.ListVisitLocations(ctx, filter)
}
func (m *Module) CountOpenVisitsInGroup(ctx context.Context, id int64) (int, error) {
	return m.engine.CountOpenVisitsInGroup(ctx, id)
}
func (m *Module) CountOpenVisitsInRoom(ctx context.Context, id int64) (int, error) {
	return m.engine.CountOpenVisitsInRoom(ctx, id)
}
func (m *Module) ListOpenVisitRooms(ctx context.Context, roomID int64) ([]OpenVisitRoom, error) {
	return m.engine.ListOpenVisitRooms(ctx, roomID)
}
