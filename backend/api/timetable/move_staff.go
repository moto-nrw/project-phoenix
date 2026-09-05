// Package timetable — atomic staff move between blocks (#1884).
//
//	POST /api/timetable/instances/{id}/move-staff
//
// {id} is the TARGET block gaining the person. With source_instance_id the
// person's existing same-day assignment is relocated (the "Mensa nimmt eine
// Person vom Schulhof" case); without it a free on-shift person from the pool
// is assigned. Both variants are one atomic save in the request's tenant tx,
// logged as a single staff_moved Änderungsprotokoll entry.
//
// The plan-then-write atomicity, the day lock, and every business rule live in
// InstanceService.MoveStaffBetweenBlocks (services/schedule/
// instance_move_staff.go). The handler parses, calls the service once, maps
// DeviationError onto the wire contract, attaches advisory shift-coverage
// warnings (#1873, never blocking), and fires the post-save SSE signals.
// A source-less direct request may still assign someone outside their shift or
// alongside another assignment: those remain deliberate advisory conflicts.
//
// Permission: SchedulesManage. Same tenant tx as the other /instances routes.
package timetable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// moveStaffRequest is the POST body.
type moveStaffRequest struct {
	StaffID          int64  `json:"staff_id"`
	SourceInstanceID *int64 `json:"source_instance_id,omitempty"`
}

// MoveStaffResponse is the 200 body.
type MoveStaffResponse struct {
	TargetInstanceID int64  `json:"target_instance_id"`
	SourceInstanceID *int64 `json:"source_instance_id,omitempty"`
	Action           string `json:"action"`
	// TimeConflicts lists the person's remaining same-day overlaps with the
	// target window; CoverageWarnings the Dienstplan gaps for it (#1873).
	// Both advisory — the writes have already landed in the request's tenant
	// tx and are never rolled back because of a warning.
	TimeConflicts    []scheduleSvc.SubstituteTimeConflict `json:"time_conflicts"`
	CoverageWarnings []scheduleSvc.ShiftCoverageWarning   `json:"coverage_warnings"`
}

// moveStaff handles POST /api/timetable/instances/{id}/move-staff.
func (rs *Resource) moveStaff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	var req moveStaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}

	result, err := rs.InstanceService.MoveStaffBetweenBlocks(ctx, id, scheduleSvc.MoveStaffInput{
		StaffID:          req.StaffID,
		SourceInstanceID: req.SourceInstanceID,
		ActorAccountID:   jwt.ActorAccountIDFromCtx(ctx),
	})
	if err != nil {
		renderDeviationError(w, r, err)
		return
	}

	appliedWrites := 1
	if result.Action == scheduleSvc.MoveStaffActionAlreadyApplied {
		appliedWrites = 0
	}
	rs.broadcastDeviationSaveEvents(ctx, result.ActiveTouched, appliedWrites, false, 0)
	rs.getLogger().Info("staff moved between blocks",
		slog.Int64("target_instance_id", id),
		slog.Int64("staff_id", req.StaffID),
		slog.String("action", result.Action),
	)

	resp, err := moveStaffResponseOf(rs, ctx, result, req.StaffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Staff move applied")
}

// moveStaffResponseOf shapes the service result and attaches the advisory
// shift-coverage probe for the moved person on the target window. A probe
// failure propagates as an error: the probe runs inside the request's tenant
// tx, and a PostgreSQL error aborts that tx, so the eventual commit would fail
// after the client already saw a 200 — the request must 5xx (and roll back)
// instead of reporting a move that never lands.
func moveStaffResponseOf(rs *Resource, ctx context.Context, result *scheduleSvc.MoveStaffResult, staffID int64) (MoveStaffResponse, error) {
	resp := MoveStaffResponse{
		TargetInstanceID: result.Target.ID,
		Action:           result.Action,
		TimeConflicts:    result.Warnings,
		CoverageWarnings: []scheduleSvc.ShiftCoverageWarning{},
	}
	if result.Source != nil {
		resp.SourceInstanceID = &result.Source.ID
	}
	if rs.TimetableData == nil || result.Action == scheduleSvc.MoveStaffActionAlreadyApplied {
		return resp, nil
	}
	coverage, err := rs.TimetableData.DetectShiftCoverageWarnings(ctx, scheduleSvc.ShiftCoverageQuery{
		Dates:     []timezone.Date{timezone.Date(result.Target.Date)},
		StartTime: result.Target.StartTime,
		EndTime:   result.Target.EndTime,
		StaffIDs:  []int64{staffID},
	})
	if err != nil {
		return MoveStaffResponse{}, fmt.Errorf("staff move coverage probe failed: %w", err)
	}
	if coverage.Warnings != nil {
		resp.CoverageWarnings = coverage.Warnings
	}
	return resp, nil
}
