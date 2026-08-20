package students

import (
	"strconv"
	"time"

	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// The aggregated list defaults to a screenful and caps the page size against
// abusive limits; the frontend loads more via the cursor.
const (
	historyDefaultLimit = 25
	historyMaxLimit     = 100
)

// historyCursorPayload is one type's keyset position inside the aggregated
// cursor. The whole cursor is base64url-encoded JSON, opaque so clients cannot
// build one by hand and the encoding may change.
type historyCursorPayload struct {
	UpdatedAt time.Time `json:"u"`
	ID        int64     `json:"i"`
}

// historyDecidedAt is the display "decided at" instant: reviewed_at when a
// reviewer stamped the decision, otherwise updated_at (withdrawn and
// auto-applied rows carry no reviewed_at, but every decide path stamps
// updated_at in the same UPDATE).
func historyDecidedAt(reviewedAt *time.Time, updatedAt time.Time) time.Time {
	if reviewedAt != nil {
		return *reviewedAt
	}
	return updatedAt
}

// MasterDataChangeRequestHistoryResponse extends the queue projection with the
// decision facts for the staff history.
type MasterDataChangeRequestHistoryResponse struct {
	MasterDataChangeRequestResponse
	DecidedAt     time.Time `json:"decided_at"`
	DecidedByName string    `json:"decided_by_name,omitempty"`
	ReviewReason  *string   `json:"review_reason,omitempty"`
}

// toMasterDataHistoryResponse maps one decided Stammdaten request for the
// aggregated list (#2432).
func toMasterDataHistoryResponse(item *userService.MasterDataHistoryItem) MasterDataChangeRequestHistoryResponse {
	return MasterDataChangeRequestHistoryResponse{
		MasterDataChangeRequestResponse: toMasterDataChangeRequestResponse(&userService.MasterDataReviewItem{
			Request:   item.Request,
			FirstName: item.FirstName,
			LastName:  item.LastName,
		}),
		DecidedAt:     historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
		ReviewReason:  item.Request.ReviewReason,
	}
}

// CareRequestHistoryResponse is one decided care-schedule request. It never
// carries a recomputed live diff — current data has moved on since the
// decision. Diff replays the frozen decision snapshot (ADR 0002, #2430);
// rows without one (withdrawn, pre-snapshot) fall back to the payload-derived
// requested summary (each entry's old side is empty).
type CareRequestHistoryResponse struct {
	ID             string                    `json:"id"`
	StudentID      string                    `json:"student_id"`
	FirstName      string                    `json:"first_name"`
	LastName       string                    `json:"last_name"`
	Status         string                    `json:"status"`
	RequestKind    string                    `json:"request_kind"`
	Requested      []CareRequestDiffResponse `json:"requested"`
	Diff           []CareRequestDiffResponse `json:"diff,omitempty"`
	DecisionReason *string                   `json:"decision_reason,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	DecidedAt      time.Time                 `json:"decided_at"`
	DecidedByName  string                    `json:"decided_by_name,omitempty"`
}

// toCareRequestHistoryResponse maps one decided care-schedule request for the
// aggregated list (#2432).
func toCareRequestHistoryResponse(item *scheduleService.CareRequestHistoryItem) CareRequestHistoryResponse {
	req := item.Request
	requested := toCareRequestDiffResponses(item.Requested)
	var diff []CareRequestDiffResponse
	if len(item.Diff) > 0 {
		diff = toCareRequestDiffResponses(item.Diff)
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
		DecidedAt:      historyDecidedAt(req.ReviewedAt, req.UpdatedAt),
		DecidedByName:  item.ReviewerName,
	}
}

// OfferingRequestHistoryResponse extends the queue projection with the
// decision facts. Its diff comes from the frozen decision snapshot.
type OfferingRequestHistoryResponse struct {
	OfferingRequestResponse
	DecidedAt     time.Time                          `json:"decided_at"`
	DecidedByName string                             `json:"decided_by_name,omitempty"`
	Requested     []OfferingRequestRequestedResponse `json:"requested,omitempty"`
}

// OfferingRequestRequestedResponse is a payload-derived recap for history
// rows without a frozen decision snapshot, such as withdrawn requests.
type OfferingRequestRequestedResponse struct {
	OfferingID string `json:"offering_id"`
	Label      string `json:"label"`
	New        string `json:"new"`
}

// toOfferingRequestHistoryResponse maps one decided offering request for the
// aggregated list (#2432).
func toOfferingRequestHistoryResponse(item *enrollmentService.OfferingChangeHistoryItem) OfferingRequestHistoryResponse {
	requested := make([]OfferingRequestRequestedResponse, 0, len(item.Requested))
	for _, entry := range item.Requested {
		requested = append(requested, OfferingRequestRequestedResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Name,
			New:        germanOfferingDiffLabel("booked", entry.Days),
		})
	}
	return OfferingRequestHistoryResponse{
		OfferingRequestResponse: toOfferingRequestResponse(&enrollmentService.OfferingChangeView{
			Request:     item.Request,
			StudentName: item.StudentName,
			Diff:        item.Diff,
		}),
		DecidedAt:     historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
		Requested:     requested,
	}
}

// StaffExcusedRequestHistoryResponse extends the queue projection with the
// decision facts for the staff history.
type StaffExcusedRequestHistoryResponse struct {
	StaffExcusedRequestResponse
	DecidedAt     time.Time `json:"decided_at"`
	DecidedByName string    `json:"decided_by_name,omitempty"`
}

// toStaffExcusedHistoryResponse maps one decided excused-absence request for
// the aggregated list (#2432).
func toStaffExcusedHistoryResponse(item *absenceService.ExcusedRequestHistoryItem) StaffExcusedRequestHistoryResponse {
	return StaffExcusedRequestHistoryResponse{
		StaffExcusedRequestResponse: toStaffExcusedRequestResponse(&absenceService.ExcusedRequestReviewItem{
			Request:   item.Request,
			FirstName: item.FirstName,
			LastName:  item.LastName,
		}),
		DecidedAt:     historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
	}
}
