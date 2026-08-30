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

// renderVacationOpeningError classifies the vacation takeover sentinels
// (#2132): caller mistakes → 400, missing rows → 404, duplicate takeover or
// double-count risk → 409 with a stable code.
func renderVacationOpeningError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, activeSvc.ErrVacationOpeningExists):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "vacation_opening_already_exists"))
	case errors.Is(err, activeSvc.ErrVacationOpeningAbsencesBeforeCutoff):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "vacation_opening_absences_before_cutoff"))
	case errors.Is(err, activeSvc.ErrVacationOpeningNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, activeSvc.ErrVacationOpeningInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// setVacationOpeningRequest is the wire shape for POST
// /api/staff/{id}/vacation/opening (#2132). The admin enters the Resturlaub
// as of the Stichtag; the backend derives the pre-introduction days.
type setVacationOpeningRequest struct {
	EffectiveDate string   `json:"effective_date"`
	RemainingDays *float64 `json:"remaining_days"`
	Note          string   `json:"note"`
}

// setVacationOpening handles POST /api/staff/{id}/vacation/opening
func (rs *Resource) setVacationOpening(w http.ResponseWriter, r *http.Request) {
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
	var req setVacationOpeningRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.RemainingDays == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("remaining_days is required")))
		return
	}
	effectiveDate, err := timezone.ParseDate(req.EffectiveDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid effective_date format, expected YYYY-MM-DD")))
		return
	}
	opening, err := rs.StaffAbsenceService.SetVacationOpening(r.Context(), staffID, decidedBy, activeSvc.SetVacationOpeningRequest{
		EffectiveDate: effectiveDate,
		RemainingDays: *req.RemainingDays,
		Note:          req.Note,
	})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderVacationOpeningError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, opening, "Vacation opening created")
}

// deleteVacationOpening handles DELETE /api/staff/{id}/vacation/opening?year=YYYY
func (rs *Resource) deleteVacationOpening(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.requireBalanceAdjustmentStaff(w, r, staffID) {
		return
	}
	year, err := parseYearQuery(r, timezone.TodayDate())
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	deletedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	if err := rs.StaffAbsenceService.DeleteVacationOpening(r.Context(), staffID, deletedBy, year); err != nil {
		tenant.MarkRollback(r.Context())
		renderVacationOpeningError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Vacation opening deleted")
}
