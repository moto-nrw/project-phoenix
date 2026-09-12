package compose

import (
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/careplan/offeringrequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

func OfferingRequestDiffLines(entries []offeringrequests.DiffEntry) []requestreview.OfferingRequestDiffResponse {
	diff := make([]requestreview.OfferingRequestDiffResponse, 0, len(entries))
	for _, entry := range entries {
		line := requestreview.OfferingRequestDiffResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Label,
			Old:        GermanOfferingDiffLabel(entry.OldState, entry.OldDays),
			New:        GermanOfferingDiffLabel(entry.NewState, entry.NewDays),
			IsCourse:   entry.IsCourse,
		}
		if len(entry.NewAutomaticDays) > 0 {
			line.Automatic = true
			line.AutomaticDays = GermanOfferingDiffLabel("booked", entry.NewAutomaticDays)
			if len(entry.NewRuleDays) > 0 {
				line.RuleDays = GermanOfferingDiffLabel("booked", entry.NewRuleDays)
			}
			line.Optoutable = len(entry.AutoTriggerIDs) > 0
			if len(entry.NewDaysWithoutRules) > 0 {
				line.NewWhenExcluded = GermanOfferingDiffLabel("booked", entry.NewDaysWithoutRules)
			}
			for _, triggerID := range entry.AutoTriggerIDs {
				line.TriggerIDs = append(line.TriggerIDs, strconv.FormatInt(triggerID, 10))
			}
			line.TriggerNames = entry.AutoTriggerNames
		}
		diff = append(diff, line)
	}
	return diff
}

func ToOfferingRequestResponse(item *offeringrequests.ReviewItem) requestreview.OfferingRequestResponse {
	row := item.Request
	diff := OfferingRequestDiffLines(item.Diff)
	unchanged := make([]requestreview.OfferingRequestUnchangedResponse, 0, len(item.Unchanged))
	for _, entry := range item.Unchanged {
		unchanged = append(unchanged, requestreview.OfferingRequestUnchangedResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Label,
			Days:       GermanOfferingDiffLabel(entry.NewState, entry.NewDays),
		})
	}
	resp := requestreview.OfferingRequestResponse{
		ID:             strconv.FormatInt(row.ID, 10),
		StudentID:      strconv.FormatInt(row.StudentID, 10),
		StudentName:    item.StudentName,
		Status:         row.Status,
		EffectiveFrom:  row.EffectiveFrom,
		Diff:           diff,
		Reason:         row.DecisionReason,
		FullWithdrawal: item.FullWithdrawal,
		CreatedAt:      row.CreatedAt,
		ReviewedAt:     row.ReviewedAt,
	}
	if item.EarliestEffectiveFrom != "" {
		resp.EarliestEffectiveFrom = item.EarliestEffectiveFrom
	}
	if item.LatestEffectiveFrom != "" {
		resp.LatestEffectiveFrom = item.LatestEffectiveFrom
	}
	if item.RequestedEffectiveFrom != "" && item.RequestedEffectiveFrom != row.EffectiveFrom {
		resp.RequestedEffectiveFrom = item.RequestedEffectiveFrom
	}
	if len(unchanged) > 0 {
		resp.Unchanged = unchanged
	}
	if row.ParentNote != nil {
		resp.Note = *row.ParentNote
	}
	return resp
}

func ToOfferingRequestHistoryResponse(item *offeringrequests.HistoryItem) requestreview.OfferingRequestHistoryResponse {
	requested := make([]requestreview.OfferingRequestRequestedResponse, 0, len(item.Requested))
	for _, entry := range item.Requested {
		requested = append(requested, requestreview.OfferingRequestRequestedResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Name,
			New:        GermanOfferingDiffLabel("booked", entry.Days),
		})
	}
	return requestreview.OfferingRequestHistoryResponse{
		OfferingRequestResponse: ToOfferingRequestResponse(&offeringrequests.ReviewItem{
			Request:     item.Request,
			StudentName: item.StudentName,
			Diff:        item.Diff,
		}),
		DecidedAt:     HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
		Requested:     requested,
	}
}
