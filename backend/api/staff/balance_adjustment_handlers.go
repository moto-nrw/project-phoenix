package staff

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
// (#1420): caller mistakes → 400, missing rows → 404, double reset → 409.
var balanceAdjustmentErrorRules = []common.ErrorRule{
	{Target: activeSvc.ErrAdjustmentInvalid, Render: common.ErrorInvalidRequest},
	{Target: activeSvc.ErrAdjustmentNotFound, Render: common.ErrorNotFound},
	{Target: activeSvc.ErrBalanceAlreadyReset, Render: common.ErrorConflict},
	{Target: activeSvc.ErrAdjustmentHasDependentReset, Render: common.ErrorConflict},
}

func renderBalanceAdjustmentError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, balanceAdjustmentErrorRules, common.ErrorInternalServer))
}

func (rs *Resource) requireBalanceAdjustmentStaff(w http.ResponseWriter, r *http.Request, staffID int64) bool {
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
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
func (rs *Resource) listBalanceAdjustments(w http.ResponseWriter, r *http.Request) {
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
func (rs *Resource) createBalanceAdjustment(w http.ResponseWriter, r *http.Request) {
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
func (rs *Resource) deleteBalanceAdjustment(w http.ResponseWriter, r *http.Request) {
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
	if err := rs.BalanceAdjustService.DeleteAdjustment(r.Context(), staffID, adjustmentID); err != nil {
		tenant.MarkRollback(r.Context())
		renderBalanceAdjustmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Balance adjustment deleted")
}

// resetStaffBalance handles POST /api/staff/{id}/time-tracking/reset
func (rs *Resource) resetStaffBalance(w http.ResponseWriter, r *http.Request) {
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
