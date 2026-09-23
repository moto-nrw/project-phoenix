package careplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// CareScheduleRequestQuery is the bounded storage read used by staff review.
type CareScheduleRequestQuery interface {
	FindCareScheduleRequest(context.Context, int64, bool) (CareScheduleChangeRequest, error)
	ListCareScheduleRequests(context.Context, CareScheduleRequestFilter) ([]CareScheduleChangeRequest, error)
}

type CareScheduleReviewQuery interface {
	GetForReview(context.Context, int64) (*carerequests.HistoryItem, error)
	ListPending(context.Context, RequestQueueFilter) ([]*CareScheduleReviewItem, *RequestCursor, error)
	ListHistory(context.Context, RequestQueueFilter) ([]*CareScheduleHistoryItem, *RequestCursor, error)
}

type CareScheduleReviewItem = carerequests.ReviewItem
type CareScheduleHistoryItem = carerequests.HistoryItem
type CareReviewBlock = carerequests.Block
