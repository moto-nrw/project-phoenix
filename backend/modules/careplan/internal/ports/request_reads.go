package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

type RequestReadRecords interface {
	PendingWeekly(context.Context, int64) (*carerequests.Request, error)
	RecentPickup(context.Context, int64, time.Time) ([]carerequests.Request, error)
}
