// Package timetable — Sammel-Vertretung endpoint (#2284).
//
//	POST /api/timetable/substitutions/bulk
//
// Applies ONE person's day-wide absence — optionally covered by ONE
// substitute — to a set of selected dates in a single atomic save. The
// multi-day sibling of POST /instances/{id}/deviations: same day-wide
// semantics per date, same all-or-nothing atomicity (Phase A classifies every
// day before Phase B writes a row), same DeviationError wire mapping.
//
// All business rules live in InstanceService.ApplyBulkSubstitution
// (services/schedule/bulk_substitution.go). The handler parses the body,
// calls the service once, and fires the post-save SSE signals.
//
// Permission: SchedulesManage. Same tenant tx as the other /instances routes.
package timetable

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// bulkSubstitutionRequest is the POST body. substitute_staff_id omitted/null
// marks the person absent on every selected date without assigning cover.
type bulkSubstitutionRequest struct {
	AbsentStaffID     int64    `json:"absent_staff_id"`
	SubstituteStaffID *int64   `json:"substitute_staff_id,omitempty"`
	Dates             []string `json:"dates"`
	Reason            *string  `json:"reason,omitempty"`
}

// BulkSubstitutionDayResponse is the per-day slice of the 200 body.
type BulkSubstitutionDayResponse struct {
	Date              string                               `json:"date"`
	AffectedInstances []AffectedInstance                   `json:"affected_instances"`
	Warnings          []scheduleSvc.SubstituteTimeConflict `json:"warnings"`
}

// BulkSubstitutionResponse is the 200 body.
type BulkSubstitutionResponse struct {
	Days          []BulkSubstitutionDayResponse `json:"days"`
	TotalAffected int                           `json:"total_affected"`
}

// applyBulkSubstitution handles POST /api/timetable/substitutions/bulk.
func (rs *Resource) applyBulkSubstitution(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	var req bulkSubstitutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}

	dates := make([]timezone.Date, 0, len(req.Dates))
	for _, raw := range req.Dates {
		date, err := timezone.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("dates must be YYYY-MM-DD")))
			return
		}
		dates = append(dates, date)
	}

	result, err := rs.InstanceService.ApplyBulkSubstitution(ctx, scheduleSvc.BulkSubstitutionInput{
		AbsentStaffID:     req.AbsentStaffID,
		SubstituteStaffID: req.SubstituteStaffID,
		Dates:             dates,
		Reason:            req.Reason,
		ActorAccountID:    jwt.ActorAccountIDFromCtx(ctx),
	})
	if err != nil {
		renderDeviationError(w, r, err)
		return
	}

	rs.broadcastDeviationSaveEvents(ctx, result.ActiveTouched, result.AppliedWrites, false, result.ClearedAcks)
	rs.getLogger().Info("bulk substitution applied",
		slog.Int64("absent_staff_id", req.AbsentStaffID),
		slog.Int("dates", len(result.Days)),
		slog.Int("affected_instances", result.AppliedWrites),
		slog.Bool("with_substitute", req.SubstituteStaffID != nil),
	)

	common.Respond(w, r, http.StatusOK, bulkSubstitutionResponseOf(result), "Bulk substitution applied")
}

// bulkSubstitutionResponseOf shapes the service result into the wire response,
// defaulting nil slices to empty ones so the JSON always carries arrays.
func bulkSubstitutionResponseOf(result *scheduleSvc.BulkSubstitutionResult) BulkSubstitutionResponse {
	days := make([]BulkSubstitutionDayResponse, 0, len(result.Days))
	for _, day := range result.Days {
		affected := make([]AffectedInstance, 0, len(day.Affected))
		for _, a := range day.Affected {
			affected = append(affected, AffectedInstance{
				InstanceID: a.InstanceID,
				Title:      a.Title,
				StartTime:  a.StartTime.Format("15:04"),
				Action:     a.Action,
			})
		}
		warnings := day.Warnings
		if warnings == nil {
			warnings = []scheduleSvc.SubstituteTimeConflict{}
		}
		days = append(days, BulkSubstitutionDayResponse{
			Date:              day.Date.String(),
			AffectedInstances: affected,
			Warnings:          warnings,
		})
	}
	return BulkSubstitutionResponse{Days: days, TotalAffected: result.AppliedWrites}
}
