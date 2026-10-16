package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
)

type RequestReadRecords interface {
	ListCareScheduleRequests(context.Context, careplan.CareScheduleRequestFilter) ([]carerequests.Request, error)
}

func NewRequestReads(records RequestReadRecords, diffs carerequests.Diffs, logger *slog.Logger) (carerequests.Reads, error) {
	if records == nil || diffs == nil {
		return nil, errors.New("care request reads: records and diffs are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &application.RequestReads{Records: requestReadRecords{records}, Diffs: diffs, Logger: logger}, nil
}

// NewParentCareRequestQueries retains request lookup/decoding for parent sharing.
func NewParentCareRequestQueries(records application.ParentCareRequestRecords, missing func() error) carerequests.ParentCareRequestQueries {
	return application.ParentCareRequestReads{Records: records, Missing: missing}
}

// requestReadRecords translates the application's list selection into the public query filter.
type requestReadRecords struct{ RequestReadRecords }

func (r requestReadRecords) ListCareRequests(ctx context.Context, studentID int64, kinds, statuses []string, since time.Time) ([]carerequests.Request, error) {
	return r.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{StudentID: studentID, RequestKinds: kinds, Statuses: statuses, RecentSince: since})
}
