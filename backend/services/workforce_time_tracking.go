package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	shiftplanning "github.com/moto-nrw/project-phoenix/modules/workforce/legacy/shiftplanning"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// The adapters in this file serve the public Workforce time-tracking
// contracts from the retained work session, absence, month, ledger,
// month-close, overview, audit-log and export services (#2690). Every wire
// shape is copied field by field; every sentinel failure of the retained
// service is reported as the matching public kind with its wording intact.

// timeTrackingSentinels pairs each retained sentinel with its public kind.
var timeTrackingSentinels = []struct {
	legacy error
	kind   error
}{
	{timetracking.ErrManagerControlledAbsence, workforce.ErrManagerControlledAbsence},
	{timetracking.ErrAllowanceBookingOverlap, workforce.ErrAllowanceBookingOverlap},
	{timetracking.ErrAbsenceRebookingBlocked, workforce.ErrAbsenceRebookingBlocked},
	{timetracking.ErrAbsenceTypeInactive, workforce.ErrAbsenceTypeInactive},
	{timetracking.ErrAbsenceTypeNotFound, workforce.ErrAbsenceTypeNotFound},
	{timetracking.ErrAbsenceTypeAllowanceExceeded, workforce.ErrAbsenceTypeAllowanceExceeded},
	{timetracking.ErrAbsenceTypeAllowanceInvalid, workforce.ErrAbsenceTypeAllowanceInvalid},
	{timetracking.ErrVacationQuotaInvalid, workforce.ErrVacationQuotaInvalid},
	{timetracking.ErrVacationQuotaExceeded, workforce.ErrVacationQuotaExceeded},
	{timetracking.ErrAdjustmentInvalid, workforce.ErrAdjustmentInvalid},
	{timetracking.ErrAdjustmentNotFound, workforce.ErrAdjustmentNotFound},
	{timetracking.ErrAdjustmentExceedsBalance, workforce.ErrAdjustmentExceedsBalance},
	{timetracking.ErrAdjustmentInClosedMonth, workforce.ErrAdjustmentInClosedMonth},
	{timetracking.ErrAdjustmentHasDependentReset, workforce.ErrAdjustmentHasDependentReset},
	{timetracking.ErrBalanceAlreadyReset, workforce.ErrBalanceAlreadyReset},
	{timetracking.ErrOpeningAlreadyExists, workforce.ErrOpeningAlreadyExists},
	{timetracking.ErrMonthCloseInvalid, workforce.ErrMonthCloseInvalid},
	{timetracking.ErrMonthNotClosable, workforce.ErrMonthNotClosable},
	{timetracking.ErrMonthNotClosed, workforce.ErrMonthNotClosed},
	{timetracking.ErrLaterMonthClosed, workforce.ErrLaterMonthClosed},
	{timetracking.ErrMonthOutOfRange, workforce.ErrMonthOutOfRange},
	{timetracking.ErrVacationOpeningInvalid, workforce.ErrVacationOpeningInvalid},
	{timetracking.ErrVacationOpeningNotFound, workforce.ErrVacationOpeningNotFound},
	{timetracking.ErrVacationOpeningExists, workforce.ErrVacationOpeningExists},
	{timetracking.ErrVacationOpeningAbsencesBeforeCutoff, workforce.ErrVacationOpeningAbsencesBeforeCutoff},
	{timetracking.ErrInvalidTargetRange, workforce.ErrInvalidTargetRange},
	{timetracking.ErrOverviewInvalid, workforce.ErrOverviewInvalid},
	{timetracking.ErrAuditLogInvalid, workforce.ErrAuditLogInvalid},
	{timetracking.ErrTimeExportInvalid, workforce.ErrTimeExportInvalid},
	{timetracking.ErrPayrollConfigIncomplete, workforce.ErrPayrollConfigIncomplete},
	{timetracking.ErrScheduleValidation, workforce.ErrScheduleValidation},
}

// mapTimeTrackingFailure reports a retained sentinel as its public kind and
// otherwise falls back to the stamp classification; the cause and its wording
// always survive.
func mapTimeTrackingFailure(err error) error {
	if err == nil {
		return nil
	}
	// The #1843 sick cascade wraps the planning layer's overlap: a shift whose
	// freed window was re-planned collides when the report is reversed.
	if errors.Is(err, shiftplanning.ErrShiftOverlap) {
		return &workforce.TimeTrackingError{Kind: workforce.ErrStaffShiftOverlap, Cause: err}
	}
	for _, pair := range timeTrackingSentinels {
		if errors.Is(err, pair.legacy) {
			return &workforce.TimeTrackingError{Kind: pair.kind, Cause: err}
		}
	}
	return MapTimeTrackingError(err)
}

