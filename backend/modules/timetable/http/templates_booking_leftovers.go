// Package timetablehttp — children a hand-maintained Regeltermin keeps
// after a booking change (#5102, ADR 0128 addendum of 2026-11-01):
//
//	GET /api/timetable/templates/booking-leftovers?period_id=…
//	PUT /api/timetable/templates/{id}/students/{studentId}/kept-days
//
// The save response of a booking change lists these Regeltermine once. This
// read finds them again from the stored bookings and roster rows, so the
// office can still remove the child or keep it after a reload.
package timetablehttp

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type bookingLeftoverResponse struct {
	TemplateID     int64  `json:"template_id"`
	TemplateName   string `json:"template_name"`
	StudentID      int64  `json:"student_id"`
	StudentName    string `json:"student_name"`
	Weekdays       []int  `json:"weekdays"`
	BookingEndedOn string `json:"booking_ended_on"`
	// From is the first day "Entfernen" changes, tomorrow at the earliest.
	From string `json:"from"`
	Kept bool   `json:"kept"`
}

type bookingLeftoversResponse struct {
	Leftovers []bookingLeftoverResponse `json:"leftovers"`
}

// listBookingLeftovers handles GET /templates/booking-leftovers. An unknown
// or inactive period shows no hints. Every read is batched over the
// period's templates, bookings and children.
func (rs *Resource) listBookingLeftovers(w http.ResponseWriter, r *http.Request) {
	if rs.TimetableData == nil || rs.CalendarPeriods == nil || rs.RosterReview == nil || rs.People == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable booking leftovers not wired")))
		return
	}
	periodID, ok := templatePeriodIDFromRequest(w, r)
	if !ok {
		return
	}
	if periodID == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("period_id is required")))
		return
	}
	leftovers, err := rs.bookingLeftovers(r.Context(), *periodID)
	if errors.Is(err, errInactivePlanningPeriod) {
		leftovers, err = []bookingLeftoverResponse{}, nil
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list booking leftovers failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, bookingLeftoversResponse{Leftovers: leftovers}, "Booking leftovers retrieved")
}

// bookingLeftovers compares the period's bookings since its start with the
// hand rows of its Regeltermine from tomorrow on. A Regeltermin the
// deviation hint already shows is left out: that hint covers its children.
func (rs *Resource) bookingLeftovers(ctx context.Context, periodID int64) ([]bookingLeftoverResponse, error) {
	result := []bookingLeftoverResponse{}
	period, err := rs.activePlanningPeriod(ctx, periodID)
	if err != nil || rs.OfferingSourceOptions == nil {
		return result, err
	}
	scope := timetable.BookingLeftoverScope{Tomorrow: rs.todayDate().AddDays(1), PeriodID: periodID}
	until := calendar.Date(period.EndDate)
	if scope.Tomorrow.After(until) {
		return result, nil
	}
	bookings, err := rs.OfferingSourceOptions.OfferingWeekdayBookings(ctx, &periodID, calendar.Date(period.StartDate))
	if err != nil || !anyBookingEnded(bookings) {
		return result, err
	}
	templates, err := rs.bookingLeftoverTemplates(ctx, periodID, &timetable.TemplateScheduleWindow{From: scope.Tomorrow, Until: until})
	if err != nil {
		return nil, err
	}
	leftovers := timetable.DetectBookingLeftovers(scope, templates, bookings)
	if len(leftovers) == 0 {
		return result, nil
	}
	return rs.namedBookingLeftovers(ctx, leftovers, scope.Tomorrow)
}

func anyBookingEnded(bookings []timetable.OfferingWeekdayBooking) bool {
	for _, booking := range bookings {
		for _, window := range booking.Windows {
			if window.Ended {
				return true
			}
		}
	}
	return false
}

