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
)

var errScheduleValidation = errors.New("schedule validation")

type scheduleValidationError struct {
	message string
}

func (e scheduleValidationError) Error() string {
	return e.message
}

func (e scheduleValidationError) Is(target error) bool {
	return target == errScheduleValidation
}

func scheduleValidationErrorf(format string, args ...any) error {
	return scheduleValidationError{message: fmt.Sprintf(format, args...)}
}

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
	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil {
		return false
	}
	staff, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		return false
	}
	return staff.ID == staffID
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
	mode := req.Mode
	if mode == "" {
		// Backwards compatibility: missing mode + flat entries means the
		// caller still uses the single-week, no-rotation contract.
		mode = "custom"
		if req.RotationLength == 0 {
			req.RotationLength = 1
		}
	}

	switch mode {
	case "template":
		if req.ModelID == nil || *req.ModelID == 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("model_id is required for mode=template")))
			return
		}
		if err := rs.WorkSessionService.AssignScheduleTemplate(r.Context(), staff, *req.ModelID); err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
	case "custom":
		if err := rs.applyCustomSchedule(r.Context(), staff, req); err != nil {
			if errors.Is(err, errScheduleValidation) {
				common.RenderError(w, r, common.ErrorInvalidRequest(err))
			} else {
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
	default:
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid mode %q", mode)))
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

	rows, err := rs.WorkSessionService.GetCurrentScheduleRows(ctx, staff.ID)
	if err != nil {
		return nil, fmt.Errorf("load custom schedule: %w", err)
	}

	entries, totals, rotation := scheduleRowsToResponseParts(rows)
	var earliest *timezone.Date
	for _, row := range rows {
		if earliest == nil || row.ValidFrom.Before(*earliest) {
			vf := row.ValidFrom
			earliest = &vf
		}
	}
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

// anchorString renders a rotation anchor; the zero Date (no anchor and no
// schedule rows, only possible with rotation_length 1) renders empty.
func anchorString(anchor timezone.Date) string {
	if anchor.IsZero() {
		return ""
	}
	return anchor.String()
}

func (rs *Resource) applyCustomSchedule(ctx context.Context, staff *users.Staff, req scheduleUpdateRequest) error {
	rotation := req.RotationLength
	if rotation == 0 {
		rotation = 1
	}
	if rotation < 1 || rotation > config.WorkTimeModelMaxRotation {
		return scheduleValidationErrorf("rotation_length must be between 1 and %d", config.WorkTimeModelMaxRotation)
	}

	anchor := timezone.Date{}
	if req.RotationAnchorDate != "" {
		parsed, err := timezone.ParseDate(req.RotationAnchorDate)
		if err != nil {
			return scheduleValidationErrorf("invalid rotation_anchor_date: %v", err)
		}
		anchor = parsed
	}

	entries, templateEntries, err := buildScheduleEntries(req.Entries, rotation)
	if err != nil {
		return err
	}

	if req.SaveAsTemplateName != "" {
		if err := rs.WorkSessionService.SaveCustomScheduleAsTemplate(ctx, staff, req.SaveAsTemplateName, rotation, anchor, templateEntries); err != nil {
			return fmt.Errorf("save as template: %w", err)
		}
		return nil
	}

	return rs.WorkSessionService.ApplyCustomScheduleRows(ctx, staff, entries, anchor)
}

func buildScheduleEntries(reqEntries []ScheduleEntryRequest, rotation int) ([]*config.StaffWorkSchedule, []*config.WorkTimeModelEntry, error) {
	entries := make([]*config.StaffWorkSchedule, 0, len(reqEntries))
	templateEntries := make([]*config.WorkTimeModelEntry, 0, len(reqEntries))
	seenSlots := make(map[string]struct{}, len(reqEntries))
	for _, e := range reqEntries {
		if e.TargetMinutes <= 0 {
			continue
		}
		if err := validateScheduleEntryRequest(e, rotation, seenSlots); err != nil {
			return nil, nil, err
		}
		startTime, err := parseScheduleStartTime(e.StartTime)
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, &config.StaffWorkSchedule{
			WeekIndex:      e.WeekIndex,
			RotationLength: rotation,
			DayOfWeek:      e.DayOfWeek,
			TargetMinutes:  e.TargetMinutes,
			StartTime:      startTime,
		})
		templateEntries = append(templateEntries, &config.WorkTimeModelEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     startTime,
		})
	}
	return entries, templateEntries, nil
}

func parseScheduleStartTime(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("15:04", *raw)
	if err != nil {
		return nil, scheduleValidationErrorf("start_time must be HH:MM")
	}
	wallClock := timezone.WallClock(parsed)
	return &wallClock, nil
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

func validateScheduleEntryRequest(e ScheduleEntryRequest, rotation int, seenSlots map[string]struct{}) error {
	if e.WeekIndex < 0 || e.WeekIndex >= rotation {
		return scheduleValidationErrorf("week_index %d outside rotation_length %d", e.WeekIndex, rotation)
	}
	if e.DayOfWeek < config.DayMonday || e.DayOfWeek > config.DaySunday {
		return scheduleValidationErrorf("day_of_week must be between 0 and 6")
	}
	if e.TargetMinutes > 720 {
		return scheduleValidationErrorf("target_minutes must be between 0 and 720")
	}
	slot := fmt.Sprintf("%d:%d", e.WeekIndex, e.DayOfWeek)
	if _, ok := seenSlots[slot]; ok {
		return scheduleValidationErrorf("duplicate schedule entry for week_index %d and day_of_week %d", e.WeekIndex, e.DayOfWeek)
	}
	seenSlots[slot] = struct{}{}
	return nil
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
	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil {
		return 0, fmt.Errorf("person not found for account: %w", err)
	}
	staff, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		return 0, fmt.Errorf("staff not found for editor account: %w", err)
	}
	return staff.ID, nil
}
