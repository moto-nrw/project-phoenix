package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

func NewMasterDataQueue(query careplan.MasterDataReviewQuery) (requestreview.Queue, error) {
	if query == nil {
		return nil, errors.New("master data queue: native query is required")
	}
	return masterDataQueue{service: query}, nil
}

type masterDataQueue struct {
	service careplan.MasterDataReviewQuery
}

func (q masterDataQueue) Open(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListPending(ctx, careQueueFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, masterDataPendingRow), careQueueCursor(next), nil
}

func (q masterDataQueue) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListHistory(ctx, careQueueFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, masterDataHistoryRow), careQueueCursor(next), nil
}

func (q masterDataQueue) OpenCount(ctx context.Context, today requestreview.Date) (int, error) {
	items, _, err := q.service.ListPending(ctx, excusedrequests.QueueFilter{UrgentDate: today.String()})
	return len(items), err
}

func mapRows[T any](items []T, build func(T) requestreview.Row) []requestreview.Row {
	rows := make([]requestreview.Row, 0, len(items))
	for _, item := range items {
		rows = append(rows, build(item))
	}
	return rows
}

func masterDataPendingRow(item *masterdatarequests.ReviewItem) requestreview.Row {
	return requestreview.Row{
		Type:                 requestreview.TypeMasterData,
		SortTime:             item.Request.CreatedAt,
		ID:                   item.Request.ID,
		StudentID:            item.Request.StudentID,
		StudentName:          item.FirstName + " " + item.LastName,
		Status:               item.Request.Status,
		Version:              excusedrequests.ParentRequestVersion(item.Request.UpdatedAt),
		BulkEligible:         item.BulkEligible,
		BulkIneligibleReason: item.BulkIneligibleReason,
		BulkIneligibleText:   item.BulkIneligibleText,
		ConflictKeys:         item.Request.ConflictKeys(),
		CurrentValueChanged:  item.CurrentValueChanged,
		Data:                 ToMasterDataChangeRequestResponse(item),
	}
}

func masterDataHistoryRow(item *masterdatarequests.HistoryItem) requestreview.Row {
	return requestreview.Row{
		Type:        requestreview.TypeMasterData,
		SortTime:    item.Request.UpdatedAt,
		Version:     excusedrequests.ParentRequestVersion(item.Request.UpdatedAt),
		ID:          item.Request.ID,
		StudentID:   item.Request.StudentID,
		StudentName: item.FirstName + " " + item.LastName,
		Status:      item.Request.Status,
		DecidedAt:   HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		CanCorrect:  item.Request.CanCorrect(),
		Data:        ToMasterDataHistoryResponse(item),
	}
}
