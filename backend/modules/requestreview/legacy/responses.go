package legacy

import (
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// The mappers from the retained service items to the projection's per-type
// wire shapes. The decide routes render the same queue shapes after a
// decision, so they share these functions with the list.

// HistoryDecidedAt is the display "decided at" instant: reviewed_at when a
// reviewer stamped the decision, otherwise updated_at (withdrawn and
// auto-applied rows carry no reviewed_at, but every decide path stamps
// updated_at in the same UPDATE).
func HistoryDecidedAt(reviewedAt *time.Time, updatedAt time.Time) time.Time {
	if reviewedAt != nil {
		return *reviewedAt
	}
	return updatedAt
}

// ToMasterDataChangeRequestResponse maps one Stammdaten request.
func ToMasterDataChangeRequestResponse(item *userService.MasterDataReviewItem) requestreview.MasterDataChangeRequestResponse {
	r := item.Request
	return requestreview.MasterDataChangeRequestResponse{
		ID:         strconv.FormatInt(r.ID, 10),
		StudentID:  strconv.FormatInt(r.StudentID, 10),
		FirstName:  item.FirstName,
		LastName:   item.LastName,
		Target:     r.Target,
		FieldKey:   r.FieldKey,
		OldValue:   r.OldValue,
		NewValue:   r.NewValue,
		Status:     r.Status,
		CreatedAt:  r.CreatedAt,
		ReviewedAt: r.ReviewedAt,
	}
}

// ToMasterDataHistoryResponse maps one decided Stammdaten request (#2432).
func ToMasterDataHistoryResponse(item *userService.MasterDataHistoryItem) requestreview.MasterDataChangeRequestHistoryResponse {
	return requestreview.MasterDataChangeRequestHistoryResponse{
		MasterDataChangeRequestResponse: ToMasterDataChangeRequestResponse(&userService.MasterDataReviewItem{
			Request:   item.Request,
			FirstName: item.FirstName,
			LastName:  item.LastName,
		}),
		DecidedAt:     HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
		ReviewReason:  item.Request.ReviewReason,
	}
}

// ToCareRequestDiffResponses maps service diff entries onto the wire shape —
// shared by the review queue and both history projections (frozen diff and
// requested summary; the latter's Old/OldModes are empty by construction).
func ToCareRequestDiffResponses(entries []scheduleService.RequestDiffEntry) []requestreview.CareRequestDiffResponse {
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
func ToCareRequestResponse(item *scheduleService.CareRequestReviewItem) requestreview.CareRequestResponse {
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
		ImpactToken:     item.ImpactToken,
	}
}

func toAffectedCareBlocks(blocks []scheduleModels.PartialAbsenceBlock) []requestreview.AffectedCareBlock {
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
func ToCareRequestHistoryResponse(item *scheduleService.CareRequestHistoryItem) requestreview.CareRequestHistoryResponse {
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

// OfferingRequestDiffLines renders the review diff, including the
// bookkeeping a Mitbuchungs-Regel line carries so the card can explain and
// override it.
func OfferingRequestDiffLines(entries []enrollmentService.OfferingChangeDiffEntry) []requestreview.OfferingRequestDiffResponse {
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

// ToOfferingRequestResponse maps one offering-change request with its live
// diff (#1665).
func ToOfferingRequestResponse(item *enrollmentService.OfferingChangeView) requestreview.OfferingRequestResponse {
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
		EffectiveFrom:  timezone.Date(row.EffectiveFrom).String(),
		Diff:           diff,
		Reason:         row.DecisionReason,
		FullWithdrawal: item.FullWithdrawal,
		CreatedAt:      row.CreatedAt,
		ReviewedAt:     row.ReviewedAt,
	}
	if !item.EarliestEffectiveFrom.IsZero() {
		resp.EarliestEffectiveFrom = item.EarliestEffectiveFrom.String()
	}
	if !item.LatestEffectiveFrom.IsZero() {
		resp.LatestEffectiveFrom = item.LatestEffectiveFrom.String()
	}
	if !item.RequestedEffectiveFrom.IsZero() && item.RequestedEffectiveFrom != timezone.Date(row.EffectiveFrom) {
		resp.RequestedEffectiveFrom = item.RequestedEffectiveFrom.String()
	}
	if len(unchanged) > 0 {
		resp.Unchanged = unchanged
	}
	if row.ParentNote != nil {
		resp.Note = *row.ParentNote
	}
	return resp
}

// ToOfferingRequestHistoryResponse maps one decided offering request (#2432).
func ToOfferingRequestHistoryResponse(item *enrollmentService.OfferingChangeHistoryItem) requestreview.OfferingRequestHistoryResponse {
	requested := make([]requestreview.OfferingRequestRequestedResponse, 0, len(item.Requested))
	for _, entry := range item.Requested {
		requested = append(requested, requestreview.OfferingRequestRequestedResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Name,
			New:        GermanOfferingDiffLabel("booked", entry.Days),
		})
	}
	return requestreview.OfferingRequestHistoryResponse{
		OfferingRequestResponse: ToOfferingRequestResponse(&enrollmentService.OfferingChangeView{
			Request:     item.Request,
			StudentName: item.StudentName,
			Diff:        item.Diff,
		}),
		DecidedAt:     HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
		Requested:     requested,
	}
}

// GermanOfferingDiffLabel renders one side of an offering diff line for the
// German-only staff portal.
func GermanOfferingDiffLabel(state string, days []string) string {
	switch state {
	case "not_booked":
		return "nicht gebucht"
	case "removed":
		return "abgemeldet"
	}
	if len(days) == 0 {
		return "alle Betreuungstage"
	}
	labels := map[string]string{
		"mon": "Mo", "tue": "Di", "wed": "Mi", "thu": "Do",
		"fri": "Fr", "sat": "Sa", "sun": "So",
	}
	parts := make([]string, 0, len(days))
	for _, day := range days {
		if label, ok := labels[day]; ok {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, ", ")
}

// ToDirectCorrectionResponse maps one office correction (#2436).
func ToDirectCorrectionResponse(item *enrollmentService.DirectCorrectionItem) requestreview.DirectCorrectionResponse {
	row := item.Adjustment
	diff := make([]requestreview.OfferingRequestDiffResponse, 0, len(item.Diff))
	for _, entry := range item.Diff {
		diff = append(diff, requestreview.OfferingRequestDiffResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Label,
			Old:        GermanOfferingDiffLabel(entry.OldState, entry.OldDays),
			New:        GermanOfferingDiffLabel(entry.NewState, entry.NewDays),
		})
	}
	return requestreview.DirectCorrectionResponse{
		ID:            strconv.FormatInt(row.ID, 10),
		StudentID:     strconv.FormatInt(row.StudentID, 10),
		StudentName:   item.StudentName,
		ChangedAt:     row.ChangedAt,
		ChangedByName: item.ActorName,
		Reason:        row.Reason,
		Diff:          diff,
	}
}

// ToStaffExcusedRequestResponse maps one parent absence approval request.
func ToStaffExcusedRequestResponse(item *excusedrequests.ReviewItem) requestreview.StaffExcusedRequestResponse {
	r := item.Request
	dates := make([]string, 0, len(r.Dates))
	for _, d := range r.Dates {
		dates = append(dates, d.String())
	}
	return requestreview.StaffExcusedRequestResponse{
		ID:            strconv.FormatInt(r.ID, 10),
		StudentID:     strconv.FormatInt(r.StudentID, 10),
		FirstName:     item.FirstName,
		LastName:      item.LastName,
		AbsenceStatus: r.AbsenceStatus,
		Status:        r.Status,
		Dates:         dates,
		Note:          r.Note,
		Reason:        r.DecisionReason,
		CreatedAt:     r.CreatedAt,
		ReviewedAt:    r.ReviewedAt,
	}
}

// ToStaffExcusedHistoryResponse maps one decided excused-absence request
// (#2432).
func ToStaffExcusedHistoryResponse(item *excusedrequests.HistoryItem) requestreview.StaffExcusedRequestHistoryResponse {
	return requestreview.StaffExcusedRequestHistoryResponse{
		StaffExcusedRequestResponse: ToStaffExcusedRequestResponse(&excusedrequests.ReviewItem{
			Request:   item.Request,
			FirstName: item.FirstName,
			LastName:  item.LastName,
		}),
		DecidedAt:     HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
	}
}
