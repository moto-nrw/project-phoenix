// Package shiftplanning composes the HTTP runtime of the staff-shift and
// shift-type administration routes over the Workforce capability (#2689).
// The route adapters themselves must not own the HTTP platform or the
// retained schedule services, so this package supplies authentication,
// tenant transaction scoping, permission names, actor resolution and the
// shared response envelope through their Runtime, and serves the public
// StaffShiftPlanning and ShiftTypeAdministration contracts from the retained
// services until those move into the Workforce owner.
package shiftplanning

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/planexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// capabilityError reports a Workforce sentinel through errors.Is while keeping
// the wording of the retained service error, which is what the HTTP envelope
// renders. Unwrap keeps the original chain reachable.
type capabilityError struct {
	kind  error
	cause error
}

func (e *capabilityError) Error() string        { return e.cause.Error() }
func (e *capabilityError) Is(target error) bool { return target == e.kind }
func (e *capabilityError) Unwrap() error        { return e.cause }

// PlanningDependencies are the retained schedule services the planning
// contract is served from.
type PlanningDependencies struct {
	Shifts     scheduleSvc.StaffShiftService
	Series     scheduleSvc.StaffShiftSeriesService
	Overview   scheduleSvc.StaffScheduleOverviewGetter
	PlanExport planexport.Service
}

type staffShiftPlanning struct{ deps PlanningDependencies }

// NewStaffShiftPlanning binds the /api/staff-shifts capability to the
// retained shift, series, overview and plan export services.
func NewStaffShiftPlanning(deps PlanningDependencies) workforce.StaffShiftPlanning {
	if deps.Shifts == nil || deps.Series == nil || deps.Overview == nil || deps.PlanExport == nil {
		panic("staff shift planning: all schedule services are required")
	}
	return staffShiftPlanning{deps: deps}
}

// --- shifts ---

func (p staffShiftPlanning) ListShifts(ctx context.Context, query workforce.ShiftRange) ([]workforce.PlannedShift, error) {
	from, err := planningDate(query.From, "from")
	if err != nil {
		return nil, err
	}
	to, err := planningDate(query.To, "to")
	if err != nil {
		return nil, err
	}
	var shifts []*scheduleModels.StaffShift
	if query.StaffID > 0 {
		shifts, err = p.deps.Shifts.ListShiftsForStaff(ctx, query.StaffID, from, to)
	} else {
		shifts, err = p.deps.Shifts.ListShifts(ctx, from, to)
	}
	if err != nil {
		return nil, mapPlanningError(err)
	}
	return plannedShiftsToCapability(shifts), nil
}

func (p staffShiftPlanning) CreateShift(ctx context.Context, input workforce.CreateStaffShift) (workforce.PlannedShift, error) {
	shift, err := shiftInputToModel(input.StaffShiftInput)
	if err != nil {
		return workforce.PlannedShift{}, err
	}
	shift.CreatedBy = input.ActorStaffID
	created, err := p.deps.Shifts.CreateShift(ctx, shift)
	if err != nil {
		return workforce.PlannedShift{}, mapPlanningError(err)
	}
	return plannedShiftToCapability(created), nil
}

func (p staffShiftPlanning) UpdateShift(ctx context.Context, input workforce.UpdateStaffShift) (workforce.PlannedShift, error) {
	shift, err := shiftInputToModel(input.StaffShiftInput)
	if err != nil {
		return workforce.PlannedShift{}, err
	}
	shift.ID = input.ID
	actor := input.ActorStaffID
	shift.UpdatedBy = &actor
	updated, err := p.deps.Shifts.UpdateShiftWithOptions(ctx, shift, scheduleSvc.StaffShiftUpdateOptions{
		PreserveExistingNotes:        input.PreserveExistingNotes,
		PreserveExistingShiftType:    input.PreserveExistingShiftType,
		PreserveExistingChangeReason: input.PreserveExistingChangeReason,
		// The ordinary update never flips the cancellation state: cancel and
		// reactivate go through ApplyCancellation, which rebuilds the cover set
		// atomically (#1841).
		PreserveExistingCancelled: true,
	})
	if err != nil {
		return workforce.PlannedShift{}, mapPlanningError(err)
	}
	return plannedShiftToCapability(updated), nil
}

