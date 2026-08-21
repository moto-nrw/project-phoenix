package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// CareRequestResponse is the staff-facing projection of one parent
// care-schedule change request in the review queue, including the live
// "current → requested" weekly diff.
type CareRequestResponse struct {
	ID              string                    `json:"id"`
	StudentID       string                    `json:"student_id"`
	FirstName       string                    `json:"first_name"`
	LastName        string                    `json:"last_name"`
	Status          string                    `json:"status"`
	RequestKind     string                    `json:"request_kind"`
	Diff            []CareRequestDiffResponse `json:"diff"`
	RequestReason   *string                   `json:"request_reason,omitempty"`
	DecisionReason  *string                   `json:"decision_reason,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	ReviewedAt      *time.Time                `json:"reviewed_at,omitempty"`
	AffectedBlocks  []AffectedCareBlock       `json:"affected_blocks"`
	ImpactAvailable bool                      `json:"impact_available"`
	ImpactToken     string                    `json:"impact_token"`
}

type AffectedCareBlock struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// CareRequestDiffResponse mirrors the request-diff wire shape the messaging
// thread page used, so the frontend's RequestDiffPanel renders it unchanged.
type CareRequestDiffResponse struct {
	Label    string   `json:"label"`
	Old      string   `json:"old"`
	New      string   `json:"new"`
	Weekday  int      `json:"weekday,omitempty"`
	CareKind string   `json:"care_kind,omitempty"`
	OldModes []string `json:"old_modes,omitempty"`
	NewMode  string   `json:"new_mode,omitempty"`
}

// toCareRequestDiffResponses maps service diff entries onto the wire shape —
// shared by the review queue and both history projections (frozen diff and
// requested summary; the latter's Old/OldModes are empty by construction).
func toCareRequestDiffResponses(entries []scheduleService.RequestDiffEntry) []CareRequestDiffResponse {
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

func toCareRequestResponse(item *scheduleService.CareRequestReviewItem) CareRequestResponse {
	r := item.Request
	diff := toCareRequestDiffResponses(item.Diff)
	return CareRequestResponse{
		ID:              strconv.FormatInt(r.ID, 10),
		StudentID:       strconv.FormatInt(r.StudentID, 10),
		FirstName:       item.FirstName,
		LastName:        item.LastName,
		Status:          r.Status,
		RequestKind:     r.RequestKind,
		Diff:            diff,
		RequestReason:   item.Reason,
		DecisionReason:  r.DecisionReason,
		CreatedAt:       r.CreatedAt,
		ReviewedAt:      r.ReviewedAt,
		AffectedBlocks:  toAffectedCareBlocks(item.AffectedBlocks),
		ImpactAvailable: item.ImpactAvailable,
		ImpactToken:     item.ImpactToken,
	}
}

func toAffectedCareBlocks(blocks []scheduleModels.PartialAbsenceBlock) []AffectedCareBlock {
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

// DecideCareRequestBody is the body of POST
// .../care-schedule-change-requests/{requestId}/decide.
type DecideCareRequestBody struct {
	Approve     *bool   `json:"approve"`
	Reason      string  `json:"reason"`
	ImpactToken *string `json:"impact_token"`
}

// decideCareScheduleChangeRequest approves (applies the weekly plan) or
// rejects (reason required) one pending request.
func (rs *Resource) decideCareScheduleChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.CareRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care request service not configured")))
		return
	}
	input, ok := decodeCareRequestDecision(w, r)
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	input.ReviewedBy = int64(claims.ID)
	item, err := rs.CareRequestService.Decide(r.Context(), input)
	if err != nil {
		renderError(w, r, careRequestDecisionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareRequestResponse(item), "Decision applied")
}

func decodeCareRequestDecision(w http.ResponseWriter, r *http.Request) (scheduleService.CareRequestDecideInput, bool) {
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return scheduleService.CareRequestDecideInput{}, false
	}
	var body DecideCareRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return scheduleService.CareRequestDecideInput{}, false
	}
	if body.Approve == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("approve is required")))
		return scheduleService.CareRequestDecideInput{}, false
	}
	return scheduleService.CareRequestDecideInput{
		RequestID: requestID, Approve: *body.Approve, Reason: body.Reason,
		ExpectedImpactToken: body.ImpactToken, RequireImpactToken: true,
	}, true
}

var careRequestDecisionErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: scheduleModels.ErrCareRequestNotFound, Render: common.ErrorNotFound},
	{Target: scheduleModels.ErrCareRequestNotPending, Render: conflictWithCode("change_request_not_pending")},
	{Target: scheduleService.ErrCareRequestGuardianAccessRevoked, Render: conflictWithCode("guardian_access_revoked")},
	{Target: scheduleService.ErrCareRequestForbidden, Render: common.ErrorForbidden},
	{Target: scheduleService.ErrPickupChangeConflict, Render: conflictWithCode("pickup_change_conflict")},
	{Target: scheduleService.ErrPickupChangeAlreadyCompleted, Render: conflictWithCode("pickup_change_completed")},
	{Target: scheduleService.ErrPickupChangeExpired, Render: conflictWithCode("pickup_change_expired")},
	{Target: scheduleService.ErrPickupChangeImpactChanged, Render: conflictWithCode("pickup_change_impact_changed")},
	{Match: isInvalidCareRequestDecision, Render: common.ErrorInvalidRequest},
}, careRequestDecisionFallback)

func conflictWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorConflictWithCode(err, code) }
}

func careRequestDecisionFallback(err error) render.Renderer {
	if renderer := companionPlanErrorRenderer(err); renderer != nil {
		return renderer
	}
	return common.ErrorInternalServer(err)
}

func isInvalidCareRequestDecision(err error) bool {
	return errors.Is(err, scheduleService.ErrCareRequestRejectReasonRequired) ||
		errors.Is(err, scheduleService.ErrCareRequestRejectReasonTooLong) ||
		errors.Is(err, scheduleService.ErrInvalidCareRequestPayload) ||
		errors.Is(err, scheduleService.ErrPickupChangeImpactRequired)
}