// parseCapabilityDate turns a public YYYY-MM-DD day into the calendar type
// the retained services take; a malformed day is an invalid request.
func parseCapabilityDate(value, field string) (timezone.Date, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return timezone.Date(""), &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: fmt.Errorf("%s must be YYYY-MM-DD", field)}
	}
	return date, nil
}

// parseCapabilityRange parses the from/to pair every range read takes.
func parseCapabilityRange(from, to string) (timezone.Date, timezone.Date, error) {
	fromDate, err := parseCapabilityDate(from, "from")
	if err != nil {
		return timezone.Date(""), timezone.Date(""), err
	}
	toDate, err := parseCapabilityDate(to, "to")
	if err != nil {
		return timezone.Date(""), timezone.Date(""), err
	}
	return fromDate, toDate, nil
}

func parseOptionalCapabilityDate(value, field string) (*timezone.Date, error) {
	if value == "" {
		return nil, nil
	}
	date, err := parseCapabilityDate(value, field)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func publicOptionalDate(value *timezone.Date) string {
	if value == nil {
		return ""
	}
	return value.String()
}

// --- work sessions ----------------------------------------------------------

type workSessionCapability struct {
	sessions timetracking.WorkSessionService
	people   users.PersonService
}

// WorkSessionCapability serves workforce.WorkSessions from the retained work
// session service; the people service resolves the staff record a schedule
// update is written for.
func WorkSessionCapability(sessions timetracking.WorkSessionService, people users.PersonService) workforce.WorkSessions {
	if sessions == nil || people == nil {
		panic("work session capability: work session and person services are required")
	}
	return workSessionCapability{sessions: sessions, people: people}
}

func publicWorkSession(entity *activeModels.WorkSession) *workforce.WorkSession {
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

func publicWorkSessionBreak(entity *activeModels.WorkSessionBreak) *workforce.WorkSessionBreak {
	if entity == nil {
		return nil
	}
	return &workforce.WorkSessionBreak{
		ID: entity.ID, TenantID: entity.TenantID, SessionID: entity.SessionID, StartedAt: entity.StartedAt, EndedAt: entity.EndedAt,
		DurationMinutes: entity.DurationMinutes, PlannedEndTime: entity.PlannedEndTime, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func publicWorkSessionBreaks(entities []*activeModels.WorkSessionBreak) []*workforce.WorkSessionBreak {
	if entities == nil {
		return nil
	}
	result := make([]*workforce.WorkSessionBreak, 0, len(entities))
	for _, entity := range entities {
		result = append(result, publicWorkSessionBreak(entity))
	}
	return result
}

func publicSessionResponse(value *timetracking.SessionResponse) *workforce.SessionResponse {
	if value == nil {
		return nil
	}
	return &workforce.SessionResponse{
		WorkSession: publicWorkSession(value.WorkSession), BreakMinutes: value.BreakMinutes, NetMinutes: value.NetMinutes,
		IsOvertime: value.IsOvertime, IsBreakCompliant: value.IsBreakCompliant, Breaks: publicWorkSessionBreaks(value.Breaks),
		EditCount: value.EditCount, AuditCount: value.AuditCount,
	}
}

func publicHistory(value *timetracking.HistoryResponse) *workforce.HistoryResponse {
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
	return result
}

func publicWorkSessionEdit(entity *timetracking.WorkSessionEdit) *workforce.WorkSessionEdit {
	if entity == nil {
		return nil
	}
	return &workforce.WorkSessionEdit{
		ID: entity.ID, TenantID: entity.TenantID, SessionID: entity.SessionID, StaffID: entity.StaffID, EditedBy: entity.EditedBy,
		FieldName: entity.FieldName, OldValue: entity.OldValue, NewValue: entity.NewValue, Notes: entity.Notes, CreatedAt: entity.CreatedAt,
	}
}

func publicWorkSessionEdits(values []*timetracking.WorkSessionEditView) []*workforce.WorkSessionEditView {
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

func legacySessionUpdate(value workforce.SessionUpdateRequest) timetracking.SessionUpdateRequest {
	result := timetracking.SessionUpdateRequest{
		Date: value.Date, CheckInTime: value.CheckInTime, CheckOutTime: value.CheckOutTime, BreakMinutes: value.BreakMinutes,
		Status: value.Status, Notes: value.Notes,
	}
	if value.Breaks != nil {
		result.Breaks = make([]timetracking.BreakDurationUpdate, 0, len(value.Breaks))
		for _, update := range value.Breaks {
			result.Breaks = append(result.Breaks, timetracking.BreakDurationUpdate(update))
		}
	}
	return result
}

func publicExportFile(value *timetracking.ExportFile) *workforce.ExportFile {
	if value == nil {
		return nil
	}
	return &workforce.ExportFile{Data: value.Data, Filename: value.Filename, ContentType: value.ContentType}
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
	session, err := c.sessions.CreateSessionAsAdmin(ctx, editorStaffID, targetStaffID, timetracking.AdminCreateSessionRequest(request))
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

func (c workSessionCapability) TodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	return c.sessions.GetTodayPresenceMap(ctx)
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
	staff, err := c.people.GetStaffByID(ctx, staffID)
	if err != nil {
		return err
	}
	legacyInput := timetracking.ScheduleUpdateInput{
		Mode: input.Mode, ModelID: input.ModelID, RotationLength: input.RotationLength,
		RotationAnchorDate: input.RotationAnchorDate, SaveAsTemplateName: input.SaveAsTemplateName,
	}
	if input.Entries != nil {
		legacyInput.Entries = make([]timetracking.ScheduleEntry, 0, len(input.Entries))
		for _, entry := range input.Entries {
			legacyInput.Entries = append(legacyInput.Entries, timetracking.ScheduleEntry(entry))
		}
	}
	return mapTimeTrackingFailure(c.sessions.UpdateSchedule(ctx, &timetracking.StaffScheduleBinding{ID: staff.ID, WorkTimeModelID: staff.WorkTimeModelID, RotationAnchorDate: staff.RotationAnchorDate}, legacyInput))
}

// --- absences ---------------------------------------------------------------

type staffAbsenceCapability struct {
	absences timetracking.StaffAbsenceService
}

// StaffAbsenceCapability serves workforce.StaffAbsences from the retained
// absence service.
func StaffAbsenceCapability(absences timetracking.StaffAbsenceService) workforce.StaffAbsences {
	if absences == nil {
		panic("staff absence capability: absence service is required")
	}
	return staffAbsenceCapability{absences: absences}
}

func publicStaffAbsence(entity *activeModels.StaffAbsence) *workforce.StaffAbsence {
	if entity == nil {
		return nil
	}
	return &workforce.StaffAbsence{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, AbsenceType: entity.AbsenceType, AbsenceTypeID: entity.AbsenceTypeID,
		AbsenceTypeLabel: entity.AbsenceTypeLabel, DateStart: entity.DateStart.String(), DateEnd: entity.DateEnd.String(), HalfDay: entity.HalfDay,
		StartHalfDay: entity.StartHalfDay, EndHalfDay: entity.EndHalfDay, Note: entity.Note, Status: entity.Status, ApprovedBy: entity.ApprovedBy,
		ApprovedAt: entity.ApprovedAt, CreatedBy: entity.CreatedBy, WorkingDays: entity.WorkingDays, DecisionNote: entity.DecisionNote,
		RequestedAt: entity.RequestedAt, SubstituteStaffID: entity.SubstituteStaffID, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func publicAbsenceResponse(value *timetracking.StaffAbsenceResponse) *workforce.StaffAbsenceResponse {
	if value == nil {
		return nil
	}
	return &workforce.StaffAbsenceResponse{
		StaffAbsence: publicStaffAbsence(value.StaffAbsence), DurationDays: value.DurationDays, AbsenceTypeID: value.AbsenceTypeID,
	}
}

func publicAbsenceResponses(values []*timetracking.StaffAbsenceResponse) []*workforce.StaffAbsenceResponse {
	if values == nil {
		return nil
	}
	result := make([]*workforce.StaffAbsenceResponse, 0, len(values))
	for _, value := range values {
		result = append(result, publicAbsenceResponse(value))
	}
	return result
}

func publicVacationOpening(entity *activeModels.StaffVacationOpening) *workforce.StaffVacationOpening {
	if entity == nil {
		return nil
	}
	return &workforce.StaffVacationOpening{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Year: entity.Year, EffectiveDate: entity.EffectiveDate.String(),
		TakenBeforeDays: entity.TakenBeforeDays, EnteredRemainingDays: entity.EnteredRemainingDays, Note: entity.Note,
		DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func (c staffAbsenceCapability) absence(value *timetracking.StaffAbsenceResponse, err error) (*workforce.StaffAbsenceResponse, error) {
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicAbsenceResponse(value), nil
}

func (c staffAbsenceCapability) ListAbsences(ctx context.Context, staffID int64, filter workforce.StaffAbsenceListFilter) ([]*workforce.StaffAbsenceResponse, error) {
	from, err := parseOptionalCapabilityDate(filter.From, "from")
	if err != nil {
		return nil, err
	}
	to, err := parseOptionalCapabilityDate(filter.To, "to")
	if err != nil {
		return nil, err
	}
	values, err := c.absences.ListAbsences(ctx, staffID, timetracking.StaffAbsenceListFilter{From: from, To: to, Status: filter.Status})
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicAbsenceResponses(values), nil
}

func (c staffAbsenceCapability) AbsencesForRange(ctx context.Context, staffID int64, from, to string) ([]*workforce.StaffAbsenceResponse, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	values, err := c.absences.GetAbsencesForRange(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicAbsenceResponses(values), nil
}

func (c staffAbsenceCapability) CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.CreateOwnAbsence(ctx, staffID, actorAccountID, timetracking.CreateAbsenceRequest(request)))
}

func (c staffAbsenceCapability) CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.CreateAbsenceFor(ctx, subjectStaffID, createdByStaffID, actorAccountID, timetracking.CreateAbsenceRequest(request)))
}

func (c staffAbsenceCapability) UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, request workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.UpdateAbsence(ctx, staffID, actorAccountID, absenceID, timetracking.UpdateAbsenceRequest(request)))
}

func (c staffAbsenceCapability) DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) error {
	return mapTimeTrackingFailure(c.absences.DeleteOwnAbsence(ctx, staffID, actorAccountID, absenceID))
}

func (c staffAbsenceCapability) DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) error {
	return mapTimeTrackingFailure(c.absences.DeleteAbsenceFor(ctx, subjectStaffID, actorStaffID, actorAccountID, absenceID))
}

func (c staffAbsenceCapability) PreviewCompTimeBalance(ctx context.Context, staffID int64, start, end string, halfDay bool) (*workforce.CompTimeBalancePreview, error) {
	startDate, err := parseCapabilityDate(start, "start")
	if err != nil {
		return nil, err
	}
	endDate, err := parseCapabilityDate(end, "end")
	if err != nil {
		return nil, err
	}
	preview, err := c.absences.PreviewCompTimeBalance(ctx, staffID, startDate, endDate, halfDay)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if preview == nil {
		return nil, nil
	}
	return new(workforce.CompTimeBalancePreview(*preview)), nil
}

func (c staffAbsenceCapability) RebookAbsences(ctx context.Context, staffID, actorAccountID int64, request workforce.RebookAbsencesRequest) (*workforce.AbsenceRebookingResult, error) {
	value, err := c.absences.RebookAbsences(ctx, timetracking.RebookAbsencesRequest{
		StaffID: staffID, ActorAccountID: actorAccountID, AbsenceIDs: request.AbsenceIDs,
		AbsenceType: request.AbsenceType, AbsenceTypeID: request.AbsenceTypeID,
		Reason: request.Reason, DryRun: request.DryRun,
	})
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	result := &workforce.AbsenceRebookingResult{
		Absences: publicAbsenceResponses(value.Absences), Days: value.Days,
		BalanceDeltaMinutes: value.BalanceDeltaMinutes, AllowanceExceeded: value.AllowanceExceeded,
		VacationExceeded: value.VacationExceeded, Applied: value.Applied,
	}
	for _, allowance := range value.Allowances {
		result.Allowances = append(result.Allowances, workforce.RebookingAllowanceYear{
			Year: allowance.Year, EntitledDays: allowance.EntitledDays, TakenDays: allowance.TakenDays,
			ReservedDays: allowance.ReservedDays, RemainingDays: allowance.RemainingDays, BookingDays: allowance.BookingDays,
		})
	}
	for _, year := range value.Vacation {
		result.Vacation = append(result.Vacation, workforce.RebookingVacationYear(year))
	}
	return result, nil
}

func (c staffAbsenceCapability) RequestVacation(ctx context.Context, staffID int64, request workforce.RequestVacationRequest) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.RequestVacation(ctx, staffID, timetracking.RequestVacationRequest(request)))
}

