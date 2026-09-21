package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

// AttendanceActivityGroups supplies activity templates and dashboard categories.
type AttendanceActivityGroups interface {
	FindByID(context.Context, any) (*active.SessionActivity, error)
	FindByIDs(context.Context, []int64) ([]*active.SessionActivity, error)
	ListSessionActivities(context.Context) ([]*active.SessionActivity, error)
}

// AttendanceActivityCategories supplies the dashboard category count.
type AttendanceActivityCategories interface {
	CountCategories(context.Context) (int, error)
}