func (p staffShiftPlanning) MoveShift(ctx context.Context, input workforce.MoveStaffShift) (workforce.PlannedShift, error) {
	date, err := planningDate(input.Date, "date")
	if err != nil {
		return workforce.PlannedShift{}, err
	}
	start, end, err := planningWindow(input.StartTime, input.EndTime)
	if err != nil {
		return workforce.PlannedShift{}, err
	}
	moved, err := p.deps.Shifts.MoveShift(ctx, scheduleSvc.MoveShiftInput{
		ShiftID: input.ShiftID, SourceStaffID: input.SourceStaffID, TargetStaffID: input.TargetStaffID,
		Date: date, StartTime: start, EndTime: end, BreakMinutes: input.BreakMinutes, ShiftTypeID: input.ShiftTypeID,
		ActorStaffID: input.ActorStaffID, ActorAccountID: input.ActorAccountID,
	})
	if err != nil {
		return workforce.PlannedShift{}, mapPlanningError(err)
	}
	return plannedShiftToCapability(moved), nil
}

func (p staffShiftPlanning) ApplyCancellation(ctx context.Context, input workforce.CancelStaffShift) (workforce.StaffShiftCancellation, error) {
	request := scheduleSvc.CancelShiftInput{
		ShiftID: input.ShiftID, Cancelled: input.Cancelled, ChangeReason: input.ChangeReason,
		ActorStaffID: input.ActorStaffID, ApplyOriginEdits: input.ApplyOriginEdits,
		BreakMinutes: input.BreakMinutes, ShiftTypeID: input.ShiftTypeID,
	}
	if input.ApplyOriginEdits {
		start, end, err := planningWindow(input.StartTime, input.EndTime)
		if err != nil {
			return workforce.StaffShiftCancellation{}, err
		}
		request.StartTime, request.EndTime = start, end
	}
	for _, replacement := range input.Replacements {
		start, end, err := planningWindow(replacement.StartTime, replacement.EndTime)
		if err != nil {
			return workforce.StaffShiftCancellation{}, err
		}
		request.Replacements = append(request.Replacements, scheduleSvc.ShiftReplacementInput{
			StaffID: replacement.StaffID, StartTime: start, EndTime: end,
			BreakMinutes: replacement.BreakMinutes, ShiftTypeID: replacement.ShiftTypeID,
		})
	}
	result, err := p.deps.Shifts.ApplyCancellation(ctx, request)
	if err != nil {
		return workforce.StaffShiftCancellation{}, mapPlanningError(err)
	}
	return workforce.StaffShiftCancellation{
		Shift: plannedShiftToCapability(result.Shift), Replacements: plannedShiftsToCapability(result.Replacements),
	}, nil
}

func (p staffShiftPlanning) DeleteShift(ctx context.Context, id int64) error {
	return mapPlanningError(p.deps.Shifts.DeleteShift(ctx, id))
}

// --- series ---

func (p staffShiftPlanning) CreateSeries(ctx context.Context, input workforce.CreateStaffShiftSeries) (workforce.StaffShiftSeriesResult, error) {
	series, err := seriesInputToModel(input.StaffShiftSeriesInput)
	if err != nil {
		return workforce.StaffShiftSeriesResult{}, err
	}
	series.CreatedBy = input.ActorStaffID
	result, err := p.deps.Series.CreateSeries(ctx, series)
	if err != nil {
		return workforce.StaffShiftSeriesResult{}, mapPlanningError(err)
	}
	return seriesResultToCapability(result), nil
}

