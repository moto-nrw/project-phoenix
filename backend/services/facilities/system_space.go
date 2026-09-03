package facilities

import (
	"context"
	"time"
)

type SystemActivity struct {
	ID              int64
	Name            string
	MaxParticipants int
	IsOpen          bool
	CategoryID      int64
	PlannedRoomID   *int64
	IsSystem        bool
	CreatedBy       *int64
}

type SystemCategory struct {
	ID          int64
	Name        string
	Description string
	Color       string
	IsSystem    bool
}

type ActivityCatalog interface {
	ListActivities(context.Context, string) ([]SystemActivity, error)
	CreateActivity(context.Context, SystemActivity) (SystemActivity, error)
	ListCategories(context.Context) ([]SystemCategory, error)
	CreateCategory(context.Context, SystemCategory) (SystemCategory, error)
}

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
