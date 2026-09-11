package students

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	requestreviewlegacy "github.com/moto-nrw/project-phoenix/modules/requestreview/legacy"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// CareRequestDetailResponse is one care-schedule request of any status, read
// from a message-thread pill (#3135). It is the history projection plus the
// guardian's reason; a still-pending row carries no decision fields. The
// requested summary and the frozen diff come from the row and its snapshot,
// never from today's weekly plan, so later corrections do not rewrite what
// was asked for or decided.
type CareRequestDetailResponse struct {
	ID             string                    `json:"id"`
	StudentID      string                    `json:"student_id"`
	FirstName      string                    `json:"first_name"`
	LastName       string                    `json:"last_name"`
	Status         string                    `json:"status"`
	RequestKind    string                    `json:"request_kind"`
	Requested      []CareRequestDiffResponse `json:"requested"`
	Diff           []CareRequestDiffResponse `json:"diff,omitempty"`
	RequestReason  *string                   `json:"request_reason,omitempty"`
	DecisionReason *string                   `json:"decision_reason,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	DecidedAt      *time.Time                `json:"decided_at,omitempty"`
	DecidedByName  string                    `json:"decided_by_name,omitempty"`
	// PickupChange is the stored ask of a pickup-change row; absent for
	// weekly-plan rows. Dates are calendar days, times wall-clock "15:04".
	PickupChange *CareRequestPickupChangeResponse `json:"pickup_change,omitempty"`
}

type CareRequestPickupChangeResponse struct {
	Date               string `json:"date"`
	PickupTime         string `json:"pickup_time"`
	PreviousPickupTime string `json:"previous_pickup_time,omitempty"`
}

func toCareRequestDetailResponse(item *scheduleService.CareRequestHistoryItem) CareRequestDetailResponse {
	req := item.Request
	var diff []CareRequestDiffResponse
	if len(item.Diff) > 0 {
		diff = requestreviewlegacy.ToCareRequestDiffResponses(item.Diff)
	}
	var pickupChange *CareRequestPickupChangeResponse
	if terms := item.PickupChange; terms != nil {
		pickupChange = &CareRequestPickupChangeResponse{
			Date:               terms.Date.String(),
			PickupTime:         terms.PickupTime,
			PreviousPickupTime: terms.PreviousPickupTime,
		}
	}
	var decidedAt *time.Time
	if req.Status != scheduleModels.CareRequestStatusPending {
		at := requestreviewlegacy.HistoryDecidedAt(req.ReviewedAt, req.UpdatedAt)
		decidedAt = &at
	}
	return CareRequestDetailResponse{
		ID:             strconv.FormatInt(req.ID, 10),
		StudentID:      strconv.FormatInt(req.StudentID, 10),
		FirstName:      item.FirstName,
		LastName:       item.LastName,
		Status:         req.Status,
		RequestKind:    req.RequestKind,
		Requested:      requestreviewlegacy.ToCareRequestDiffResponses(item.Requested),
		Diff:           diff,
		RequestReason:  item.RequestReason,
		DecisionReason: req.DecisionReason,
		CreatedAt:      req.CreatedAt,
		DecidedAt:      decidedAt,
		DecidedByName:  item.ReviewerName,
		PickupChange:   pickupChange,
	}
}

// getCareScheduleChangeRequest serves GET
// .../care-schedule-change-requests/{requestId}: one request of any status
// for a reader the review policy allows for the child. A row of another
// school is not found; a child outside the reader's scope is forbidden.
func (rs *Resource) getCareScheduleChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.CareRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care request service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	item, err := rs.CareRequestService.GetForReview(r.Context(), requestID)
	if err != nil {
		renderError(w, r, careRequestDetailErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareRequestDetailResponse(item), "Care request")
}

var careRequestDetailErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Target: scheduleModels.ErrCareRequestNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Target: scheduleService.ErrCareRequestForbidden, Render: common.ErrorForbidden},
), common.ErrorInternalServer)
