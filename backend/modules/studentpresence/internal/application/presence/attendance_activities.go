package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// AttendanceActivityGroups supplies activity templates and dashboard categories.
type AttendanceActivityGroups interface {
	FindByID(context.Context, any) (*ports.SessionActivity, error)
	FindByIDs(context.Context, []int64) ([]*ports.SessionActivity, error)
	ListSessionActivities(context.Context) ([]*ports.SessionActivity, error)
}

// AttendanceActivityCategories supplies the dashboard category count.
type AttendanceActivityCategories interface {
	CountCategories(context.Context) (int, error)
}
