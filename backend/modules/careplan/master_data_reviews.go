package careplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
)

type StudentDataRequestQuery interface {
	ListStudentDataRequests(context.Context, StudentDataRequestFilter) ([]StudentDataChangeRequest, error)
}

// MasterDataReviewQuery is the staff read capability for parent field changes.
// It preserves the owner queue's keyset before applying child-level scope.
type MasterDataReviewQuery interface {
	ListPending(context.Context, RequestQueueFilter) ([]*MasterDataReviewItem, *RequestCursor, error)
	ListHistory(context.Context, RequestQueueFilter) ([]*MasterDataHistoryItem, *RequestCursor, error)
}
type MasterDataReviewItem = masterdatarequests.ReviewItem
type MasterDataHistoryItem = masterdatarequests.HistoryItem
