package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// workSessionCapability embeds the service so GetTodayPresenceMap, whose
// signature already matches, is promoted; the methods below convert values
// and classify failures through sessions, the same service.
type workSessionCapability struct {
	WorkSessionService
	sessions WorkSessionService
	staff    func(context.Context, int64) (StaffScheduleBinding, error)
}

// WorkSessionCapability serves workforce.WorkSessions from the work session
// service; the lookup resolves the staff record a schedule update is written
// for.
func WorkSessionCapability(sessions WorkSessionService, staff func(context.Context, int64) (StaffScheduleBinding, error)) workforce.WorkSessions {
	if sessions == nil || staff == nil {
		panic("work session capability: work session service and staff lookup are required")
	}
	return workSessionCapability{WorkSessionService: sessions, sessions: sessions, staff: staff}
}

func publicWorkSession(entity *WorkSession) *workforce.WorkSession {
	if entity == nil {
		return nil
	}
	return &workforce.WorkSession{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Date: entity.Date.String(), Status: entity.Status,
		Source: entity.Source, CheckInTime: entity.CheckInTime, CheckOutTime: entity.CheckOutTime, ReopenedAt: entity.ReopenedAt,
		BreakMinutes: entity.BreakMinutes, Notes: entity.Notes, AutoCheckedOut: entity.AutoCheckedOut, CreatedBy: entity.CreatedBy,
		UpdatedBy: entity.UpdatedBy, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func publicWorkSessionBreak(entity *WorkSessionBreak) *workforce.WorkSessionBreak {
	if entity == nil {
		return nil
	}
	return &workforce.WorkSessionBreak{
		ID: entity.ID, TenantID: entity.TenantID, SessionID: entity.SessionID, StartedAt: entity.StartedAt, EndedAt: entity.EndedAt,
		DurationMinutes: entity.DurationMinutes, PlannedEndTime: entity.PlannedEndTime, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func publicWorkSessionBreaks(entities []*WorkSessionBreak) []*workforce.WorkSessionBreak {
	if entities == nil {
		return nil
	}
	result := make([]*workforce.WorkSessionBreak, 0, len(entities))
	for _, entity := range entities {
		result = append(result, publicWorkSessionBreak(entity))
	}
	return result
}

func publicSessionResponse(value *SessionResponse) *workforce.SessionResponse {
	if value == nil {
		return nil
	}
	return &workforce.SessionResponse{
		WorkSession: publicWorkSession(value.WorkSession), BreakMinutes: value.BreakMinutes, NetMinutes: value.NetMinutes,
		IsOvertime: value.IsOvertime, IsBreakCompliant: value.IsBreakCompliant, Breaks: publicWorkSessionBreaks(value.Breaks),
		EditCount: value.EditCount, AuditCount: value.AuditCount,
	}
}

func publicHistory(value *HistoryResponse) *workforce.HistoryResponse {
	if value == nil {
		return nil
	}
	result := &workforce.HistoryResponse{}
	if value.Sessions != nil {
		result.Sessions = make([]*workforce.SessionResponse, 0, len(value.Sessions))
		for _, session := range value.Sessions {
			result.Sessions = append(result.Sessions, publicSessionResponse(session))
		}
	}
	if value.WeeklySummaries != nil {
		result.WeeklySummaries = make([]workforce.WorkWeekSummary, 0, len(value.WeeklySummaries))
		for _, week := range value.WeeklySummaries {
			result.WeeklySummaries = append(result.WeeklySummaries, workforce.WorkWeekSummary(week))
		}
	}
	if value.LaborDays != nil {
		result.LaborDays = make([]workforce.WorkDayLaborHint, 0, len(value.LaborDays))
		for _, day := range value.LaborDays {
			result.LaborDays = append(result.LaborDays, publicWorkDayLaborHint(day))
		}
	}
	return result
}

func publicWorkDayLaborHint(day WorkDayLaborHint) workforce.WorkDayLaborHint {
	return workforce.WorkDayLaborHint{
		Date: day.Date.String(), NetMinutes: day.NetMinutes, BreakMinutes: day.BreakMinutes,
		CountedBreakMinutes: day.CountedBreakMinutes, RequiredBreakMinutes: day.RequiredBreakMinutes,
		IsBreakTooShort: day.IsBreakTooShort, IsOverDailyMax: day.IsOverDailyMax, IsRunning: day.IsRunning,
		OverDailyMaxAt: day.OverDailyMaxAt, BreakTooShortAt: day.BreakTooShortAt,
		BreakTooShortRequiredMinutes: day.BreakTooShortRequiredMinutes,
	}
}

func publicWorkSessionEdit(entity *WorkSessionEdit) *workforce.WorkSessionEdit {
	if entity == nil {
		return nil
	}
	return &workforce.WorkSessionEdit{
		ID: entity.ID, TenantID: entity.TenantID, SessionID: entity.SessionID, StaffID: entity.StaffID, EditedBy: entity.EditedBy,
		FieldName: entity.FieldName, OldValue: entity.OldValue, NewValue: entity.NewValue, Notes: entity.Notes, CreatedAt: entity.CreatedAt,
	}
}

func publicWorkSessionEdits(values []*WorkSessionEditView) []*workforce.WorkSessionEditView {
	if values == nil {
		return nil
	}
	result := make([]*workforce.WorkSessionEditView, 0, len(values))
	for _, value := range values {
		if value == nil {
			result = append(result, nil)
			continue
		}
		result = append(result, &workforce.WorkSessionEditView{
			WorkSessionEdit: publicWorkSessionEdit(value.WorkSessionEdit), EditorName: value.EditorName, IsSelfEdit: value.IsSelfEdit,
		})
	}
	return result
}

func legacySessionUpdate(value workforce.SessionUpdateRequest) SessionUpdateRequest {
	result := SessionUpdateRequest{
		Date: value.Date, CheckInTime: value.CheckInTime, CheckOutTime: value.CheckOutTime, BreakMinutes: value.BreakMinutes,
		Status: value.Status, Notes: value.Notes,
	}
	if value.Breaks != nil {
		result.Breaks = make([]BreakDurationUpdate, 0, len(value.Breaks))
		for _, update := range value.Breaks {
			result.Breaks = append(result.Breaks, BreakDurationUpdate(update))
		}
	}
	return result
}

func publicExportFile(value *ExportFile) *workforce.ExportFile {
	if value == nil {
		return nil
	}
	return &workforce.ExportFile{Data: value.Data, Filename: value.Filename, ContentType: value.ContentType}
}

// AutoCheckIn serves Student Presence's auto-check-in from the work session
// service.
type AutoCheckIn struct {
	sessions WorkSessionService
}

// AutoCheckInCapability serves Student Presence's auto-check-in from the work
// session service. The error passes unchanged: Presence recognises the
// planned-start refusal by its shape.
func AutoCheckInCapability(sessions WorkSessionService) AutoCheckIn {
	if sessions == nil {
		panic("auto check-in capability: work session service is required")
	}
	return AutoCheckIn{sessions: sessions}
}

func (a AutoCheckIn) EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*workforce.WorkSession, error) {
	session, err := a.sessions.EnsureCheckedIn(ctx, staffID, source)
	if err != nil {
		return nil, err
	}
	return publicWorkSession(session), nil
}

func (c workSessionCapability) CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*workforce.WorkSession, error) {
	session, err := c.sessions.CheckIn(ctx, staffID, status, source, reason)
	if err != nil {
		return nil, classifyCheckInError(ctx, err)
	}
	return publicWorkSession(session), nil
}

func (c workSessionCapability) CheckOut(ctx context.Context, staffID int64, reason string) (*workforce.WorkSession, error) {
	session, err := c.sessions.CheckOut(ctx, staffID, reason)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSession(session), nil
}

func (c workSessionCapability) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*workforce.WorkSessionBreak, error) {
	workBreak, err := c.sessions.StartBreak(ctx, staffID, plannedDurationMinutes)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSessionBreak(workBreak), nil
}

func (c workSessionCapability) EndBreak(ctx context.Context, staffID int64) (*workforce.WorkSession, error) {
	session, err := c.sessions.EndBreak(ctx, staffID)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSession(session), nil
}

func (c workSessionCapability) LatestOpenSession(ctx context.Context, staffID int64) (*workforce.WorkSession, error) {
	session, err := c.sessions.GetLatestOpenSession(ctx, staffID)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSession(session), nil
}

func (c workSessionCapability) SessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionBreak, error) {
	breaks, err := c.sessions.GetSessionBreaks(ctx, staffID, sessionID)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSessionBreaks(breaks), nil
}

