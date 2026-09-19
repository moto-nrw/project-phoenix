package careplan

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/careplan/offeringrequests"
)

type OfferingReviewDiffEntry = offeringrequests.DiffEntry

func OfferingReviewConflictKeys(item OfferingReviewItem) []string {
	keys := make([]string, 0, len(item.Diff))
	for _, entry := range item.Diff {
		if entry.OfferingID > 0 {
			keys = append(keys, "offer:"+strconv.FormatInt(entry.OfferingID, 10))
		}
	}
	return keys
}

func OfferingReviewUrgentOn(item OfferingReviewItem, today Date) bool {
	return !Date(item.Request.EffectiveFrom).After(today)
}

func OfferingReviewPastOn(item OfferingReviewItem, today Date) bool {
	day := Date(item.Request.EffectiveFrom)
	return !day.IsZero() && day.Before(today)
}

type OfferingReviewItem = offeringrequests.ReviewItem
type OfferingHistoryItem = offeringrequests.HistoryItem
type OfferingRequestedItem = offeringrequests.RequestedItem
type OfferingReviewSelection = offeringrequests.Selection

func CanonicalOfferingReviewDays(days []string) []string {
	return offeringrequests.CanonicalDays(days)
}

func ParseOfferingReviewSelections(raw json.RawMessage) ([]OfferingReviewSelection, error) {
	return offeringrequests.ParseSelections(raw)
}

func ParseOfferingReviewDecisionDiff(raw json.RawMessage) ([]OfferingReviewDiffEntry, error) {
	return offeringrequests.DecisionDiff(raw)
}

type OfferingChangeRequestQuery interface {
	ListOfferingChanges(context.Context, OfferingChangeFilter) ([]OfferingChangeRequest, error)
}

type OfferingReviewQuery interface {
	ListPending(context.Context, RequestQueueFilter) ([]*OfferingReviewItem, *RequestCursor, error)
	ListHistory(context.Context, RequestQueueFilter) ([]*OfferingHistoryItem, *RequestCursor, error)
	PendingCount(context.Context, Date) (int, error)
}

func ReviewCorrectionDiff(before, after json.RawMessage) ([]OfferingReviewDiffEntry, error) {
	return offeringrequests.CorrectionDiff(before, after)
}
