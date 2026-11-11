package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayOwnChildException is the existing dated plan needed to reconcile a
// holiday booking. Excused marks a pickup the team has taken over.
type HolidayOwnChildException struct {
	ID, StudentID int64
	Day           calendar.Date
	Reason        *string
	Excused       bool
}

// HolidayOwnChildPlan is one arrival or pickup to write for a booked day.
type HolidayOwnChildPlan struct {
	StudentID int64
	Day       calendar.Date
	Time      time.Time
	Reason    string
}

// HolidayOwnChildPlanChanges contains only the exceptions Enrollment owns.
type HolidayOwnChildPlanChanges struct {
	Arrivals, Pickups             []HolidayOwnChildPlan
	DeleteArrivals, DeletePickups []int64
}

// HolidayOwnChildPlans binds Care Plan's reads and guarded bulk commands in
// the caller's tenant transaction. Enrollment decides which entries it owns.
type HolidayOwnChildPlans interface {
	HolidayOwnChildArrivals(context.Context, calendar.Date) ([]HolidayOwnChildException, error)
	HolidayOwnChildPickups(context.Context, calendar.Date) ([]HolidayOwnChildException, error)
	ApplyHolidayOwnChildPlans(context.Context, HolidayOwnChildPlanChanges) error
}
