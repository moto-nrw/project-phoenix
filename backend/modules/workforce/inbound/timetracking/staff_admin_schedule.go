package timetracking

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

// getSchedule handles GET /api/staff/{id}/schedule
func (rs *StaffAdminResource) getSchedule(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.canReadSchedule(r.Context(), staffID) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("insufficient permission to read schedule")))
		return
	}

	staff, err := rs.PersonService.StaffByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	resp, err := rs.buildScheduleResponse(r.Context(), scheduleSubject{ID: staff.ID, WorkTimeModelID: staff.WorkTimeModelID, RotationAnchorDate: staff.RotationAnchorDate})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Schedule retrieved successfully")
}

func (rs *StaffAdminResource) canReadSchedule(ctx context.Context, staffID int64) bool {
	userPermissions := rs.identity(ctx).Permissions
	if common.HasPermission(permissions.TimeTrackingManage, userPermissions) {
		return true
	}
	if !common.HasPermission(permissions.TimeTrackingOwn, userPermissions) {
		return false
	}
	claims := rs.identity(ctx)
	if claims.AccountID == 0 {
		return false
	}
	ownStaffID, err := rs.PersonService.ResolveStaffIDByAccountID(ctx, claims.AccountID)
	if err != nil {
		return false
	}
	return ownStaffID == staffID
}

// updateSchedule handles PUT /api/staff/{id}/schedule
func (rs *StaffAdminResource) updateSchedule(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staff, err := rs.PersonService.StaffByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	var req scheduleUpdateRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := rs.WorkSessionService.UpdateStaffSchedule(r.Context(), staff.ID, req.toServiceInput()); err != nil {
		if errors.Is(err, workforce.ErrScheduleValidation) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	refreshed, err := rs.PersonService.StaffByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp, err := rs.buildScheduleResponse(r.Context(), scheduleSubject{ID: refreshed.ID, WorkTimeModelID: refreshed.WorkTimeModelID, RotationAnchorDate: refreshed.RotationAnchorDate})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Schedule updated successfully")
}

// scheduleSubject is the projection of a staff row the schedule responses
// need. RotationAnchorDate is a calendar day or empty.
type scheduleSubject struct {
	ID                 int64
	WorkTimeModelID    *int64
	RotationAnchorDate string
}

func (rs *StaffAdminResource) buildScheduleResponse(ctx context.Context, staff scheduleSubject) (*ScheduleResponse, error) {
	if rs.schedules == nil {
		return nil, errors.New("schedule reader is not wired")
	}
	if staff.WorkTimeModelID != nil && *staff.WorkTimeModelID > 0 {
		return rs.buildTemplateScheduleResponse(ctx, staff)
	}
	return rs.buildCustomScheduleResponse(ctx, staff)
}

func (rs *StaffAdminResource) buildTemplateScheduleResponse(ctx context.Context, staff scheduleSubject) (*ScheduleResponse, error) {
	model, err := rs.schedules.FindWorkTimeModel(ctx, *staff.WorkTimeModelID)
	if err != nil {
		return nil, fmt.Errorf("load assigned model: %w", err)
	}
	anchor := model.RotationAnchorDate
	if staff.RotationAnchorDate != "" {
		anchor = staff.RotationAnchorDate
	}

	rows, err := rs.schedules.CurrentStaffSchedule(ctx, staff.ID)
	if err != nil {
		return nil, fmt.Errorf("load assigned schedule snapshot: %w", err)
	}
	rotation := model.RotationLength
	var entries []ScheduleEntryResponse
	var totals []int
	if len(rows) > 0 {
		entries, totals, rotation = scheduleRowsToResponseParts(rows)
	} else {
		entries, totals = modelEntriesToResponseParts(model.Entries, rotation)
	}
	return &ScheduleResponse{
		Mode: "template",
		Model: &ScheduleModelInfo{
			ID:                 model.ID,
			Name:               model.Name,
			RotationLength:     model.RotationLength,
			RotationAnchorDate: model.RotationAnchorDate,
		},
		RotationLength:     rotation,
		RotationAnchorDate: anchor,
		Entries:            entries,
		WeeklyTotals:       totals,
	}, nil
}

func (rs *StaffAdminResource) buildCustomScheduleResponse(ctx context.Context, staff scheduleSubject) (*ScheduleResponse, error) {
	rows, err := rs.schedules.CurrentStaffSchedule(ctx, staff.ID)
	if err != nil {
		return nil, fmt.Errorf("load custom schedule: %w", err)
	}

	entries, totals, rotation := scheduleRowsToResponseParts(rows)
	earliest := earliestValidFrom(rows)
	// The zero anchor (no anchor and no schedule rows, only possible with
	// rotation_length 1) renders empty.
	anchor := staff.RotationAnchorDate
	if anchor == "" {
		anchor = earliest
	}
	return &ScheduleResponse{
		Mode:               "custom",
		RotationLength:     rotation,
		RotationAnchorDate: anchor,
		Entries:            entries,
		WeeklyTotals:       totals,
		ValidFrom:          earliest,
	}, nil
}

// earliestValidFrom returns the earliest valid_from across schedule rows, or
// "" when there are none. Calendar days compare lexically.
func earliestValidFrom(rows []workforce.StaffWorkSchedule) string {
	earliest := ""
	for _, row := range rows {
		if earliest == "" || row.ValidFrom < earliest {
			earliest = row.ValidFrom
		}
	}
	return earliest
}

// formatScheduleStartTime renders a wall clock ("15:04", empty when unset)
// as the nullable wire field.
func formatScheduleStartTime(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func scheduleRowsToResponseParts(rows []workforce.StaffWorkSchedule) ([]ScheduleEntryResponse, []int, int) {
	rotation := 1
	for _, row := range rows {
		if row.RotationLength > rotation {
			rotation = row.RotationLength
		}
	}
	if rotation < 1 {
		rotation = 1
	}
	totals := make([]int, rotation)
	entries := make([]ScheduleEntryResponse, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, ScheduleEntryResponse{
			WeekIndex:     row.WeekIndex,
			DayOfWeek:     row.DayOfWeek,
			TargetMinutes: row.TargetMinutes,
			StartTime:     formatScheduleStartTime(row.StartTime),
		})
		if row.WeekIndex >= 0 && row.WeekIndex < rotation {
			totals[row.WeekIndex] += row.TargetMinutes
		}
	}
	return entries, totals, rotation
}

func modelEntriesToResponseParts(modelEntries []workforce.WorkTimeModelEntry, rotation int) ([]ScheduleEntryResponse, []int) {
	if rotation < 1 {
		rotation = 1
	}
	entries := make([]ScheduleEntryResponse, 0, len(modelEntries))
	totals := make([]int, rotation)
	for _, e := range modelEntries {
		entries = append(entries, ScheduleEntryResponse{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     formatScheduleStartTime(e.StartTime),
		})
		if e.WeekIndex >= 0 && e.WeekIndex < rotation {
			totals[e.WeekIndex] += e.TargetMinutes
		}
	}
	return entries, totals
}

// resolveEditorStaffID maps the JWT account id to a staff id, the staff
// record of the admin currently making the request. Lands in
// audit.work_session_edits.edited_by so the audit trail can name a real
// person, not an opaque account.
func (rs *StaffAdminResource) resolveEditorStaffID(ctx context.Context) (int64, error) {
	claims := rs.identity(ctx)
	if claims.AccountID == 0 {
		return 0, errors.New("invalid token")
	}
	return rs.PersonService.ResolveStaffIDByAccountID(ctx, claims.AccountID)
}