func (c staffAbsenceCapability) CancelAbsence(ctx context.Context, staffID, actorAccountID, absenceID int64) error {
	return mapTimeTrackingFailure(c.absences.CancelAbsence(ctx, staffID, actorAccountID, absenceID))
}

func (c staffAbsenceCapability) ResubmitAbsence(ctx context.Context, staffID, actorAccountID, absenceID int64, note string) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.ResubmitAbsence(ctx, staffID, actorAccountID, absenceID, note))
}

func (c staffAbsenceCapability) ApproveAbsence(ctx context.Context, absenceID, actorAccountID, decidedByStaffID int64, note string) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.ApproveAbsence(ctx, absenceID, actorAccountID, decidedByStaffID, note))
}

func (c staffAbsenceCapability) DenyAbsence(ctx context.Context, absenceID, actorAccountID, decidedByStaffID int64, reason string) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.DenyAbsence(ctx, absenceID, actorAccountID, decidedByStaffID, reason))
}

func (c staffAbsenceCapability) QuestionAbsence(ctx context.Context, absenceID, actorAccountID int64, note string) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.QuestionAbsence(ctx, absenceID, actorAccountID, note))
}

func (c staffAbsenceCapability) ListPendingRequests(ctx context.Context) ([]*workforce.StaffAbsenceResponse, error) {
	values, err := c.absences.ListPendingRequests(ctx)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicAbsenceResponses(values), nil
}

