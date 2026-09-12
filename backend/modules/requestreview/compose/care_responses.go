package compose

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

func HistoryDecidedAt(reviewedAt *time.Time, updatedAt time.Time) time.Time {
	if reviewedAt != nil {
		return *reviewedAt
	}
	return updatedAt
}

func ToCareRequestDiffResponses(entries []careplan.CareRequestDiffEntry) []requestreview.CareRequestDiffResponse {
	out := make([]requestreview.CareRequestDiffResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, requestreview.CareRequestDiffResponse{
			Label:    e.Label,
			Old:      e.Old,
			New:      e.New,
			Weekday:  e.Weekday,
			CareKind: e.CareKind,
			OldModes: e.OldModes,
			NewMode:  e.NewMode,
		})
	}
	return out
}

// ToCareRequestResponse maps one care-schedule request with its live diff.
func ToCareRequestResponse(item *careplan.CareScheduleReviewItem) requestreview.CareRequestResponse {
	r := item.Request
	return requestreview.CareRequestResponse{
		ID:              strconv.FormatInt(r.ID, 10),
		StudentID:       strconv.FormatInt(r.StudentID, 10),
		FirstName:       item.FirstName,
		LastName:        item.LastName,
		Status:          r.Status,
		RequestKind:     r.RequestKind,
		Diff:            ToCareRequestDiffResponses(item.Diff),
		RequestReason:   item.Reason,
		DecisionReason:  r.DecisionReason,
		CreatedAt:       r.CreatedAt,
		ReviewedAt:      r.ReviewedAt,
		AffectedBlocks:  toAffectedCareBlocks(item.AffectedBlocks),
		ImpactAvailable: item.ImpactAvailable,
		ImpactToken:     careImpactToken(item),
	}
}

func toAffectedCareBlocks(blocks []careplan.CareReviewBlock) []requestreview.AffectedCareBlock {
	out := make([]requestreview.AffectedCareBlock, 0, len(blocks))
	for _, block := range blocks {
		out = append(out, requestreview.AffectedCareBlock{
			ID:        strconv.FormatInt(block.ID, 10),
			Title:     block.Title,
			StartTime: block.StartTime.Format("15:04"),
			EndTime:   block.EndTime.Format("15:04"),
		})
	}
	return out
}

// ToCareRequestHistoryResponse maps one decided care-schedule request (#2432).
func ToCareRequestHistoryResponse(item *careplan.CareScheduleHistoryItem) requestreview.CareRequestHistoryResponse {
	req := item.Request
	requested := ToCareRequestDiffResponses(item.Requested)
	var diff []requestreview.CareRequestDiffResponse
	if len(item.Diff) > 0 {
		diff = ToCareRequestDiffResponses(item.Diff)
	}
	return requestreview.CareRequestHistoryResponse{
		ID:             strconv.FormatInt(req.ID, 10),
		StudentID:      strconv.FormatInt(req.StudentID, 10),
		FirstName:      item.FirstName,
		LastName:       item.LastName,
		Status:         req.Status,
		RequestKind:    req.RequestKind,
		Requested:      requested,
		Diff:           diff,
		DecisionReason: req.DecisionReason,
		CreatedAt:      req.CreatedAt,
		DecidedAt:      HistoryDecidedAt(req.ReviewedAt, req.UpdatedAt),
		DecidedByName:  item.ReviewerName,
	}
}

func careImpactToken(item *careplan.CareScheduleReviewItem) string {
	if !item.ImpactAvailable {
		return ""
	}
	hash := sha256.New()
	for _, block := range item.AffectedBlocks {
		for _, value := range []string{strconv.FormatInt(block.ID, 10), block.Title, block.StartTime.Format("15:04:05"), block.EndTime.Format("15:04:05")} {
			hash.Write([]byte(value))
			hash.Write([]byte{0})
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
