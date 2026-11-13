package ports

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
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

// VisitAdmission is read after locking the target session and the child.
type VisitAdmission struct {
	HasOpenVisit   bool
	GroupOccupancy int
}

type VisitLocationStore interface {
	ListVisitLocations(context.Context, studentpresence.VisitLocationFilter) ([]VisitLocation, Stats, error)
	CountOpenVisitsInGroup(context.Context, int64) (int, Stats, error)
	CountOpenVisitsInGroupWithRevision(context.Context, int64) (int, int64, Stats, error)
	CountOpenVisitsInRoom(context.Context, int64) (int, Stats, error)
	CountOpenVisitsInRoomWithRevision(context.Context, int64) (int, int64, Stats, error)
	ListOpenVisitRooms(context.Context, int64) ([]studentpresence.OpenVisitRoom, Stats, error)
}