func (p staffShiftPlanning) SplitSeries(ctx context.Context, input workforce.SplitStaffShiftSeries) (workforce.StaffShiftSeriesResult, error) {
	effective, err := planningDate(input.EffectiveDate, "effective_date")
	if err != nil {
		return workforce.StaffShiftSeriesResult{}, err
	}
	start, end, err := planningWindow(input.StartTime, input.EndTime)
	if err != nil {
		return workforce.StaffShiftSeriesResult{}, err
	}
	var validUntil *timezone.Date
	if input.ValidUntil != "" {
		until, err := planningDate(input.ValidUntil, "valid_until")
		if err != nil {
			return workforce.StaffShiftSeriesResult{}, err
		}
		validUntil = &until
	}
	var weekdays []int16
	if input.Weekdays != nil {
		weekdays = make([]int16, 0, len(input.Weekdays))
		for _, weekday := range input.Weekdays {
			if weekday < 1 || weekday > 7 {
				return workforce.StaffShiftSeriesResult{}, &workforce.InvalidShiftSeriesError{Reason: "weekdays must be between 1 (Monday) and 7 (Sunday)"}
			}
			weekdays = append(weekdays, int16(weekday)) // #nosec G115 -- range checked above
		}
	}
	result, err := p.deps.Series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
		SeriesID: input.SeriesID, EffectiveDate: effective, OccurrenceShiftID: input.OccurrenceShiftID,
		Weekdays: weekdays, StartTime: start, EndTime: end, BreakMinutes: input.BreakMinutes,
		ShiftTypeID: input.ShiftTypeID, ShiftTypeIDSet: input.ShiftTypeIDSet, Notes: input.Notes,
		ValidUntil: validUntil, ValidUntilSet: input.ValidUntilSet, WeekPattern: input.WeekPattern,
		ActorStaffID: input.ActorStaffID,
	})
	if err != nil {
		return workforce.StaffShiftSeriesResult{}, mapPlanningError(err)
	}
	return seriesResultToCapability(result), nil
}

func (p staffShiftPlanning) GetSeries(ctx context.Context, id int64) (workforce.StaffShiftSeries, error) {
	series, err := p.deps.Series.GetSeries(ctx, id)
	if err != nil {
		return workforce.StaffShiftSeries{}, mapPlanningError(err)
	}
	return seriesToCapability(series), nil
}

func (p staffShiftPlanning) EndSeries(ctx context.Context, seriesID int64, from string) (workforce.StaffShiftSeriesResult, error) {
	date, err := planningDate(from, "from")
	if err != nil {
		return workforce.StaffShiftSeriesResult{}, err
	}
	result, err := p.deps.Series.EndSeries(ctx, seriesID, date)
	if err != nil {
		return workforce.StaffShiftSeriesResult{}, mapPlanningError(err)
	}
	return seriesResultToCapability(result), nil
}

// --- overview and export ---

func (p staffShiftPlanning) Overview(ctx context.Context, from, to string) (workforce.StaffScheduleOverview, error) {
	fromDate, err := planningDate(from, "from")
	if err != nil {
		return workforce.StaffScheduleOverview{}, err
	}
	toDate, err := planningDate(to, "to")
	if err != nil {
		return workforce.StaffScheduleOverview{}, err
	}
	overview, err := p.deps.Overview.GetOverview(ctx, fromDate, toDate)
	if err != nil {
		return workforce.StaffScheduleOverview{}, mapPlanningError(err)
	}
	return overviewToCapability(overview), nil
}

