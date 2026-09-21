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

type requestReadRecords struct{ RequestReadRecords }

func (r requestReadRecords) PendingWeekly(ctx context.Context, id int64) (*carerequests.Request, error) {
	rows, err := r.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{StudentID: id, RequestKinds: []string{"weekly_schedule"}, Statuses: []string{"pending"}})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
func (r requestReadRecords) RecentPickup(ctx context.Context, id int64, since time.Time) ([]carerequests.Request, error) {
	return r.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{StudentID: id, RequestKinds: []string{"pickup_change"}, RecentSince: since})
}
