package timetablehttp

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Optional series end (#3594): a series may stop before its planning period
// ends, e.g. a holiday-care week. The request names the inclusive last day,
// stored as activities.groups.series_last_day; materialization plans nothing
// after it. It is not a schedule valid_until, which marks a capped segment.

// parseSeriesEndDate reads an end_date value (YYYY-MM-DD).
func parseSeriesEndDate(w http.ResponseWriter, r *http.Request, raw *string) (*calendar.Date, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	parsed, err := berlinDate(*raw)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return nil, false
	}
	return &parsed, true
}

// validateSeriesEnd checks the last day against the series start and the
// planning period; nil (no end) always passes.
func (rs *Resource) validateSeriesEnd(
	w http.ResponseWriter,
	r *http.Request,
	lastDay *calendar.Date,
	firstDay calendar.Date,
	calendarPeriodID *int64,
) bool {
	if lastDay == nil {
		return true
	}
	var periodEnd *calendar.Date
	if calendarPeriodID != nil {
		if rs.CalendarPeriods == nil {
			common.RenderError(w, r, common.ErrorInternalServer(errors.New("calendar period service not wired")))
			return false
		}
		period, err := rs.CalendarPeriods.FindCalendarPeriod(r.Context(), *calendarPeriodID)
		if err != nil {
			renderTemplatePeriodLookupError(w, r, err)
			return false
		}
		end := calendar.Date(period.EndDate)
		periodEnd = &end
	}
	if err := timetableModule.ValidateSeriesLastDay(*lastDay, firstDay, periodEnd); err != nil {
		common.RenderError(w, r, codedInvalidOnField(err, "end_date"))
		return false
	}
	return true
}

// seriesLastDayString is the stored form of a validated last day.
func seriesLastDayString(lastDay *calendar.Date) *string {
	if lastDay == nil {
		return nil
	}
	day := lastDay.String()
	return &day
}

// updateSeriesFirstDay is the lower bound for an edited last day: the pulled
// forward start when the edit moves it, otherwise the stored series start
// (zero when the series starts with its planning period). The stored rows
// arrive in weekday order, not by valid_from, so the earliest of them is the
// series start; a later split day must not reject a valid last day. A row
// without valid_from starts with the planning period and so precedes every
// split — it leaves no lower bound beyond the period itself.
func updateSeriesFirstDay(start *calendar.Date, stored templateResponse) calendar.Date {
	if start != nil {
		return *start
	}
	earliest := calendar.Date("")
	for _, schedule := range stored.Schedules {
		if schedule.ValidFrom == "" {
			return calendar.Date("")
		}
		validFrom := calendar.Date(schedule.ValidFrom)
		if earliest.IsZero() || validFrom.Before(earliest) {
			earliest = validFrom
		}
	}
	return earliest
}

// prepareTemplateUpdate checks the edit against the stored series and fills in
// what the request left out: workdays, the target period, offering sources
// and, when sent, the series' last day (#3594).
func (rs *Resource) prepareTemplateUpdate(
	w http.ResponseWriter,
	r *http.Request,
	parsed *parsedUpdateTemplate,
	stored templateResponse,
) bool {
	if err := validateLegacyTemplateWorkdays(stored.Schedules, parsed.req.Weekdays); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return false
	}
	parsed.req.CalendarPeriodID = updateCalendarPeriodID(
		parsed.startDate, parsed.req.CalendarPeriodID, stored)
	applyOfferingSourcePresence(parsed.req, stored)
	if parsed.req.EndDate.Set && !rs.validateSeriesEnd(w, r, parsed.endDate, updateSeriesFirstDay(parsed.startDate, stored), parsed.req.CalendarPeriodID) {
		return false
	}
	return validateNotesOnlyPredecessorPropagation(w, r, parsed, stored)
}

// validateNotesOnlyPredecessorPropagation keeps the chain-reconciliation
// switch confined to the one save it represents. The request body is a full
// PUT, so an otherwise authorised caller could combine the switch with a
// title, recurrence, roster, or source edit. That must not make the living
// segment's note overwrite capped predecessor segments.
func validateNotesOnlyPredecessorPropagation(
	w http.ResponseWriter,
	r *http.Request,
	parsed *parsedUpdateTemplate,
	stored templateResponse,
) bool {
	if !parsed.req.PropagateNotesToPredecessors || notesOnlyTemplateUpdate(parsed, stored) {
		return true
	}
	common.RenderError(w, r, invalidOnField(
		common.CodeTimetableTemplateInvalid,
		"propagate_notes_to_predecessors",
		"Die Wochennotiz kann nur allein für alle Termine geändert werden.",
	))
	return false
}

func notesOnlyTemplateUpdate(parsed *parsedUpdateTemplate, stored templateResponse) bool {
	req := parsed.req
	if sameOptionalString(normalizeNotes(req.Notes), stored.Notes) ||
		req.StartDate != nil || req.EndDate.Set || req.SeriesRosterFrom != nil ||
		len(req.SeriesRosterScopeStudentIDs) > 0 || len(req.SeriesRosterScopeStaffIDs) > 0 ||
		len(req.SeriesRosterScopeWeekdays) > 0 || req.SeriesRosterPrimaryChanged {
		return false
	}
	return sameTemplateFieldsExceptNotes(req, stored) &&
		sameTemplateRecurrence(req, parsed, stored) &&
		sameTemplateRoster(req, stored)
}

