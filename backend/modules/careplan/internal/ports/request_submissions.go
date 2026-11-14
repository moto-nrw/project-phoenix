package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

type RequestSubmissionRecords interface {
	Create(context.Context, carerequests.Request) (carerequests.Request, error)
	IsPendingConflict(error) bool
}

// RequestWeeklyBaseline renders a weekly ask against the plan the family
// sees, so a submission or edit can store that view as its baseline (#6990).
type RequestWeeklyBaseline interface {
	Weekly(context.Context, int64, json.RawMessage) ([]carerequests.DiffEntry, error)
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
