package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CallerReader is the slice of the user-context service the school portal
// resolves the caller with.
type CallerReader interface {
	GetMySchoolClasses(ctx context.Context) ([]string, error)
	// CurrentStaffID resolves the caller's staff member; found is false,
	// without an error, for a caller who is no staff member.
	CurrentStaffID(ctx context.Context) (staffID int64, found bool, err error)
}

// ArrivalWriteScope decides whether the school portal may write class
// arrival exceptions; the root binds it to the Settings Platform.

// ClassDayPickupReader is the pickup capability the day report reads: the
// effective times and the Abholart of single days (#3945).
type ClassDayPickupReader interface {
	PickupTimeReader
	GetBulkDayDepartureModesForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]string, error)
}

// CareDayResolver is the retained schedule service's care-day derivation.
type CareDayResolver interface {
	ResolveForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]careplan.CareDayStatus, error)
}

// PickupTimeReader is the retained pickup schedule service's bulk read.
type PickupTimeReader interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*careplan.EffectivePickupTime, error)
}

// ArrivalTimeReader is the retained arrival schedule service's bulk read.
type ArrivalTimeReader interface {
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*careplan.EffectiveArrivalTime, error)
}
