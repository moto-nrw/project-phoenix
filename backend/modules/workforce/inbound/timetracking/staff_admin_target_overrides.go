package timetracking

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// targetOverrideRequest is the wire shape of a Sonderarbeitszeit (#3259):
// an inclusive date range and one daily target in minutes.
type targetOverrideRequest struct {
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	DailyMinutes *int   `json:"daily_minutes"`
}

func (req targetOverrideRequest) fields() (workforce.StaffTargetOverrideFields, error) {
	if req.DailyMinutes == nil {
		return workforce.StaffTargetOverrideFields{}, errors.New("daily_minutes is required")
	}
	return workforce.StaffTargetOverrideFields{StartDate: req.StartDate, EndDate: req.EndDate, DailyMinutes: *req.DailyMinutes}, nil
}

// registerTargetOverrideRoutes serves the Sonderarbeitszeiten of one staff
// member. Same tier as the schedule write: they change the Soll.
func (rs *StaffAdminResource) registerTargetOverrideRoutes(r chi.Router, withTx common.Middleware) {
	timeTracking := common.RequiresPermission(permissions.TimeTrackingManage)

	r.With(timeTracking, withTx).Get("/{id}/target-overrides", rs.listTargetOverrides)
	r.With(timeTracking, withTx).Post("/{id}/target-overrides", rs.createTargetOverride)
	r.With(timeTracking, withTx).Delete("/{id}/target-overrides/{overrideId}", rs.deleteTargetOverride)
}

// targetOverrideStaff parses the staff ID and proves the staff member exists
// in the caller's tenant before any override is read or written.
func (rs *StaffAdminResource) targetOverrideStaff(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if rs.targetOverrides == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("target overrides are not wired")))
		return 0, false
	}
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return 0, false
	}
	if _, err := rs.PersonService.StaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return 0, false
	}
	return staffID, true
}

// listTargetOverrides handles GET /api/staff/{id}/target-overrides
func (rs *StaffAdminResource) listTargetOverrides(w http.ResponseWriter, r *http.Request) {
	staffID, ok := rs.targetOverrideStaff(w, r)
	if !ok {
		return
	}
	overrides, err := rs.targetOverrides.ListStaffTargetOverrides(r.Context(), staffID)
	if err != nil {
		renderTargetOverrideError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overrides, "Target overrides retrieved successfully")
}

// createTargetOverride handles POST /api/staff/{id}/target-overrides
func (rs *StaffAdminResource) createTargetOverride(w http.ResponseWriter, r *http.Request) {
	staffID, ok := rs.targetOverrideStaff(w, r)
	if !ok {
		return
	}
	fields, ok := decodeTargetOverride(w, r)
	if !ok {
		return
	}
	var createdBy *int64
	if editorID, err := rs.resolveEditorStaffID(r.Context()); err == nil {
		createdBy = &editorID
	}
	override, err := rs.targetOverrides.CreateStaffTargetOverride(r.Context(), staffID, fields, createdBy)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderTargetOverrideError(w, r, err)
		return
	}
	rs.notifyChanged(r)
	common.Respond(w, r, http.StatusCreated, override, "Target override created")
}

// deleteTargetOverride handles DELETE /api/staff/{id}/target-overrides/{overrideId}
func (rs *StaffAdminResource) deleteTargetOverride(w http.ResponseWriter, r *http.Request) {
	staffID, ok := rs.targetOverrideStaff(w, r)
	if !ok {
		return
	}
	overrideID, err := parseInt64Param(r, "overrideId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.targetOverrides.DeleteStaffTargetOverride(r.Context(), staffID, overrideID); err != nil {
		tenant.MarkRollback(r.Context())
		renderTargetOverrideError(w, r, err)
		return
	}
	rs.notifyChanged(r)
	common.Respond(w, r, http.StatusOK, nil, "Target override deleted")
}

func decodeTargetOverride(w http.ResponseWriter, r *http.Request) (workforce.StaffTargetOverrideFields, bool) {
	var req targetOverrideRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return workforce.StaffTargetOverrideFields{}, false
	}
	fields, err := req.fields()
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return workforce.StaffTargetOverrideFields{}, false
	}
	return fields, true
}

// notifyChanged invalidates the time-account views after the write commits.
func (rs *StaffAdminResource) notifyChanged(r *http.Request) {
	if rs.notifyTimeTracking != nil {
		rs.notifyTimeTracking(r.Context())
	}
}

func renderTargetOverrideError(w http.ResponseWriter, r *http.Request, err error) {
	var typed *workforce.TargetOverrideError
	switch {
	case errors.Is(err, workforce.ErrStaffTargetOverrideNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.As(err, &typed) && errors.Is(err, workforce.ErrStaffTargetOverrideRejected):
		common.RenderError(w, r, common.ErrorConflictMessage(typed.Reason))
	case errors.As(err, &typed):
		common.RenderError(w, r, common.ErrorInvalidRequestMessage(typed.Reason))
	case errors.Is(err, workforce.ErrInvalidWorkTime):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
