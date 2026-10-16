package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RequestSubmissionRecords interface {
	Create(context.Context, carerequests.Request) (carerequests.Request, error)
	IsPendingConflict(error) bool
}

type RequestSubmissionPickup interface {
	PickupTime(context.Context, int64, calendar.Date) (*time.Time, error)
	// DayDepartureMode is the Abholart the OGS set for that day (#3945);
	// found is false when the day has none.
	DayDepartureMode(context.Context, int64, calendar.Date) (mode string, found bool, err error)
	// WeeklyDepartureModes is the child's effective weekly Abholart plan,
	// keyed "mon" to "fri".
	WeeklyDepartureModes(context.Context, int64) (map[string][]string, error)
}

type RequestSubmissionEffects interface {
	RecordSubmission(context.Context, *carerequests.Request) error
	NotifySubmission(context.Context, *carerequests.Request) error
	WakeGuardians(context.Context, *carerequests.Request)
}
