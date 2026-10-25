package requestreview

import (
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

func HistoryDecidedAt(reviewedAt *time.Time, updatedAt time.Time) time.Time {
	if reviewedAt != nil {
		return *reviewedAt
	}
	return updatedAt
}

func ToCareRequestDiffResponses(entries []careplan.CareRequestDiffEntry) []CareRequestDiffResponse {
	out := make([]CareRequestDiffResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, CareRequestDiffResponse{
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
func ToCareRequestResponse(item *careplan.CareScheduleReviewItem) CareRequestResponse {
	r := item.Request
	return CareRequestResponse{
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
		Signed:          carerequests.StoredSignature(r.Payload) != nil,
	}
}

func toAffectedCareBlocks(blocks []careplan.CareReviewBlock) []AffectedCareBlock {
	out := make([]AffectedCareBlock, 0, len(blocks))
	for _, block := range blocks {
		out = append(out, AffectedCareBlock{
			ID:        strconv.FormatInt(block.ID, 10),
			Title:     block.Title,
			StartTime: block.StartTime.Format("15:04"),
			EndTime:   block.EndTime.Format("15:04"),
		})
	}
	return out
}

// ToCareRequestHistoryResponse maps one decided care-schedule request (#2432).
func ToCareRequestHistoryResponse(item *careplan.CareScheduleHistoryItem) CareRequestHistoryResponse {
	req := item.Request
	requested := ToCareRequestDiffResponses(item.Requested)
	var diff []CareRequestDiffResponse
	if len(item.Diff) > 0 {
		diff = ToCareRequestDiffResponses(item.Diff)
	}
	return CareRequestHistoryResponse{
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
		Signed:         carerequests.StoredSignature(req.Payload) != nil,
	}
}

func careImpactToken(item *careplan.CareScheduleReviewItem) string {
	if !item.ImpactAvailable {
		return ""
	}
	return securityruntime.Fingerprint(carerequests.PickupImpactContent(item.AffectedBlocks))
}
