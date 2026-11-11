package enrollmenthttp

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The allocation of holiday places by criteria in the Träger-Portal (#4700):
// the criteria of the Träger at /traeger/holiday-periods/criteria, the
// allocation list of a period and its decisions, and the Büro's check of a
// registration's answers. The same Träger session, the right "Anmeldung
// verwalten" and the Träger transaction as the periods.

func (rs *HolidayPeriodResource) allocationRoutes(r chi.Router, withTx common.Middleware) {
	allocate := r.With(rs.requireManage, withTx)
	allocate.Get("/{id}/allocation", rs.allocationBoard)
	allocate.Post("/{id}/allocation/decisions", rs.decideAllocation)
	allocate.Put("/{id}/registrations/{registrationId}/criteria", rs.checkCriteria)
}

type holidayCriterionRequest struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Question string `json:"question"`
	Scope    string `json:"scope"`
	Active   bool   `json:"active"`
}

// HolidayCriteriaRequest is the body of PUT /traeger/holiday-periods/criteria:
// every criterion of the Träger in the order of its priority.
type HolidayCriteriaRequest struct {
	Criteria []holidayCriterionRequest `json:"criteria"`
}

// Bind accepts the body; the Owner validates the list.
func (req *HolidayCriteriaRequest) Bind(*http.Request) error { return nil }

func (rs *HolidayPeriodResource) listCriteria(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	criteria, err := rs.allocation.HolidayCriteria(r.Context(), reviewer)
	if err != nil {
		renderHolidayAllocationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, criteria, "Holiday criteria retrieved successfully")
}

func (rs *HolidayPeriodResource) saveCriteria(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &HolidayCriteriaRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	input := make([]capability.HolidayCriterionInput, len(req.Criteria))
	for i, criterion := range req.Criteria {
		input[i] = capability.HolidayCriterionInput(criterion)
	}
	criteria, err := rs.allocation.SaveHolidayCriteria(r.Context(), reviewer, input)
	var fields validation.Errors
	if errors.As(err, &fields) {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err != nil {
		renderHolidayAllocationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, criteria, "Holiday criteria saved successfully")
}

func (rs *HolidayPeriodResource) allocationBoard(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	board, err := rs.allocation.HolidayAllocationBoard(r.Context(), reviewer, periodID)
	if err != nil {
		renderHolidayAllocationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, board, "Holiday allocation retrieved successfully")
}

type holidayAllocationChoiceRequest struct {
	BookingID int64  `json:"booking_id"`
	Placement string `json:"placement"`
	Proposal  string `json:"proposal"`
	Reason    string `json:"reason"`
}

// HolidayAllocationRequest is the body of POST
// /traeger/holiday-periods/{id}/allocation/decisions: a decision per
// pending booking with the proposal the Büro saw and, against it, a reason.
type HolidayAllocationRequest struct {
	Decisions []holidayAllocationChoiceRequest `json:"decisions"`
}

// Bind accepts the body; the Owner checks the decisions.
func (req *HolidayAllocationRequest) Bind(*http.Request) error { return nil }

func (rs *HolidayPeriodResource) decideAllocation(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &HolidayAllocationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	choices := make([]capability.HolidayAllocationChoice, len(req.Decisions))
	for i, choice := range req.Decisions {
		choices[i] = capability.HolidayAllocationChoice(choice)
	}
	board, err := rs.allocation.DecideHolidayAllocation(r.Context(), reviewer, periodID, choices)
	if err != nil {
		renderHolidayAllocationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, board, "Holiday places allocated successfully")
}

// HolidayCriteriaCheckRequest is the body of PUT
// /traeger/holiday-periods/{id}/registrations/{registrationId}/criteria.
type HolidayCriteriaCheckRequest struct {
	Checks []capability.HolidayCriterionCheck `json:"checks"`
}

// Bind accepts the body; the Owner checks the checks.
func (req *HolidayCriteriaCheckRequest) Bind(*http.Request) error { return nil }

func (rs *HolidayPeriodResource) checkCriteria(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	req := &HolidayCriteriaCheckRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.allocation.CheckHolidayCriteria(r.Context(), reviewer, periodID, registrationID, req.Checks)
	if err != nil {
		renderHolidayAllocationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Holiday criteria checked successfully")
}

// renderHolidayAllocationError answers a refused allocation; every refusal
// rolls the Träger transaction back.
func renderHolidayAllocationError(w http.ResponseWriter, r *http.Request, err error) {
	tenant.MarkRollback(r.Context())
	var full *capability.HolidayAllocationFullRefusal
	var reason *capability.HolidayAllocationReasonRefusal
	switch {
	case errors.As(err, &reason):
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeEnrollmentHolidayRegistrationInvalid,
			fmt.Sprintf("decisions.%d.reason", reason.Index)))
	case errors.As(err, &full):
		common.RenderError(w, r, common.ErrorConflictWithDetails(err, common.CodeEnrollmentHolidayAllocationFull, map[string]any{"week_id": full.WeekID}))
	case errors.Is(err, capability.ErrHolidayAllocationOff):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, capability.ErrHolidayAllocationChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayAllocationChanged))
	default:
		renderHolidayReviewError(w, r, err)
	}
}
