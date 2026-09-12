package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

type excusedQueue struct {
	service excusedrequests.ReviewQuery
	today   func() excusedrequests.Date
}

func NewExcusedQueue(query excusedrequests.ReviewQuery, today func() excusedrequests.Date) (requestreview.Queue, error) {
	if query == nil || today == nil {
		return nil, errors.New("excused queue: native query and clock are required")
	}
	return excusedQueue{service: query, today: today}, nil
}

func (q excusedQueue) Open(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListPending(ctx, careQueueFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	today, parseErr := excusedrequests.ParseDate(filter.UrgentDate)
	if parseErr != nil {
		today = q.today()
	}
	return mapRows(items, func(item *excusedrequests.ReviewItem) requestreview.Row {
		return excusedPendingRow(item, today)
	}), careQueueCursor(next), nil
}

func (q excusedQueue) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListHistory(ctx, careQueueFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, excusedHistoryRow), careQueueCursor(next), nil
}

// OpenCount counts the whole queue on the request's shared day.
func (q excusedQueue) OpenCount(ctx context.Context, today requestreview.Date) (int, error) {
	items, _, err := q.service.ListPending(ctx, excusedrequests.QueueFilter{UrgentDate: today.String()})
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

func excusedPendingRow(item *excusedrequests.ReviewItem, today excusedrequests.Date) requestreview.Row {
	urgent := item.Request.UrgentOn(today)
	return requestreview.Row{
		Type:                 requestreview.TypeExcused,
		SortTime:             item.Request.CreatedAt,
		ID:                   item.Request.ID,
		StudentID:            item.Request.StudentID,
		StudentName:          item.FirstName + " " + item.LastName,
		Status:               item.Request.Status,
		Version:              excusedrequests.ParentRequestVersion(item.Request.UpdatedAt),
		UrgentToday:          urgent,
		Past:                 item.Request.PastOn(today),
		BulkEligible:         item.BulkEligible,
		BulkIneligibleReason: item.BulkIneligibleReason,
		BulkIneligibleText:   item.BulkIneligibleText,
		ConflictKeys:         item.Request.ConflictKeys(),
		CurrentValueChanged:  item.CurrentValueChanged,
		CurrentStatusByDate:  item.CurrentStatusByDate,
		Data:                 ToStaffExcusedRequestResponse(item),
	}
}

func excusedHistoryRow(item *excusedrequests.HistoryItem) requestreview.Row {
	return requestreview.Row{
		Type:        requestreview.TypeExcused,
		SortTime:    item.Request.UpdatedAt,
		Version:     excusedrequests.ParentRequestVersion(item.Request.UpdatedAt),
		ID:          item.Request.ID,
		StudentID:   item.Request.StudentID,
		StudentName: item.FirstName + " " + item.LastName,
		Status:      item.Request.Status,
		DecidedAt:   HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		CanCorrect:  item.Request.CanCorrect(),
		Data:        ToStaffExcusedHistoryResponse(item),
	}
}