func (c staffAbsenceCapability) ListAbsenceRequests(ctx context.Context, query workforce.AbsenceRequestListQuery) ([]*workforce.StaffAbsenceRequestItem, error) {
	values, err := c.absences.ListAbsenceRequests(ctx, timetracking.AbsenceRequestListQuery(query))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if values == nil {
		return nil, nil
	}
	result := make([]*workforce.StaffAbsenceRequestItem, 0, len(values))
	for _, value := range values {
		if value == nil {
			result = append(result, nil)
			continue
		}
		result = append(result, &workforce.StaffAbsenceRequestItem{
			StaffAbsenceResponse: publicAbsenceResponse(value.StaffAbsenceResponse), StaffName: value.StaffName, DecidedByName: value.DecidedByName,
		})
	}
	return result, nil
}

func (c staffAbsenceCapability) VacationQuotaSummary(ctx context.Context, staffID int64, year int) (*workforce.VacationQuotaSummary, error) {
	summary, err := c.absences.GetVacationQuotaSummary(ctx, staffID, year)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if summary == nil {
		return nil, nil
	}
	return &workforce.VacationQuotaSummary{
		StaffID: summary.StaffID, Year: summary.Year, EntitledDays: summary.EntitledDays, CarryoverDays: summary.CarryoverDays,
		TakenBeforeDays: summary.TakenBeforeDays, TakenDays: summary.TakenDays, ReservedDays: summary.ReservedDays,
		RemainingDays: summary.RemainingDays, Opening: publicVacationOpening(summary.Opening),
	}, nil
}

