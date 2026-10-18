package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RequestConflictRecords interface {
	Find(context.Context, int64, bool) (carerequests.Request, error)
}

type RequestConflictPlans interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	// CurrentStaffID is the caller's staff profile; found is false when the
	// caller has none.
	CurrentStaffID(context.Context) (staffID int64, found bool, err error)
	SaveApprovedException(context.Context, int64, int64, calendar.Date, time.Time, string, int64) (int64, error)
	Sync(context.Context, int64) error
	UpsertStudentPickupSchedule(context.Context, *careplan.PickupSchedule) error
}
