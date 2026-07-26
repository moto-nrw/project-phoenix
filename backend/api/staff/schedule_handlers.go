package staff

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// getSchedule handles GET /api/staff/{id}/schedule
func (rs *Resource) getSchedule(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.canReadSchedule(r.Context(), staffID) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("insufficient permission to read schedule")))
		return
	}

	staff, err := rs.PersonService.GetStaffByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	resp, err := rs.buildScheduleResponse(r.Context(), staff)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Schedule retrieved successfully")
}

func (rs *Resource) canReadSchedule(ctx context.Context, staffID int64) bool {
	userPermissions := jwt.PermissionsFromCtx(ctx)
	if authorize.HasPermission(permissions.TimeTrackingManage, userPermissions) {
		return true
	}
	if !authorize.HasPermission(permissions.TimeTrackingOwn, userPermissions) {
		return false
	}
	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		return false
	}
	ownStaffID, err := rs.PersonService.ResolveStaffIDByAccountID(ctx, int64(claims.ID))
	if err != nil {
		return false
	}
	return ownStaffID == staffID
}

// updateSchedule handles PUT /api/staff/{id}/schedule
func (rs *Resource) updateSchedule(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staff, err := rs.PersonService.GetStaffByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	var req scheduleUpdateRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := rs.WorkSessionService.UpdateSchedule(r.Context(), staff, req.toServiceInput()); err != nil {
		if errors.Is(err, activeSvc.ErrScheduleValidation) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	refreshed, err := rs.PersonService.GetStaffByID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp, err := rs.buildScheduleResponse(r.Context(), refreshed)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Schedule updated successfully")
}

func (rs *Resource) buildScheduleResponse(ctx context.Context, staff *users.Staff) (*ScheduleResponse, error) {
	if staff.WorkTimeModelID != nil && *staff.WorkTimeModelID > 0 {
		return rs.buildTemplateScheduleResponse(ctx, staff)
	}
	return rs.buildCustomScheduleResponse(ctx, staff)
}

func (rs *Resource) buildTemplateScheduleResponse(ctx context.Context, staff *users.Staff) (*ScheduleResponse, error) {
	model, err := rs.WorkSessionService.GetWorkTimeModelByID(ctx, *staff.WorkTimeModelID)
	if err != nil {
		return nil, fmt.Errorf("load assigned model: %w", err)
	}
	anchor := model.RotationAnchorDate
	if staff.RotationAnchorDate != nil {
		anchor = *staff.RotationAnchorDate
	}

	rows, err := rs.WorkSessionService.GetCurrentScheduleRows(ctx, staff.ID)
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
			RotationAnchorDate: model.RotationAnchorDate.String(),
		},
		RotationLength:     rotation,
		RotationAnchorDate: anchor.String(),
		Entries:            entries,
		WeeklyTotals:       totals,
	}, nil
}

func (rs *Resource) buildCustomScheduleResponse(ctx context.Context, staff *users.Staff) (*ScheduleResponse, error) {
	rows, err := rs.WorkSessionService.GetCurrentScheduleRows(ctx, staff.ID)
	if err != nil {
		return nil, fmt.Errorf("load custom schedule: %w", err)
	}

	entries, totals, rotation := scheduleRowsToResponseParts(rows)
	earliest := earliestValidFrom(rows)
	anchor := timezone.Date{}
	if staff.RotationAnchorDate != nil {
		anchor = *staff.RotationAnchorDate
	} else if earliest != nil {
		anchor = *earliest
	}
	resp := &ScheduleResponse{
		Mode:               "custom",
		RotationLength:     rotation,
		RotationAnchorDate: anchorString(anchor),
		Entries:            entries,
		WeeklyTotals:       totals,
	}
	if earliest != nil {
		resp.ValidFrom = earliest.String()
	}
	return resp, nil
}

// earliestValidFrom returns the earliest valid_from across schedule rows, or
// nil when there are none.
func earliestValidFrom(rows []*config.StaffWorkSchedule) *timezone.Date {
	var earliest *timezone.Date
	for _, row := range rows {
		if earliest == nil || row.ValidFrom.Before(*earliest) {
			vf := row.ValidFrom
			earliest = &vf
		}
	}
	return earliest
}

// anchorString renders a rotation anchor; the zero Date (no anchor and no
// schedule rows, only possible with rotation_length 1) renders empty.
func anchorString(anchor timezone.Date) string {
	if anchor.IsZero() {
		return ""
	}
	return anchor.String()
}

func formatScheduleStartTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := timezone.WallClock(*value).Format("15:04")
	return &formatted
}

func scheduleRowsToResponseParts(rows []*config.StaffWorkSchedule) ([]ScheduleEntryResponse, []int, int) {
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

func modelEntriesToResponseParts(modelEntries []*config.WorkTimeModelEntry, rotation int) ([]ScheduleEntryResponse, []int) {
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
func (rs *Resource) resolveEditorStaffID(ctx context.Context) (int64, error) {
	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		return 0, errors.New("invalid token")
	}
	return rs.PersonService.ResolveStaffIDByAccountID(ctx, int64(claims.ID))
}