func (c staffAbsenceCapability) UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error {
	return mapTimeTrackingFailure(c.absences.UpsertVacationQuota(ctx, staffID, year, entitled, carryover))
}

func (c staffAbsenceCapability) SetVacationQuota(ctx context.Context, change workforce.VacationQuotaChange) error {
	return mapTimeTrackingFailure(c.absences.SetVacationQuota(ctx, timetracking.VacationQuotaChange(change)))
}

// VacationTakeoverCapability binds the import to the public Workforce contract.
func VacationTakeoverCapability(absences timetracking.StaffAbsenceService) workforce.VacationTakeovers {
	if absences == nil {
		return nil
	}
	return staffAbsenceCapability{absences: absences}
}

func (c staffAbsenceCapability) ValidateVacationOpeningAbsencesBefore(ctx context.Context, staffID int64, effectiveDate string) error {
	effective, err := parseCapabilityDate(effectiveDate, "effective_date")
	if err != nil {
		return err
	}
	return mapTimeTrackingFailure(c.absences.ValidateVacationOpeningAbsencesBefore(ctx, staffID, effective))
}

func (c staffAbsenceCapability) SetVacationOpening(ctx context.Context, staffID, decidedBy int64, request workforce.SetVacationOpeningRequest) (*workforce.StaffVacationOpening, error) {
	effective, err := parseCapabilityDate(request.EffectiveDate, "effective_date")
	if err != nil {
		return nil, err
	}
	opening, err := c.absences.SetVacationOpening(ctx, staffID, decidedBy, timetracking.SetVacationOpeningRequest{
		EffectiveDate: effective, RemainingDays: request.RemainingDays, Note: request.Note,
	})
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicVacationOpening(opening), nil
}

func (c staffAbsenceCapability) DeleteVacationOpening(ctx context.Context, staffID, deletedBy int64, year int) error {
	return mapTimeTrackingFailure(c.absences.DeleteVacationOpening(ctx, staffID, deletedBy, year))
}

// --- months -----------------------------------------------------------------

type workTimeMonthCapability struct {
	months timetracking.WorkTimeMonthService
}

// WorkTimeMonthCapability serves workforce.WorkTimeMonths from the retained
// month service.
func WorkTimeMonthCapability(months timetracking.WorkTimeMonthService) workforce.WorkTimeMonths {
	if months == nil {
		panic("work time month capability: month service is required")
	}
	return workTimeMonthCapability{months: months}
}

