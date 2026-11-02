// Package timetablehttp — POST /api/timetable/templates/{id}/students/{studentId}/remove-days.
//
// "Dort auch entfernen" (#5102): a booking change took weekdays away from a
// child, but a hand-maintained Regeltermin still plans the child there. The
// office confirms, and the child leaves that Regeltermin on exactly those
// weekdays from the effective date on. Past days and attendance stay.
package timetablehttp

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var errBoundedStudentTemplateDaysRemovalStale = errors.New("the booking gap changed")

type removeStudentTemplateDaysRequest struct {
	Weekdays      []int  `json:"weekdays"`
	EffectiveFrom string `json:"effective_from"`
	// EffectiveUntil is optional (#5497): the day a later booking carries
	// the Regeltermin again (exclusive). The child stays planned from then.
	EffectiveUntil string `json:"effective_until,omitempty"`
}

func (req *removeStudentTemplateDaysRequest) Bind(_ *http.Request) error {
	if len(req.Weekdays) == 0 {
		return errors.New("weekdays are required")
	}
	if req.EffectiveFrom == "" {
		return errors.New("effective_from is required (YYYY-MM-DD)")
	}
	return nil
}

type removeStudentTemplateDaysResponse struct {
	RosterRows  int `json:"roster_rows"`
	Occurrences int `json:"occurrences"`
}

func (rs *Resource) removeStudentTemplateDays(w http.ResponseWriter, r *http.Request) {
	templateID, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "studentId", "invalid student id")
	if !ok {
		return
	}
	if rs.StudentTemplateDays == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("student template days not wired")))
		return
	}
	req := &removeStudentTemplateDaysRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	from, err := calendar.ParseDate(req.EffectiveFrom)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid effective_from format, expected YYYY-MM-DD")))
		return
	}
	until, ok := optionalRequestDate(w, r, req.EffectiveUntil, "effective_until")
	if !ok {
		return
	}
	if !validStudentTemplateDaysWeekdays(req.Weekdays) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("weekdays must be ISO weekdays")))
		return
	}
	removalFrom, err := rs.boundedStudentTemplateDaysRemovalFrom(r.Context(), templateID, studentID, req.Weekdays, from, until)
	if err != nil {
		switch {
		case errors.Is(err, errBoundedStudentTemplateDaysRemovalStale):
			common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeTimetableTemplateBookingGapStale))
		default:
			common.RenderError(w, r, common.ErrorInternalServerWrap("validate bounded student template days removal failed", err))
		}
		return
	}
	result, err := rs.StudentTemplateDays.RemoveStudentTemplateDays(r.Context(), timetableModule.StudentTemplateDaysRemoval{
		TemplateID: templateID, StudentID: studentID, Weekdays: req.Weekdays, From: removalFrom, Until: until,
	})
	switch {
	case errors.Is(err, timetableModule.ErrGroupNotFound):
		renderTemplateNotFound(w, r)
	case errors.Is(err, timetableModule.ErrStudentTemplateDaysInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	default:
		// The child left the template's planned occurrences (#5017).
		rs.broadcastStaffingDeviationChanged(r.Context(), "template_student_days")
		common.Respond(w, r, http.StatusOK, removeStudentTemplateDaysResponse{
			RosterRows: result.RosterRows, Occurrences: result.Occurrences,
		}, "Student removed from template days")
	}
}

// boundedStudentTemplateDaysRemovalFrom clamps the mutable start, then
// confirms a bounded removal still names the current booking gap.
func (rs *Resource) boundedStudentTemplateDaysRemovalFrom(
	ctx context.Context, templateID, studentID int64, weekdays []int, from calendar.Date, until *calendar.Date,
) (calendar.Date, error) {
	removalFrom := max(from, rs.todayDate().AddDays(1))
	if until == nil || !until.After(removalFrom) {
		return removalFrom, nil
	}
	return rs.validateBoundedStudentTemplateDaysRemoval(ctx, templateID, studentID, weekdays, removalFrom, until)
}

