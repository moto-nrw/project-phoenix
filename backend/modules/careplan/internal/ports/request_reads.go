package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

type RequestReadRecords interface {
	ListCareRequests(context.Context, int64, []string, []string, time.Time) ([]carerequests.Request, error)
}