func (c workTimeMonthCapability) MonthSummary(ctx context.Context, staffID int64, year, month int) (*workforce.MonthSummary, error) {
	summary, err := c.months.GetMonthSummary(ctx, staffID, year, month)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if summary == nil {
		return nil, nil
	}
	result := &workforce.MonthSummary{
		StaffID: summary.StaffID, Year: summary.Year, Month: summary.Month, CarryInMinutes: summary.CarryInMinutes,
		TargetMinutes: summary.TargetMinutes, TargetMinutesToDate: summary.TargetMinutesToDate, ActualMinutes: summary.ActualMinutes,
		CreditedSickMinutes: summary.CreditedSickMinutes, CreditedVacationMinutes: summary.CreditedVacationMinutes,
		CreditedTrainingMinutes: summary.CreditedTrainingMinutes, CreditedOtherMinutes: summary.CreditedOtherMinutes,
		SickDays: summary.SickDays, VacationDays: summary.VacationDays, TrainingDays: summary.TrainingDays,
		PlannedShiftMinutes: summary.PlannedShiftMinutes, AdjustmentMinutes: summary.AdjustmentMinutes,
		BalanceMinutes: summary.BalanceMinutes, ClosingBalanceMinutes: summary.ClosingBalanceMinutes, IsClosed: summary.IsClosed,
		ClosedAt: summary.ClosedAt, ClosedBy: summary.ClosedBy, CloseReason: summary.CloseReason,
		FrozenClosingBalanceMinutes: summary.FrozenClosingBalanceMinutes, DriftMinutes: summary.DriftMinutes,
		CarryInFrozen: summary.CarryInFrozen, CarryInFrozenFromMonth: summary.CarryInFrozenFromMonth,
	}
	if summary.Adjustments != nil {
		result.Adjustments = make([]workforce.AdjustmentView, 0, len(summary.Adjustments))
		for _, adjustment := range summary.Adjustments {
			result.Adjustments = append(result.Adjustments, workforce.AdjustmentView{
				ID: adjustment.ID, Type: adjustment.Type, MinutesDelta: adjustment.MinutesDelta, EffectiveDate: adjustment.EffectiveDate.String(),
				Note: adjustment.Note, DecidedBy: adjustment.DecidedBy, DecidedAt: adjustment.DecidedAt,
			})
		}
	}
	return result, nil
}