func (c workSessionCapability) UpdateSession(ctx context.Context, staffID, sessionID int64, updates workforce.SessionUpdateRequest) (*workforce.WorkSession, error) {
	session, err := c.sessions.UpdateSession(ctx, staffID, sessionID, legacySessionUpdate(updates))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSession(session), nil
}

func (c workSessionCapability) UpdateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID, sessionID int64, updates workforce.SessionUpdateRequest) (*workforce.WorkSession, error) {
	session, err := c.sessions.UpdateSessionAsAdmin(ctx, editorStaffID, targetStaffID, sessionID, legacySessionUpdate(updates))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSession(session), nil
}

func (c workSessionCapability) CreateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID int64, request workforce.AdminCreateSessionRequest) (*workforce.WorkSession, error) {
	session, err := c.sessions.CreateSessionAsAdmin(ctx, editorStaffID, targetStaffID, AdminCreateSessionRequest(request))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSession(session), nil
}

func (c workSessionCapability) HistoryIntersecting(ctx context.Context, staffID int64, from, to string) (*workforce.HistoryResponse, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	history, err := c.sessions.GetHistoryIntersecting(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicHistory(history), nil
}

func (c workSessionCapability) SessionEdits(ctx context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionEditView, error) {
	edits, err := c.sessions.GetSessionEdits(ctx, staffID, sessionID)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSessionEdits(edits), nil
}

func (c workSessionCapability) SessionEditsForStaff(ctx context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionEditView, error) {
	edits, err := c.sessions.GetSessionEditsForStaff(ctx, staffID, sessionID)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicWorkSessionEdits(edits), nil
}

func (c workSessionCapability) ExportSessions(ctx context.Context, staffID int64, from, to, format string) (*workforce.ExportFile, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	file, err := c.sessions.ExportSessions(ctx, staffID, fromDate, toDate, format)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicExportFile(file), nil
}

func (c workSessionCapability) UpdateStaffSchedule(ctx context.Context, staffID int64, input workforce.ScheduleUpdateInput) error {
	// A template keeps the version it assigns starting today (#3892).
	if input.ValidFrom != "" && (input.Mode == "template" || input.SaveAsTemplateName != "") {
		return fmt.Errorf("%w: valid_from is only supported for a custom schedule", workforce.ErrScheduleValidation)
	}
	binding, err := c.staff(ctx, staffID)
	if err != nil {
		return err
	}
	return mapTimeTrackingFailure(c.sessions.UpdateSchedule(ctx, &binding, input))
}
