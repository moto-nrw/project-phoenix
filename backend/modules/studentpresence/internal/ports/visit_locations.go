package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type VisitLocation struct {
	studentpresence.Visit

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

type VisitLocationStore interface {
	ListVisitLocations(context.Context, studentpresence.VisitLocationFilter) ([]VisitLocation, Stats, error)
	CountOpenVisitsInGroup(context.Context, int64) (int, Stats, error)
	CountOpenVisitsInGroupWithRevision(context.Context, int64) (int, int64, Stats, error)
	CountOpenVisitsInRoom(context.Context, int64) (int, Stats, error)
	ListOpenVisitRooms(context.Context, int64) ([]studentpresence.OpenVisitRoom, Stats, error)
}