func sameTemplateFieldsExceptNotes(req *updateTemplateRequest, stored templateResponse) bool {
	if req.Name != stored.Name || req.Type != stored.Type || req.CategoryID != stored.CategoryID ||
		req.RoomID != optionalInt64Value(stored.RoomID) ||
		!sameOptionalInt64(req.EducationGroupID, stored.EducationGroupID) ||
		!sameOptionalInt(req.RequiredStaff, stored.RequiredStaffOverride) ||
		!sameOptionalInt64(req.CalendarPeriodID, stored.CalendarPeriodID) ||
		req.TargetGroupType != stored.TargetGroupType ||
		!sameOptionalInt16(req.TargetGradeLevel, stored.TargetGradeLevel) ||
		!sameOptionalString(req.TargetSchoolClass, stored.TargetSchoolClass) ||
		!sameOptionalString(req.ListKind, stored.ListKind) ||
		!sameValues(req.SourceCareOfferingIDs.Value, stored.SourceCareOfferingIDs) ||
		!sameValues(req.SourceGradeLevels.Value, stored.SourceGradeLevels) ||
		!sameValues(req.SourceSchoolClasses.Value, stored.SourceSchoolClasses) {
		return false
	}
	if req.PlanningTrackID.Set && !sameOptionalInt64(req.PlanningTrackID.Value, stored.PlanningTrackID) {
		return false
	}
	if req.MaxParticipants.Set && derefInt(req.MaxParticipants.Value) != derefInt(stored.MaxParticipants) {
		return false
	}
	if req.IncludeClosingDays != nil && *req.IncludeClosingDays != stored.IncludeClosingDays {
		return false
	}
	return req.Targets == nil || sameTemplateTargets(req.Targets, stored.Targets)
}

func sameTemplateRecurrence(req *updateTemplateRequest, parsed *parsedUpdateTemplate, stored templateResponse) bool {
	if len(req.Weekdays) != len(stored.Schedules) {
		return false
	}
	startTime, endTime := parsed.startTime.Format("15:04"), parsed.endTime.Format("15:04")
	byWeekday := make(map[int]templateScheduleResponse, len(stored.Schedules))
	for _, schedule := range stored.Schedules {
		byWeekday[schedule.Weekday] = schedule
	}
	seen := make(map[int]struct{}, len(req.Weekdays))
	for _, weekday := range req.Weekdays {
		if _, duplicate := seen[weekday]; duplicate {
			return false
		}
		seen[weekday] = struct{}{}
		schedule, ok := byWeekday[weekday]
		if !ok || schedule.StartTime != startTime || schedule.EndTime != endTime ||
			schedule.WeekPattern != parsed.weekPattern ||
			!sameOptionalInt64(schedule.CalendarPeriodID, req.CalendarPeriodID) {
			return false
		}
	}
	return true
}

func sameTemplateRoster(req *updateTemplateRequest, stored templateResponse) bool {
	if !sameIDSet(req.StudentIDs, stored.StudentIDs) || !sameIDSet(req.StaffIDs, stored.StaffIDs) ||
		!sameOptionalInt64(req.PrimaryStaffID, stored.PrimaryStaffID) ||
		len(req.WeekdayAssignments) != len(stored.WeekdayAssignments) {
		return false
	}
	byWeekday := make(map[int]templateWeekdayAssignmentResponse, len(stored.WeekdayAssignments))
	for _, assignment := range stored.WeekdayAssignments {
		byWeekday[assignment.Weekday] = assignment
	}
	for _, assignment := range req.WeekdayAssignments {
		storedAssignment, ok := byWeekday[assignment.Weekday]
		if !ok || !sameIDSet(assignment.StudentIDs, storedAssignment.StudentIDs) ||
			!sameIDSet(assignment.StaffIDs, storedAssignment.StaffIDs) ||
			!sameOptionalInt64(assignment.PrimaryStaffID, storedAssignment.PrimaryStaffID) {
			return false
		}
	}
	return true
}

func sameTemplateTargets(requested []templateTargetRequest, stored []templateTargetResponse) bool {
	if len(requested) != len(stored) {
		return false
	}
	matched := make([]bool, len(stored))
	for _, target := range requested {
		found := false
		for index, candidate := range stored {
			if !matched[index] && target.Type == candidate.Type &&
				sameOptionalInt16(target.GradeLevel, candidate.GradeLevel) &&
				sameOptionalString(target.SchoolClass, candidate.SchoolClass) &&
				sameOptionalInt64(target.EducationGroupID, candidate.EducationGroupID) {
				matched[index], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sameIDSet(left, right []int64) bool {
	seen := make(map[int64]struct{}, len(left))
	for _, id := range left {
		if id > 0 {
			seen[id] = struct{}{}
		}
	}
	other := make(map[int64]struct{}, len(right))
	for _, id := range right {
		if id > 0 {
			other[id] = struct{}{}
		}
	}
	if len(seen) != len(other) {
		return false
	}
	for id := range other {
		if _, ok := seen[id]; !ok {
			return false
		}
	}
	return true
}

func sameValues[T comparable](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func optionalInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func sameOptionalInt64(left, right *int64) bool {
	return left == right || left != nil && right != nil && *left == *right
}

func sameOptionalInt(left, right *int) bool {
	return left == right || left != nil && right != nil && *left == *right
}

func sameOptionalInt16(left, right *int16) bool {
	return left == right || left != nil && right != nil && *left == *right
}

func sameOptionalString(left, right *string) bool {
	return left == right || left != nil && right != nil && *left == *right
}
