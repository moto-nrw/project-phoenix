package careplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// CareScheduleRequestQuery is the bounded storage read used by staff review.
type CareScheduleRequestQuery interface {
	ListCareScheduleRequests(context.Context, CareScheduleRequestFilter) ([]CareScheduleChangeRequest, error)
}

type CareScheduleReviewQuery interface {
	ListPending(context.Context, RequestQueueFilter) ([]*CareScheduleReviewItem, *RequestCursor, error)
	ListHistory(context.Context, RequestQueueFilter) ([]*CareScheduleHistoryItem, *RequestCursor, error)
}

type CareScheduleReviewItem = carerequests.ReviewItem
type CareScheduleHistoryItem = carerequests.HistoryItem
type CareReviewBlock = carerequests.Block