func (c workTimeMonthCapability) DailyTargets(ctx context.Context, staffID int64, from, to string) ([]workforce.DailyTarget, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	targets, err := c.months.GetDailyTargets(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if targets == nil {
		return nil, nil
	}
	result := make([]workforce.DailyTarget, 0, len(targets))
	for _, target := range targets {
		result = append(result, workforce.DailyTarget{Date: target.Date.String(), TargetMinutes: target.TargetMinutes})
	}
	return result, nil
}

func (c workTimeMonthCapability) DailyProjection(ctx context.Context, staffID int64, from, to string) ([]workforce.DailyProjection, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	days, err := c.months.GetDailyProjection(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if days == nil {
		return nil, nil
	}
	result := make([]workforce.DailyProjection, 0, len(days))
	for _, day := range days {
		result = append(result, workforce.DailyProjection{
			Date: day.Date.String(), TargetMinutes: day.TargetMinutes, CreditMinutes: day.CreditMinutes,
			ActualMinutes: day.ActualMinutes, BalanceMinutes: day.BalanceMinutes,
		})
	}
	return result, nil
}

// --- balance ledger ---------------------------------------------------------

type balanceAdjustmentCapability struct {
	ledger timetracking.StaffBalanceAdjustmentService
}

// BalanceAdjustmentCapability serves workforce.BalanceAdjustments from the
// retained ledger service.
func BalanceAdjustmentCapability(ledger timetracking.StaffBalanceAdjustmentService) workforce.BalanceAdjustments {
	if ledger == nil {
		panic("balance adjustment capability: ledger service is required")
	}
	return balanceAdjustmentCapability{ledger: ledger}
}

func publicBalanceAdjustment(entity *activeModels.StaffBalanceAdjustment) *workforce.StaffBalanceAdjustment {
	if entity == nil {
		return nil
	}
	return &workforce.StaffBalanceAdjustment{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Type: entity.Type, MinutesDelta: entity.MinutesDelta,
		EffectiveDate: entity.EffectiveDate.String(), Note: entity.Note, DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func (c balanceAdjustmentCapability) adjustment(entity *activeModels.StaffBalanceAdjustment, err error) (*workforce.StaffBalanceAdjustment, error) {
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicBalanceAdjustment(entity), nil
}

func (c balanceAdjustmentCapability) ListAdjustments(ctx context.Context, staffID int64, from, to string) ([]*workforce.StaffBalanceAdjustment, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	entities, err := c.ledger.ListAdjustments(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if entities == nil {
		return nil, nil
	}
	result := make([]*workforce.StaffBalanceAdjustment, 0, len(entities))
	for _, entity := range entities {
		result = append(result, publicBalanceAdjustment(entity))
	}
	return result, nil
}

func (c balanceAdjustmentCapability) CreateAdjustment(ctx context.Context, staffID, decidedBy int64, request workforce.CreateBalanceAdjustmentRequest) (*workforce.StaffBalanceAdjustment, error) {
	effective, err := parseCapabilityDate(request.EffectiveDate, "effective_date")
	if err != nil {
		return nil, err
	}
	return c.adjustment(c.ledger.CreateAdjustment(ctx, staffID, decidedBy, timetracking.CreateBalanceAdjustmentRequest{
		Type: request.Type, MinutesDelta: request.MinutesDelta, EffectiveDate: effective, Note: request.Note,
	}))
}

func (c balanceAdjustmentCapability) DeleteAdjustment(ctx context.Context, staffID, adjustmentID, deletedBy int64) error {
	return mapTimeTrackingFailure(c.ledger.DeleteAdjustment(ctx, staffID, adjustmentID, deletedBy))
}

func (c balanceAdjustmentCapability) ResetBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate string, carryoverMinutes int, note string) (*workforce.StaffBalanceAdjustment, error) {
	effective, err := parseCapabilityDate(effectiveDate, "effective_date")
	if err != nil {
		return nil, err
	}
	return c.adjustment(c.ledger.ResetBalance(ctx, staffID, decidedBy, effective, carryoverMinutes, note))
}

func (c balanceAdjustmentCapability) CreateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate string, balanceMinutes int, note string) (*workforce.StaffBalanceAdjustment, error) {
	effective, err := parseCapabilityDate(effectiveDate, "effective_date")
	if err != nil {
		return nil, err
	}
	return c.adjustment(c.ledger.CreateOpeningBalance(ctx, staffID, decidedBy, effective, balanceMinutes, note))
}

// OpeningBalanceBookingCapability exposes preview and booking through one
// Workforce boundary without widening the general ledger administration port.
func OpeningBalanceBookingCapability(ledger timetracking.StaffBalanceAdjustmentService) workforce.OpeningBalanceBookings {
	if ledger == nil {
		return nil
	}
	return balanceAdjustmentCapability{ledger: ledger}
}

func (c balanceAdjustmentCapability) ValidateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate string, balanceMinutes int, note string) error {
	effective, err := parseCapabilityDate(effectiveDate, "effective_date")
	if err != nil {
		return err
	}
	return mapTimeTrackingFailure(c.ledger.ValidateOpeningBalance(ctx, staffID, decidedBy, effective, balanceMinutes, note))
}

// --- month close ------------------------------------------------------------

type monthClosingCapability struct {
	closing timetracking.StaffMonthCloseService
}

// MonthClosingCapability serves workforce.MonthClosing from the retained
// month-close service.
func MonthClosingCapability(closing timetracking.StaffMonthCloseService) workforce.MonthClosing {
	if closing == nil {
		panic("month closing capability: month close service is required")
	}
	return monthClosingCapability{closing: closing}
}

func (c monthClosingCapability) CloseMonth(ctx context.Context, closedBy int64, year, month int, reason string) (*workforce.MonthCloseResult, error) {
	result, err := c.closing.CloseMonth(ctx, closedBy, year, month, reason)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if result == nil {
		return nil, nil
	}
	return &workforce.MonthCloseResult{
		Year: result.Year, Month: result.Month, ClosedStaff: result.ClosedStaff, SkippedStaff: result.SkippedStaff,
		Snapshots: publicMonthSnapshots(result.Snapshots),
	}, nil
}

func (c monthClosingCapability) ReopenMonth(ctx context.Context, staffID, reopenedBy int64, year, month int, reason string) error {
	return mapTimeTrackingFailure(c.closing.ReopenMonth(ctx, staffID, reopenedBy, year, month, reason))
}

func (c monthClosingCapability) ListMonthStatus(ctx context.Context, year, month int) ([]*workforce.StaffMonthBalanceSnapshot, error) {
	snapshots, err := c.closing.ListMonthStatus(ctx, year, month)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicMonthSnapshots(snapshots), nil
}

// --- overview ---------------------------------------------------------------

type staffOverviewCapability struct {
	overview timetracking.StaffOverviewService
}

// StaffOverviewCapability serves workforce.StaffOverview from the retained
// overview service.
func StaffOverviewCapability(overview timetracking.StaffOverviewService) workforce.StaffOverview {
	if overview == nil {
		panic("staff overview capability: overview service is required")
	}
	return staffOverviewCapability{overview: overview}
}

func (c staffOverviewCapability) DashboardSummary(ctx context.Context, period string) (*workforce.DashboardSummary, error) {
	summary, err := c.overview.GetDashboardSummary(ctx, period)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if summary == nil {
		return nil, nil
	}
	return &workforce.DashboardSummary{
		ActiveStaffCount: summary.ActiveStaffCount, SickToday: summary.SickToday, VacationToday: summary.VacationToday,
		CurrentlyClockedIn: summary.CurrentlyClockedIn, ExpectedClockedIn: summary.ExpectedClockedIn,
		Period:                  workforce.DashboardPeriodTotals(summary.Period),
		SaldoSchoolTotalMinutes: summary.SaldoSchoolTotalMinutes, PendingRequestsCount: summary.PendingRequestsCount,
	}, nil
}

func (c staffOverviewCapability) TimeTrackingOverview(ctx context.Context, filters workforce.OverviewFilters) (*workforce.TimeTrackingOverview, error) {
	overview, err := c.overview.GetTimeTrackingOverview(ctx, timetracking.OverviewFilters(filters))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if overview == nil {
		return nil, nil
	}
	result := &workforce.TimeTrackingOverview{Year: overview.Year, Month: overview.Month, Rows: make([]workforce.TimeTrackingOverviewRow, 0, len(overview.Rows))}
	for _, row := range overview.Rows {
		result.Rows = append(result.Rows, workforce.TimeTrackingOverviewRow(row))
	}
	return result, nil
}

// --- audit log --------------------------------------------------------------

type timeTrackingAuditLogCapability struct {
	log timetracking.TimeTrackingAuditLogService
}

// TimeTrackingAuditLogCapability serves workforce.TimeTrackingAuditLog from
// the retained audit-log service.
func TimeTrackingAuditLogCapability(log timetracking.TimeTrackingAuditLogService) workforce.TimeTrackingAuditLog {
	if log == nil {
		panic("time tracking audit log capability: audit log service is required")
	}
	return timeTrackingAuditLogCapability{log: log}
}

func (c timeTrackingAuditLogCapability) ListAuditLog(ctx context.Context, request workforce.AuditLogListRequest) (*workforce.AuditLogPage, error) {
	from, err := parseOptionalCapabilityDate(request.From, "from")
	if err != nil {
		return nil, err
	}
	to, err := parseOptionalCapabilityDate(request.To, "to")
	if err != nil {
		return nil, err
	}
	page, err := c.log.ListAuditLog(ctx, timetracking.AuditLogListRequest{
		From: from, To: to, StaffID: request.StaffID, ActorStaffID: request.ActorStaffID, Sources: request.Sources,
		Cursor: request.Cursor, Limit: request.Limit,
	})
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if page == nil {
		return nil, nil
	}
	result := &workforce.AuditLogPage{NextCursor: page.NextCursor, RetentionCutoff: page.RetentionCutoff}
	if page.Events != nil {
		result.Events = make([]*workforce.AuditLogEvent, 0, len(page.Events))
		for _, event := range page.Events {
			if event == nil {
				result.Events = append(result.Events, nil)
				continue
			}
			public := &workforce.AuditLogEvent{
				OccurredAt: event.OccurredAt, Source: event.Source, EntryID: event.EntryID, Actor: workforce.AuditLogActor(event.Actor),
				Reason: event.Reason, Detail: event.Detail,
			}
			if event.Staff != nil {
				public.Staff = new(workforce.AuditLogPerson(*event.Staff))
			}
			result.Events = append(result.Events, public)
		}
	}
	return result, nil
}

// --- export -----------------------------------------------------------------

type staffTimeExportCapability struct {
	export timetracking.StaffTimeExportService
}

// StaffTimeExportCapability serves workforce.StaffTimeExport from the retained
// export service.
func StaffTimeExportCapability(export timetracking.StaffTimeExportService) workforce.StaffTimeExport {
	if export == nil {
		panic("staff time export capability: export service is required")
	}
	return staffTimeExportCapability{export: export}
}

func (c staffTimeExportCapability) ExportStaffTime(ctx context.Context, request workforce.TimeExportRequest, actorAccountID int64, actorRole string) (*workforce.ExportFile, error) {
	file, err := c.export.Export(ctx, timetracking.TimeExportRequest(request), actorAccountID, actorRole)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicExportFile(file), nil
}

func (c staffTimeExportCapability) DatevReport(ctx context.Context, request workforce.TimeExportRequest) (*workforce.DatevExportReport, error) {
	report, err := c.export.DatevReport(ctx, timetracking.TimeExportRequest(request))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if report == nil {
		return nil, nil
	}
	result := &workforce.DatevExportReport{
		Format: report.Format, Year: report.Year, Month: report.Month, LineCount: report.LineCount, StaffExported: report.StaffExported,
		UnconfiguredCategories: report.UnconfiguredCategories, OpenMonth: report.OpenMonth,
	}
	if report.StaffSkipped != nil {
		result.StaffSkipped = make([]workforce.DatevSkippedStaff, 0, len(report.StaffSkipped))
		for _, skipped := range report.StaffSkipped {
			result.StaffSkipped = append(result.StaffSkipped, workforce.DatevSkippedStaff(skipped))
		}
	}
	return result, nil
}