func (p staffShiftPlanning) ExportPlan(ctx context.Context, request workforce.PlanExportRequest) (workforce.PlanExportFile, error) {
	params, err := planexport.ParseParams(request.From, request.To, request.Template, request.Variant, request.Format)
	if err != nil {
		return workforce.PlanExportFile{}, &capabilityError{kind: workforce.ErrPlanExportInvalid, cause: err}
	}
	if params.Variant == planexport.VariantInternal && !request.AllowInternal {
		return workforce.PlanExportFile{}, workforce.ErrPlanExportForbidden
	}
	file, err := p.deps.PlanExport.ExportDienstplan(ctx, params)
	if err != nil {
		if errors.Is(err, planexport.ErrInvalidParams) {
			return workforce.PlanExportFile{}, &capabilityError{kind: workforce.ErrPlanExportInvalid, cause: err}
		}
		return workforce.PlanExportFile{}, err
	}
	return workforce.PlanExportFile{ContentType: file.ContentType, Filename: file.Filename, Data: file.Data}, nil
}

// --- mapping ---

func shiftInputToModel(input workforce.StaffShiftInput) (*scheduleModels.StaffShift, error) {
	date, err := planningDate(input.Date, "date")
	if err != nil {
		return nil, err
	}
	start, end, err := planningWindow(input.StartTime, input.EndTime)
	if err != nil {
		return nil, err
	}
	return &scheduleModels.StaffShift{
		StaffID: input.StaffID, Date: scheduleModels.Date(date), StartTime: start, EndTime: end,
		BreakMinutes: input.BreakMinutes, ShiftTypeID: input.ShiftTypeID, Notes: input.Notes,
		Cancelled: input.Cancelled, ChangeReason: input.ChangeReason, OriginShiftID: input.OriginShiftID,
	}, nil
}

func seriesInputToModel(input workforce.StaffShiftSeriesInput) (*scheduleModels.StaffShiftSeries, error) {
	validFrom, err := planningDate(input.ValidFrom, "valid_from")
	if err != nil {
		return nil, err
	}
	var validUntil *scheduleModels.Date
	if input.ValidUntil != "" {
		until, err := planningDate(input.ValidUntil, "valid_until")
		if err != nil {
			return nil, err
		}
		value := scheduleModels.Date(until)
		validUntil = &value
	}
	start, end, err := planningWindow(input.StartTime, input.EndTime)
	if err != nil {
		return nil, err
	}
	weekdays := make([]int16, 0, len(input.Weekdays))
	for _, weekday := range input.Weekdays {
		if weekday < 1 || weekday > 7 {
			return nil, &workforce.InvalidShiftSeriesError{Reason: "weekdays must be between 1 (Monday) and 7 (Sunday)"}
		}
		weekdays = append(weekdays, int16(weekday)) // #nosec G115 -- range checked above
	}
	return &scheduleModels.StaffShiftSeries{
		StaffID: input.StaffID, Weekdays: weekdays, StartTime: start, EndTime: end, BreakMinutes: input.BreakMinutes,
		ShiftTypeID: input.ShiftTypeID, Notes: input.Notes, CalendarPeriodID: input.CalendarPeriodID,
		WeekPattern: input.WeekPattern, ValidFrom: scheduleModels.Date(validFrom), ValidUntil: validUntil,
	}, nil
}

func plannedShiftToCapability(shift *scheduleModels.StaffShift) workforce.PlannedShift {
	if shift == nil {
		return workforce.PlannedShift{}
	}
	value := workforce.PlannedShift{StaffShift: workforce.StaffShift{
		ID: shift.ID, TenantID: shift.TenantID, StaffID: shift.StaffID, Date: shift.Date.String(),
		StartTime: clockString(shift.StartTime), EndTime: clockString(shift.EndTime), BreakMinutes: shift.BreakMinutes,
		ShiftTypeID: shift.ShiftTypeID, Notes: shift.Notes, SeriesID: shift.SeriesID, Detached: shift.Detached,
		Cancelled: shift.Cancelled, ChangeReason: shift.ChangeReason, OriginShiftID: shift.OriginShiftID,
		SickAbsenceID: shift.SickAbsenceID, CreatedBy: shift.CreatedBy, UpdatedBy: shift.UpdatedBy,
		CreatedAt: shift.CreatedAt, UpdatedAt: shift.UpdatedAt,
	}}
	if shift.SeriesOccurrenceDate != nil {
		value.SeriesOccurrenceDate = shift.SeriesOccurrenceDate.String()
	}
	if shift.ShiftType != nil {
		value.ShiftType = &workforce.ShiftTypeLabel{Name: shift.ShiftType.Name, Color: shift.ShiftType.Color}
	}
	return value
}

