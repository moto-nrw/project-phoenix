package timetracking

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// balanceAdjustmentErrorRules classifies the adjustment service sentinels
// (#1420): caller mistakes → 400, missing rows → 404. The conflict
// sentinels are rendered with machine-readable codes in
// renderBalanceAdjustmentError before this table is consulted.
var balanceAdjustmentErrorRules = []common.ErrorRule{
	{Target: activeSvc.ErrAdjustmentInvalid, Render: common.ErrorInvalidRequest},
	{Target: activeSvc.ErrAdjustmentNotFound, Render: common.ErrorNotFound},
}

func renderBalanceAdjustmentError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, activeSvc.ErrBalanceAlreadyReset) {
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "balance_already_reset"))
		return
	}
	if errors.Is(err, activeSvc.ErrAdjustmentHasDependentReset) {
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "dependent_balance_reset"))
		return
	}
	if errors.Is(err, activeSvc.ErrAdjustmentExceedsBalance) {
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "balance_adjustment_exceeds_balance"))
		return
	}
	if errors.Is(err, activeSvc.ErrAdjustmentInClosedMonth) {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "adjustment_in_closed_month"))
		return
	}
	if errors.Is(err, activeSvc.ErrOpeningAlreadyExists) {
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "opening_balance_already_exists"))
		return
	}
	common.RenderError(w, r, common.RenderWithRules(err, balanceAdjustmentErrorRules, common.ErrorInternalServer))
}

func (rs *StaffAdminResource) requireBalanceAdjustmentStaff(w http.ResponseWriter, r *http.Request, staffID int64) bool {
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		if common.IsNotFound(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return false
	}
	return true
}

// createBalanceAdjustmentRequest is the wire shape for POST
// /api/staff/{id}/time-tracking/adjustments. MinutesDelta is signed and must
// be negative — payout and comp-time grants only reduce the Stundenkonto.
type createBalanceAdjustmentRequest struct {
	Type          string `json:"type"`
	MinutesDelta  int    `json:"minutes_delta"`
	EffectiveDate string `json:"effective_date"`
	Note          string `json:"note"`
}

// resetBalanceRequest is the wire shape for POST
// /api/staff/{id}/time-tracking/reset (#1420 5c).
type resetBalanceRequest struct {
	EffectiveDate    string `json:"effective_date"`
	CarryoverMinutes int    `json:"carryover_minutes"`
	Note             string `json:"note"`
}

// listBalanceAdjustments handles GET /api/staff/{id}/time-tracking/adjustments?from=&to=
func (rs *StaffAdminResource) listBalanceAdjustments(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.requireBalanceAdjustmentStaff(w, r, staffID) {
		return
	}
	from, err := timezone.ParseDate(r.URL.Query().Get("from"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}
	to, err := timezone.ParseDate(r.URL.Query().Get("to"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}
	adjustments, err := rs.BalanceAdjustService.ListAdjustments(r.Context(), staffID, from, to)
	if err != nil {
		renderBalanceAdjustmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, adjustments, "Balance adjustments retrieved")
}

// createBalanceAdjustment handles POST /api/staff/{id}/time-tracking/adjustments
func (rs *StaffAdminResource) createBalanceAdjustment(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.requireBalanceAdjustmentStaff(w, r, staffID) {
		return
	}
	decidedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	var req createBalanceAdjustmentRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	effectiveDate, err := timezone.ParseDate(req.EffectiveDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid effective_date format, expected YYYY-MM-DD")))
		return
	}
	adjustment, err := rs.BalanceAdjustService.CreateAdjustment(r.Context(), staffID, decidedBy, activeSvc.CreateBalanceAdjustmentRequest{
		Type:          req.Type,
		MinutesDelta:  req.MinutesDelta,
		EffectiveDate: effectiveDate,
		Note:          req.Note,
	})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBalanceAdjustmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, adjustment, "Balance adjustment created")
}

// deleteBalanceAdjustment handles DELETE /api/staff/{id}/time-tracking/adjustments/{adjustmentId}
func (rs *StaffAdminResource) deleteBalanceAdjustment(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.requireBalanceAdjustmentStaff(w, r, staffID) {
		return
	}
	adjustmentID, err := parseInt64Param(r, "adjustmentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	deletedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	if err := rs.BalanceAdjustService.DeleteAdjustment(r.Context(), staffID, adjustmentID, deletedBy); err != nil {
		tenant.MarkRollback(r.Context())
		renderBalanceAdjustmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Balance adjustment deleted")
}

// openingBalanceRequest is the wire shape for POST
// /api/staff/{id}/time-tracking/opening (#2132). BalanceMinutes is SIGNED —
// a migrated Stundenkonto may start negative.
type openingBalanceRequest struct {
	EffectiveDate  string `json:"effective_date"`
	BalanceMinutes *int   `json:"balance_minutes"`
	Note           string `json:"note"`
}

// createOpeningBalance handles POST /api/staff/{id}/time-tracking/opening
func (rs *StaffAdminResource) createOpeningBalance(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.requireBalanceAdjustmentStaff(w, r, staffID) {
		return
	}
	decidedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	var req openingBalanceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.BalanceMinutes == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("balance_minutes is required")))
		return
	}
	effectiveDate, err := timezone.ParseDate(req.EffectiveDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid effective_date format, expected YYYY-MM-DD")))
		return
	}
	adjustment, err := rs.BalanceAdjustService.CreateOpeningBalance(r.Context(), staffID, decidedBy, effectiveDate, *req.BalanceMinutes, req.Note)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBalanceAdjustmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, adjustment, "Opening balance created")
}

// resetStaffBalance handles POST /api/staff/{id}/time-tracking/reset
func (rs *StaffAdminResource) resetStaffBalance(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.requireBalanceAdjustmentStaff(w, r, staffID) {
		return
	}
	decidedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	var req resetBalanceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	effectiveDate, err := timezone.ParseDate(req.EffectiveDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid effective_date format, expected YYYY-MM-DD")))
		return
	}
	adjustment, err := rs.BalanceAdjustService.ResetBalance(r.Context(), staffID, decidedBy, effectiveDate, req.CarryoverMinutes, req.Note)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderBalanceAdjustmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, adjustment, "Balance reset")
}
