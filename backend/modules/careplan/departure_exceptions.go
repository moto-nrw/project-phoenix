package careplan

import (
	"context"
	"errors"
	"time"
)

// Day departure modes (#3945). They are the People Directory's weekly
// departure vocabulary, carried as plain strings because Care Plan owns only
// the day override, never the weekly plan.
const (
	DepartureModeAlone       = "alone"
	DepartureModeBus         = "bus"
	DepartureModePickup      = "pickup"
	DepartureModeAccompanied = "accompanied"
)

// ErrDepartureExceptionNotCareDay refuses a day override where the school
// does not run care. Such a value would have no schedule view to apply to.
var ErrDepartureExceptionNotCareDay = errors.New("die Abholart kann nur an einem Betreuungstag geändert werden")

// DepartureException is the Abholart of one child on one day. It overrides
// the weekly plan for that day only and is independent of the Gehzeit, which
// stays in PickupException.
type DepartureException struct {
	ID            int64
	TenantID      int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	StudentID     int64
	ExceptionDate Date
	DepartureMode string
	Reason        *string
	// CreatedBy is the staff membership that set or approved the value. It is
	// nil only after that membership was deleted.
	CreatedBy *int64
}

// DepartureExceptions is the owner's day-departure capability. Save replaces
// the value of the child's day; there is at most one per child and day.
type DepartureExceptions interface {
	ListDepartureExceptions(context.Context, StudentScheduleFilter) ([]DepartureException, error)
	SaveDepartureException(context.Context, DepartureException) (DepartureException, error)
	DeleteDepartureException(context.Context, int64) error
}

// ValidDepartureMode reports whether mode names one of the four Abholarten.
func ValidDepartureMode(mode string) bool {
	switch mode {
	case DepartureModeAlone, DepartureModeBus, DepartureModePickup, DepartureModeAccompanied:
		return true
	default:
		return false
	}
}