func plannedShiftsToCapability(shifts []*scheduleModels.StaffShift) []workforce.PlannedShift {
	result := make([]workforce.PlannedShift, 0, len(shifts))
	for _, shift := range shifts {
		if shift == nil {
			continue
		}
		result = append(result, plannedShiftToCapability(shift))
	}
	return result
}

func seriesToCapability(series *scheduleModels.StaffShiftSeries) workforce.StaffShiftSeries {
	if series == nil {
		return workforce.StaffShiftSeries{}
	}
	weekdays := make([]int, 0, len(series.Weekdays))
	for _, weekday := range series.Weekdays {
		weekdays = append(weekdays, int(weekday))
	}
	value := workforce.StaffShiftSeries{
		ID: series.ID, TenantID: series.TenantID, StaffID: series.StaffID, Weekdays: weekdays,
		StartTime: clockString(series.StartTime), EndTime: clockString(series.EndTime), BreakMinutes: series.BreakMinutes,
		ShiftTypeID: series.ShiftTypeID, Notes: series.Notes, CalendarPeriodID: series.CalendarPeriodID,
		WeekPattern: series.WeekPattern, ValidFrom: series.ValidFrom.String(), SeriesRootID: series.SeriesRootID,
		RetainedOccurrenceShiftID: series.RetainedOccurrenceShiftID, CreatedBy: series.CreatedBy, UpdatedBy: series.UpdatedBy,
		CreatedAt: series.CreatedAt, UpdatedAt: series.UpdatedAt,
	}
	if series.ValidUntil != nil {
		value.ValidUntil = series.ValidUntil.String()
	}
	return value
}

func seriesResultToCapability(result *scheduleSvc.SeriesResult) workforce.StaffShiftSeriesResult {
	if result == nil {
		return workforce.StaffShiftSeriesResult{SkippedDates: []string{}}
	}
	skipped := make([]string, 0, len(result.SkippedDates))
	for _, date := range result.SkippedDates {
		skipped = append(skipped, date.String())
	}
	value := workforce.StaffShiftSeriesResult{
		OldSeriesID: result.OldSeriesID, Created: result.Created, Deleted: result.Deleted, SkippedDates: skipped,
	}
	if result.Series != nil {
		value.SeriesID = result.Series.ID
	}
	return value
}

