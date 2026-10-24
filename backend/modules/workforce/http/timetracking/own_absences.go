package timetracking

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// listAbsences handles GET /api/time-tracking/absences with an overlapping
// date range, a status filter, or both.
func (rs *Resource) listAbsences(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	query := r.URL.Query()
	fromStr := query.Get("from")
	toStr := query.Get("to")
	status := strings.TrimSpace(query.Get("status"))
	if (fromStr == "") != (toStr == "") {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters must be provided together")))
		return
	}

	filter := workforce.StaffAbsenceListFilter{Status: status}
	if fromStr != "" {
		from, to, ok := parseDateRange(w, r)
		if !ok {
			return
		}
		filter.From = from.String()
		filter.To = to.String()
	} else if status == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters or status are required")))
		return
	}

	absences, err := rs.StaffAbsenceService.ListAbsences(r.Context(), staffID, filter)
	if err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, absences, "Absences retrieved successfully")
}

// createAbsence handles POST /api/time-tracking/absences
func (rs *Resource) createAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	var req workforce.CreateAbsenceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Self-service passes the caller as subject AND creator; the account id
	// makes the #1843 sick-cascade protocol entries carry a real actor.
	actorAccountID := userClaims.AccountID
	tenantID := tenant.FromContext(r.Context())
	var absence *workforce.StaffAbsenceResponse
	if err := rs.withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		var txErr error
		absence, txErr = rs.AbsenceWrites.CreateOwnAbsence(ctx, staffID, &actorAccountID, req)
		return txErr
	}); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, absence, "Absence created successfully")
}

// updateAbsence handles PUT /api/time-tracking/absences/{id}
func (rs *Resource) updateAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	absenceID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid absence ID")
	if !ok {
		return
	}

	var req workforce.UpdateAbsenceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	actorAccountID := userClaims.AccountID
	var absence *workforce.StaffAbsenceResponse
	if err := rs.withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		var txErr error
		absence, txErr = rs.AbsenceWrites.UpdateAbsence(ctx, staffID, &actorAccountID, absenceID, req)
		return txErr
	}); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, absence, "Absence updated successfully")
}

// deleteAbsence handles DELETE /api/time-tracking/absences/{id}
func (rs *Resource) deleteAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	absenceID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid absence ID")
	if !ok {
		return
	}

	// Deleting one's own sick report reverses its plan cascade (#1843); the
	// account id stamps the sick_cleared protocol entries.
	actorAccountID := userClaims.AccountID
	tenantID := tenant.FromContext(r.Context())
	if err := rs.withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		return rs.AbsenceWrites.DeleteOwnAbsence(ctx, staffID, &actorAccountID, absenceID)
	}); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.RespondNoContent(w, r)
}
