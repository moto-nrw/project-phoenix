package active

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// VisitWithRoom enriches a presence visit with its session and room directory data.
type VisitWithRoom struct {
	studentpresence.Visit
	ActiveGroup *VisitRoomGroup
}

type VisitRoomGroup struct {
	ID, RoomID                                    int64
	CreatedAt, UpdatedAt, StartTime, LastActivity time.Time
	EndTime                                       *time.Time
	GroupID, DeviceID                             *int64
	TimeoutMinutes                                int
	Room                                          *VisitRoom
}

type VisitRoom struct {
	ID                   int64
	CreatedAt, UpdatedAt time.Time
	Name                 string
}
