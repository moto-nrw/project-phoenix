package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

func NewCareScheduleQueue(query careplan.CareScheduleReviewQuery, today func() careplan.Date) (requestreview.Queue, error) {
	if query == nil || today == nil {
		return nil, errors.New("care schedule queue: native query and clock are required")
	}
	return careScheduleQueue{query: query, today: today}, nil
}

type careScheduleQueue struct {
	query careplan.CareScheduleReviewQuery
	today func() careplan.Date
}

func careQueueFilter(filter requestreview.QueueFilter) careplan.RequestQueueFilter {
	result := careplan.RequestQueueFilter{UrgentOnly: filter.UrgentOnly, UrgentDate: filter.UrgentDate, StudentIDs: filter.StudentIDs, StudentID: filter.StudentID, Search: filter.Search, Limit: filter.Limit}
	if filter.Before != nil {
		result.BeforeInstant, result.BeforeID = filter.Before.Instant, filter.Before.ID
	}
	return result
}

func careQueueCursor(cursor *careplan.RequestCursor) *requestreview.Cursor {
	if cursor == nil {
		return nil
	}
	return &requestreview.Cursor{Instant: cursor.UpdatedAt, ID: cursor.ID}
}

func (q careScheduleQueue) Open(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, cursor, err := q.query.ListPending(ctx, careQueueFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	today, err := careplan.ParseDate(filter.UrgentDate)
	if err != nil {
		today = q.today()
	}
	rows := make([]requestreview.Row, 0, len(items))
	for _, item := range items {
		rows = append(rows, requestreview.Row{
			Type: requestreview.TypeCareSchedule, SortTime: item.Request.CreatedAt,
			ID: item.Request.ID, StudentID: item.Request.StudentID, StudentName: item.FirstName + " " + item.LastName, Status: item.Request.Status,
			Version: excusedrequests.ParentRequestVersion(item.Request.UpdatedAt), UrgentToday: careplan.CareReviewUrgentOn(*item, today), Past: careplan.CareReviewPastOn(*item, today),
			BulkIneligibleReason: "single_only", BulkIneligibleText: "Betreuungszeiten müssen einzeln geprüft werden.",
			ConflictKeys: careplan.CareReviewConflictKeys(*item), Data: ToCareRequestResponse(item),
		})
	}
	return rows, careQueueCursor(cursor), nil
}

func (q careScheduleQueue) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, cursor, err := q.query.ListHistory(ctx, careQueueFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	rows := make([]requestreview.Row, 0, len(items))
	for _, item := range items {
		row := item.Request
		rows = append(rows, requestreview.Row{Type: requestreview.TypeCareSchedule, SortTime: row.UpdatedAt,
			ID: row.ID, StudentID: row.StudentID, StudentName: item.FirstName + " " + item.LastName, Status: row.Status,
			Version: excusedrequests.ParentRequestVersion(row.UpdatedAt), DecidedAt: HistoryDecidedAt(row.ReviewedAt, row.UpdatedAt),
			CanCorrect: row.RequestKind == "pickup_change" && (row.Status == "approved" || row.Status == "rejected"), Data: ToCareRequestHistoryResponse(item),
		})
	}
	return rows, careQueueCursor(cursor), nil
}

func (q careScheduleQueue) OpenCount(ctx context.Context, today requestreview.Date) (int, error) {
	items, _, err := q.query.ListPending(ctx, careplan.RequestQueueFilter{UrgentDate: today.String()})
	return len(items), err
}
