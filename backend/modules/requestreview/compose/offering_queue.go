package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

func NewOfferingQueue(query careplan.OfferingReviewQuery, today func() careplan.Date) (requestreview.Queue, error) {
	if query == nil || today == nil {
		return nil, errors.New("offering queue: native query and clock are required")
	}
	return offeringQueue{query: query, today: today}, nil
}

type offeringQueue struct {
	query careplan.OfferingReviewQuery
	today func() careplan.Date
}

func (q offeringQueue) Open(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, cursor, err := q.query.ListPending(ctx, careQueueFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	day, err := careplan.ParseDate(filter.UrgentDate)
	if err != nil {
		day = q.today()
	}
	rows := make([]requestreview.Row, 0, len(items))
	for _, item := range items {
		row := item.Request
		rows = append(rows, requestreview.Row{Type: requestreview.TypeOffering, SortTime: row.CreatedAt, ID: row.ID, StudentID: row.StudentID, StudentName: item.StudentName, Status: row.Status,
			Version: excusedrequests.ParentRequestVersion(row.UpdatedAt), UrgentToday: careplan.OfferingReviewUrgentOn(*item, day), Past: careplan.OfferingReviewPastOn(*item, day),
			BulkIneligibleReason: "single_only", BulkIneligibleText: "Angebote müssen einzeln geprüft werden.", ConflictKeys: careplan.OfferingReviewConflictKeys(*item), Data: ToOfferingRequestResponse(item),
		})
	}
	return rows, careQueueCursor(cursor), nil
}
func (q offeringQueue) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, cursor, err := q.query.ListHistory(ctx, careQueueFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	rows := make([]requestreview.Row, 0, len(items))
	for _, item := range items {
		row := item.Request
		rows = append(rows, requestreview.Row{Type: requestreview.TypeOffering, SortTime: row.UpdatedAt, ID: row.ID, StudentID: row.StudentID, StudentName: item.StudentName, Status: row.Status,
			Version: excusedrequests.ParentRequestVersion(row.UpdatedAt), DecidedAt: HistoryDecidedAt(row.ReviewedAt, row.UpdatedAt), Data: ToOfferingRequestHistoryResponse(item),
		})
	}
	return rows, careQueueCursor(cursor), nil
}
func (q offeringQueue) OpenCount(ctx context.Context, today requestreview.Date) (int, error) {
	return q.query.PendingCount(ctx, careplan.Date(today.String()))
}