// bookingLeftoverTemplates reads the Regeltermine planned from tomorrow on,
// without duty blocks and those the deviation hint shows, with the latest
// end per weekday and their roster rows.
func (rs *Resource) bookingLeftoverTemplates(ctx context.Context, periodID int64, window *timetable.TemplateScheduleWindow) ([]timetable.BookingLeftoverTemplate, error) {
	ratio := rs.childrenPerStaffRatio(ctx)
	rows, err := rs.TimetableData.ListTemplateEntriesForPeriod(ctx, &periodID, ratio, window)
	if err != nil {
		return nil, err
	}
	deviations, err := rs.templateRosterDeviations(ctx, periodID)
	if err != nil {
		return nil, err
	}
	shown := make(map[int64]bool, len(deviations))
	for _, deviation := range deviations {
		shown[deviation.TemplateID] = true
	}
	templates := slices.DeleteFunc(mapTemplateRows(rows, ratio, nil), func(template templateResponse) bool {
		return template.Type == timetable.GroupTypeDuty || shown[template.ID]
	})
	ids := make([]int64, 0, len(templates))
	for _, template := range templates {
		ids = append(ids, template.ID)
	}
	rosters, err := rs.RosterReview.PlannedRosters(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]timetable.BookingLeftoverTemplate, 0, len(templates))
	for _, template := range templates {
		result = append(result, timetable.BookingLeftoverTemplate{
			ID: template.ID, Name: template.Name, Kind: timetable.RosterOfferingKindFor(template.Type, template.ListKind),
			WeekdayEnds: latestScheduleEnds(template.Schedules), Roster: rosters[template.ID],
		})
	}
	return result, nil
}

// latestScheduleEnds keeps the latest end time per weekday; "HH:MM" strings
// compare in clock order.
func latestScheduleEnds(schedules []templateScheduleResponse) map[int]string {
	ends := make(map[int]string, len(schedules))
	for _, schedule := range schedules {
		if current, found := ends[schedule.Weekday]; !found || schedule.EndTime > current {
			ends[schedule.Weekday] = schedule.EndTime
		}
	}
	return ends
}

// namedBookingLeftovers adds the children's names and drops children who no
// longer attend tomorrow.
func (rs *Resource) namedBookingLeftovers(ctx context.Context, leftovers []timetable.BookingLeftover, day calendar.Date) ([]bookingLeftoverResponse, error) {
	studentIDs := make([]int64, 0, len(leftovers))
	for _, leftover := range leftovers {
		studentIDs = append(studentIDs, leftover.StudentID)
	}
	names, err := rs.studentNames(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make([]bookingLeftoverResponse, 0, len(leftovers))
	for _, leftover := range leftovers {
		name, attends := names[leftover.StudentID]
		if !attends {
			continue
		}
		result = append(result, bookingLeftoverResponse{
			TemplateID: leftover.TemplateID, TemplateName: leftover.TemplateName,
			StudentID: leftover.StudentID, StudentName: name, Weekdays: leftover.Weekdays,
			BookingEndedOn: leftover.BookingEndedOn.String(), From: leftover.From.String(), Kept: leftover.Kept,
		})
	}
	return result, nil
}

type keptStudentTemplateDaysRequest struct {
	Weekdays       []int  `json:"weekdays"`
	BookingEndedOn string `json:"booking_ended_on"`
	Kept           *bool  `json:"kept"`
}

func (req *keptStudentTemplateDaysRequest) Bind(*http.Request) error {
	switch {
	case len(req.Weekdays) == 0:
		return errors.New("weekdays are required")
	case req.BookingEndedOn == "":
		return errors.New("booking_ended_on is required (YYYY-MM-DD)")
	case req.Kept == nil:
		return errors.New("kept is required")
	}
	return nil
}

type keptStudentTemplateDaysResponse struct {
	Kept       bool `json:"kept"`
	RosterRows int  `json:"roster_rows"`
}

// setKeptStudentTemplateDays handles PUT
// /templates/{id}/students/{studentId}/kept-days: "Bleibt so" for one
// child, or its undo. The plan stays as it is.
func (rs *Resource) setKeptStudentTemplateDays(w http.ResponseWriter, r *http.Request) {
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
	req := &keptStudentTemplateDaysRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	endedOn, err := calendar.ParseDate(req.BookingEndedOn)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid booking_ended_on format, expected YYYY-MM-DD")))
		return
	}
	rows, err := rs.StudentTemplateDays.KeepStudentTemplateDays(r.Context(), timetable.StudentTemplateDaysKeep{
		TemplateID: templateID, StudentID: studentID, Weekdays: req.Weekdays, BookingEndedOn: endedOn, Kept: *req.Kept,
	})
	switch {
	case errors.Is(err, timetable.ErrGroupNotFound):
		renderTemplateNotFound(w, r)
	case errors.Is(err, timetable.ErrStudentTemplateDaysInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	default:
		common.Respond(w, r, http.StatusOK, keptStudentTemplateDaysResponse{Kept: *req.Kept, RosterRows: rows}, "Student template days kept")
	}
}