func overviewToCapability(overview *scheduleSvc.StaffScheduleOverview) workforce.StaffScheduleOverview {
	if overview == nil {
		return workforce.StaffScheduleOverview{}
	}
	usedWeeks := make([]string, 0, len(overview.UsedWeeks))
	for _, week := range overview.UsedWeeks {
		usedWeeks = append(usedWeeks, week.String())
	}
	staff := make([]workforce.OverviewStaff, 0, len(overview.Staff))
	for _, member := range overview.Staff {
		if member == nil || member.Person == nil {
			continue
		}
		staff = append(staff, workforce.OverviewStaff{ID: member.ID, FirstName: member.Person.FirstName, LastName: member.Person.LastName})
	}
	assignments := make([]workforce.OverviewAssignment, 0, len(overview.Assignments))
	for _, assignment := range overview.Assignments {
		intervals := make([]workforce.CoverageInterval, 0, len(assignment.UncoveredIntervals))
		for _, interval := range assignment.UncoveredIntervals {
			intervals = append(intervals, workforce.CoverageInterval{StartTime: clockString(interval.StartTime), EndTime: clockString(interval.EndTime)})
		}
		assignments = append(assignments, workforce.OverviewAssignment{
			InstanceID: assignment.InstanceID, StaffID: assignment.StaffID, Date: assignment.Date.String(),
			StartTime: clockString(assignment.StartTime), EndTime: clockString(assignment.EndTime),
			ActivityTitle: assignment.ActivityTitle, RoomID: assignment.RoomID, RoomName: assignment.RoomName,
			Status: assignment.Status, IsAbsent: assignment.IsAbsent, IsSubstitute: assignment.IsSubstitute,
			AbsenceReason: assignment.AbsenceReason, CoverageStatus: assignment.CoverageStatus,
			CoverageReason: assignment.CoverageReason, UncoveredIntervals: intervals,
		})
	}
	summaries := make([]workforce.WeeklySummary, 0, len(overview.WeeklySummaries))
	for _, summary := range overview.WeeklySummaries {
		summaries = append(summaries, workforce.WeeklySummary{
			StaffID: summary.StaffID, WeekStart: summary.WeekStart.String(), PlannedMinutes: summary.PlannedMinutes,
			TargetMinutes: summary.TargetMinutes, DeltaMinutes: summary.DeltaMinutes,
		})
	}
	return workforce.StaffScheduleOverview{
		From: overview.From.String(), To: overview.To.String(), DienstplanInUse: overview.DienstplanInUse,
		UsedWeeks: usedWeeks, Staff: staff, Shifts: plannedShiftsToCapability(overview.Shifts),
		Assignments: assignments, WeeklySummaries: summaries,
	}
}

// planningDate parses a calendar day; a malformed one is the invalid-shift
// rejection so the route renders it as a bad request.
func planningDate(value, field string) (timezone.Date, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return timezone.Date(""), &workforce.InvalidStaffShiftError{Reason: field + " must be YYYY-MM-DD"}
	}
	return date, nil
}

// planningWindow parses the ClockLayout wall clocks of a shift window into
// the normalized clock values the retained services expect.
func planningWindow(startValue, endValue string) (time.Time, time.Time, error) {
	start, err := planningClock(startValue, "start_time")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := planningClock(endValue, "end_time")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}

func planningClock(value, field string) (time.Time, error) {
	parsed, err := time.Parse(workforce.ClockLayout, value)
	if err != nil {
		return time.Time{}, &workforce.InvalidStaffShiftError{Reason: field + " must be " + workforce.ClockLayout}
	}
	return timezone.NormalizeWallClock(parsed), nil
}

func clockString(value time.Time) string {
	return timezone.NormalizeWallClock(value).Format(workforce.ClockLayout)
}

var planningErrorKinds = []struct {
	service    error
	capability error
}{
	{scheduleSvc.ErrShiftOverlap, workforce.ErrStaffShiftOverlap},
	{scheduleSvc.ErrShiftConflict, workforce.ErrStaffShiftConflict},
	{scheduleSvc.ErrShiftNotFound, workforce.ErrStaffShiftNotFound},
	{scheduleSvc.ErrShiftRangeTooLarge, workforce.ErrStaffShiftRangeTooLarge},
	{scheduleSvc.ErrShiftInvalid, workforce.ErrInvalidStaffShift},
	{scheduleSvc.ErrShiftTypeNotFound, workforce.ErrShiftTypeNotFound},
	{scheduleSvc.ErrShiftTypeInactive, workforce.ErrShiftTypeInactive},
	{scheduleSvc.ErrSeriesNotFound, workforce.ErrShiftSeriesNotFound},
	{scheduleSvc.ErrSeriesInvalid, workforce.ErrInvalidShiftSeries},
}

func mapPlanningError(err error) error {
	if err == nil {
		return nil
	}
	for _, kind := range planningErrorKinds {
		if errors.Is(err, kind.service) {
			return &capabilityError{kind: kind.capability, cause: err}
		}
	}
	return err
}
