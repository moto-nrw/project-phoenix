package ports

import (
	"context"
	"time"
)

type VisitLocation struct {
	Visit
	GroupID        *int64
	RoomID         int64
	GroupCreatedAt time.Time
	GroupUpdatedAt time.Time
	StartTime      time.Time
	LastActivity   time.Time
	EndTime        *time.Time
	TemplateID     *int64
	DeviceID       *int64
	TimeoutMinutes int
}

type VisitLocationFilter struct {
	VisitFilter
	RunningGroupsOnly bool
	LatestPerStudent  bool
}

type OpenVisitRoom struct {
	StudentID int64
	RoomID    int64
}

type VisitLocationStore interface {
	ListVisitLocations(context.Context, VisitLocationFilter) ([]VisitLocation, Stats, error)
	CountOpenVisitsInGroup(context.Context, int64) (int, Stats, error)
	CountOpenVisitsInRoom(context.Context, int64) (int, Stats, error)
	ListOpenVisitRooms(context.Context, int64) ([]OpenVisitRoom, Stats, error)
}
