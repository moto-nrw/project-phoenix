package facilities

import (
	"context"
	"time"
)

type OpenGroup struct {
	ID        int64
	StartTime time.Time
	EndTime   *time.Time
	IsToday   bool
}

type OpenGroupSupervisor struct {
	ID        int64
	GroupID   int64
	StaffID   int64
	Ended     bool
	FirstName string
	LastName  string
}

type OpenGroupVisit struct{ ExitTime *time.Time }

type OpenGroupCatalog interface {
	ListByRoom(context.Context, int64) ([]OpenGroup, error)
	ListSupervisors(context.Context, []int64) ([]OpenGroupSupervisor, error)
	ListVisits(context.Context, int64) ([]OpenGroupVisit, error)
}