// validateBoundedStudentTemplateDaysRemoval recalculates a bounded gap under
// the recurrence gate shared with booking changes. The command takes the gate
// again before it writes; PostgreSQL advisory locks are transaction-reentrant.
func (rs *Resource) validateBoundedStudentTemplateDaysRemoval(
	ctx context.Context, templateID, studentID int64, weekdays []int, from calendar.Date, until *calendar.Date,
) (calendar.Date, error) {
	if !rs.boundedStudentTemplateDaysRemovalWired() {
		return "", errors.New("bounded student template days removal not wired")
	}
	if err := rs.RecurrenceLock.LockRecurrenceWrites(ctx); err != nil {
		return "", err
	}
	segments, err := rs.TemplateSeries.ListGroups(ctx, timetableModule.ActiveTemplateSeries(templateID))
	if err != nil {
		return "", err
	}
	series := make(map[int64]struct{}, len(segments))
	for _, segment := range segments {
		series[segment.ID] = struct{}{}
	}
	periods, err := rs.CalendarPeriods.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		ActiveOnly: true, OverlappingFrom: from.String(), OverlappingTo: until.String(),
	})
	if err != nil {
		return "", err
	}
	seriesLeftovers := make([]timetableModule.BookingLeftover, 0)
	for _, period := range periods {
		leftovers, err := rs.detectBookingLeftovers(ctx, period.ID)
		if err != nil {
			return "", err
		}
		for _, leftover := range leftovers {
			if _, belongsToSeries := series[leftover.TemplateID]; !belongsToSeries {
				continue
			}
			seriesLeftovers = append(seriesLeftovers, leftover)
		}
	}
	if effectiveFrom, matches := boundedStudentTemplateDaysRemovalMatchesSeries(seriesLeftovers, studentID, weekdays, from, until); matches {
		return effectiveFrom, nil
	}
	return "", errBoundedStudentTemplateDaysRemovalStale
}

// boundedStudentTemplateDaysRemovalMatchesSeries confirms every requested
// weekday across the active segments of one Regeltermin series.
func boundedStudentTemplateDaysRemovalMatchesSeries(
	leftovers []timetableModule.BookingLeftover, studentID int64, weekdays []int, from calendar.Date, until *calendar.Date,
) (calendar.Date, bool) {
	matchedWeekdays := make(map[int]bool, len(weekdays))
	var effectiveFrom calendar.Date
	for _, leftover := range leftovers {
		fromLeftover, matches := boundedStudentTemplateDaysRemovalMatches(leftover, studentID, from, until)
		if !matches {
			continue
		}
		for _, weekday := range leftover.Weekdays {
			matchedWeekdays[weekday] = true
		}
		if effectiveFrom == "" || fromLeftover.Before(effectiveFrom) {
			effectiveFrom = fromLeftover
		}
	}
	return effectiveFrom, effectiveFrom != "" &&
		sameRemovalWeekdays(sortedRemovalWeekdays(matchedWeekdays), weekdays) && until.After(effectiveFrom)
}

func (rs *Resource) boundedStudentTemplateDaysRemovalWired() bool {
	return rs.RecurrenceLock != nil && rs.CalendarPeriods != nil && rs.TimetableData != nil && rs.RosterReview != nil &&
		rs.MaterializationService != nil && rs.OfferingSourceOptions != nil && rs.TemplateSeries != nil
}

func boundedStudentTemplateDaysRemovalMatches(
	leftover timetableModule.BookingLeftover, studentID int64, from calendar.Date, until *calendar.Date,
) (calendar.Date, bool) {
	if leftover.StudentID != studentID || leftover.Until == nil || leftover.BookingEndedOn.After(from) {
		return "", false
	}
	effectiveFrom := max(from, leftover.From)
	if !until.After(effectiveFrom) {
		return "", false
	}
	return effectiveFrom, *leftover.Until == *until
}

func sortedRemovalWeekdays(weekdays map[int]bool) []int {
	result := make([]int, 0, len(weekdays))
	for weekday := range weekdays {
		result = append(result, weekday)
	}
	slices.Sort(result)
	return result
}

func validStudentTemplateDaysWeekdays(weekdays []int) bool {
	for _, weekday := range weekdays {
		if weekday < timetableModule.WeekdayMonday || weekday > timetableModule.WeekdaySunday {
			return false
		}
	}
	return true
}

func sameRemovalWeekdays(left, right []int) bool {
	left = slices.Clone(left)
	right = slices.Clone(right)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(slices.Compact(left), slices.Compact(right))
}

// optionalRequestDate parses an optional YYYY-MM-DD body field; empty is nil.
func optionalRequestDate(w http.ResponseWriter, r *http.Request, value, field string) (*calendar.Date, bool) {
	if value == "" {
		return nil, true
	}
	date, err := calendar.ParseDate(value)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid "+field+" format, expected YYYY-MM-DD")))
		return nil, false
	}
	return &date, true
}
