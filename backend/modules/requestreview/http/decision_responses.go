package requestreviewhttp

import (
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

// CareRequestDetailResponse is one care-schedule request of any status, read
// from a message-thread pill (#3135). It is the history projection plus the
// guardian's reason; a still-pending row carries no decision fields. The
// requested summary and the frozen diff come from the row and its snapshot,
// never from today's weekly plan, so later corrections do not rewrite what
// was asked for or decided.
type CareRequestDetailResponse struct {
	ID             string                                  `json:"id"`
	StudentID      string                                  `json:"student_id"`
	FirstName      string                                  `json:"first_name"`
	LastName       string                                  `json:"last_name"`
	Status         string                                  `json:"status"`
	RequestKind    string                                  `json:"request_kind"`
	Requested      []requestreview.CareRequestDiffResponse `json:"requested"`
	Diff           []requestreview.CareRequestDiffResponse `json:"diff,omitempty"`
	RequestReason  *string                                 `json:"request_reason,omitempty"`
	DecisionReason *string                                 `json:"decision_reason,omitempty"`
	CreatedAt      time.Time                               `json:"created_at"`
	DecidedAt      *time.Time                              `json:"decided_at,omitempty"`
	DecidedByName  string                                  `json:"decided_by_name,omitempty"`
	// PickupChange is the stored ask of a pickup-change row; absent for
	// weekly-plan rows. Dates are calendar days, times wall-clock "15:04".
	PickupChange *CareRequestPickupChangeResponse `json:"pickup_change,omitempty"`
	// Signed says the guardian signed the request (#4780).
	Signed bool `json:"signed"`
}

// CareRequestPickupChangeResponse is the stored ask of one day. PickupTime
// or DepartureMode is empty for a leg the request leaves unchanged (#3945).
type CareRequestPickupChangeResponse struct {
	Date                   string   `json:"date"`
	PickupTime             string   `json:"pickup_time,omitempty"`
	PreviousPickupTime     string   `json:"previous_pickup_time,omitempty"`
	DepartureMode          string   `json:"departure_mode,omitempty"`
	PreviousDepartureModes []string `json:"previous_departure_modes,omitempty"`
	// PickupPerson is the one-day collector the family named (#4780).
	PickupPerson string `json:"pickup_person,omitempty"`
}

func toCareRequestDetailResponse(item *carerequests.HistoryItem) CareRequestDetailResponse {
	req := item.Request
	var diff []requestreview.CareRequestDiffResponse
	if len(item.Diff) > 0 {
		diff = requestreview.ToCareRequestDiffResponses(item.Diff)
	}
	var pickupChange *CareRequestPickupChangeResponse
	if terms := item.PickupChange; terms != nil {
		pickupChange = &CareRequestPickupChangeResponse{
			Date:                   terms.Date.String(),
			PickupTime:             terms.PickupTime,
			PreviousPickupTime:     terms.PreviousPickupTime,
			DepartureMode:          terms.DepartureMode,
			PreviousDepartureModes: terms.PreviousDepartureModes,
			PickupPerson:           terms.PickupPersonName,
		}
	}
	var decidedAt *time.Time
	if req.Status != carerequests.StatusPending {
		at := requestreview.HistoryDecidedAt(req.ReviewedAt, req.UpdatedAt)
		decidedAt = &at
	}
	return CareRequestDetailResponse{
		ID:             strconv.FormatInt(req.ID, 10),
		StudentID:      strconv.FormatInt(req.StudentID, 10),
		FirstName:      item.FirstName,
		LastName:       item.LastName,
		Status:         req.Status,
		RequestKind:    req.RequestKind,
		Requested:      requestreview.ToCareRequestDiffResponses(item.Requested),
		Diff:           diff,
		RequestReason:  item.RequestReason,
		DecisionReason: req.DecisionReason,
		CreatedAt:      req.CreatedAt,
		DecidedAt:      decidedAt,
		DecidedByName:  item.ReviewerName,
		PickupChange:   pickupChange,
		Signed:         carerequests.StoredSignature(req.Payload) != nil,
	}
}

func WriteCareDetail(w http.ResponseWriter, r *http.Request, item *carerequests.HistoryItem) {
	common.Respond(w, r, http.StatusOK, toCareRequestDetailResponse(item), "Care request")
}
func WriteCareDecision(w http.ResponseWriter, r *http.Request, item *careplan.CareScheduleReviewItem) {
	common.Respond(w, r, http.StatusOK, requestreview.ToCareRequestResponse(item), "Decision applied")
}
func WriteMasterDataDecision(w http.ResponseWriter, r *http.Request, item *masterdatarequests.ReviewItem) {
	common.Respond(w, r, http.StatusOK, requestreview.ToMasterDataChangeRequestResponse(item), "Decision applied")
}
func WriteExcusedDecision(w http.ResponseWriter, r *http.Request, item *excusedrequests.ReviewItem) {
	common.Respond(w, r, http.StatusOK, requestreview.ToStaffExcusedRequestResponse(item), "Decision applied")
}
