package timetablehttp

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const leftoverGanztagID = int64(880101)

// bookingLeftoversSetup drives the booking leftover routes (#5102) over the
// real template list, roster rows and student template days. The offering
// side is the stub; its windows stand for Care Plan's booking links.
type bookingLeftoversSetup struct {
	*rosterReviewSetup
	sources *rosterMaintenanceSources
	period  *templateTestPeriod
}

func buildBookingLeftoversModule(t *testing.T) *bookingLeftoversSetup {
	t.Helper()
	sources := &rosterMaintenanceSources{results: map[int64]timetableModule.TemplateRosterMaintenance{}}
	s := buildRosterReviewModule(t, sources)
	s.res.StudentTemplateDays = RootCompositions.StudentTemplateDays(s.db, func() calendar.Date { return rosterReviewMonday })
	s.router.Get("/templates/booking-leftovers", s.res.listBookingLeftovers)
	s.router.Put("/templates/{id}/students/{studentId}/kept-days", s.res.setKeptStudentTemplateDays)
	s.router.Post("/templates/{id}/students/{studentId}/remove-days", s.res.removeStudentTemplateDays)
	period := createTemplateTestPeriodRange(t, s.db, "Leftover-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	return &bookingLeftoversSetup{rosterReviewSetup: s, sources: sources, period: period}
}

// createHandTemplate creates a care block Monday to Friday 14:00–16:00
// whose children staff picked by hand, without a Klasse target.
func (s *bookingLeftoversSetup) createHandTemplate(t *testing.T, name string, studentIDs []int64) int64 {
	t.Helper()
	body := map[string]any{
		"name":               name,
		"type":               timetableModule.GroupTypeCare,
		"weekdays":           []int{1, 2, 3, 4, 5},
		"start_time":         "14:00",
		"end_time":           "16:00",
		"room_id":            s.roomID,
		"category_id":        s.category.ID,
		"calendar_period_id": s.period.ID,
		"target_group_type":  timetableModule.TargetGroupTypeNone,
		"student_ids":        studentIDs,
		"staff_ids":          []int64{s.staffA},
	}
	w := doTemplateJSON(t, s.router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[createTemplateResponse](t, w).TemplateID
}

// thursdayDroppedBookings: a Korrektur ended the child's Ganztag link on the
// change day and booked it again without Thursday; the other child books
// Monday to Friday throughout.
func thursdayDroppedBookings(changed, unchanged int64, change calendar.Date) []timetableModule.OfferingWeekdayBooking {
	start, end := rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(121)
	window := func(from, until calendar.Date, ended bool, weekdays ...int) timetableModule.OfferingBookingWindow {
		return timetableModule.OfferingBookingWindow{ValidFrom: &from, ValidUntil: &until, Weekdays: weekdays, Ended: ended}
	}
	return []timetableModule.OfferingWeekdayBooking{
		{OfferingID: leftoverGanztagID, OfferingName: "Ganztag", StudentID: changed, CountsAsCare: true, Weekdays: []int{1, 2, 3, 4, 5},
			Windows: []timetableModule.OfferingBookingWindow{window(start, change, true, 1, 2, 3, 4, 5), window(change, end, false, 1, 2, 3, 5)}},
		{OfferingID: leftoverGanztagID, OfferingName: "Ganztag", StudentID: unchanged, CountsAsCare: true, Weekdays: []int{1, 2, 3, 4, 5},
			Windows: []timetableModule.OfferingBookingWindow{window(start, end, false, 1, 2, 3, 4, 5)}},
	}
}

func (s *bookingLeftoversSetup) listLeftovers(t *testing.T) []bookingLeftoverResponse {
	t.Helper()
	w := doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/templates/booking-leftovers?period_id=%d", s.period.ID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[bookingLeftoversResponse](t, w).Leftovers
}

// TestBookingLeftoversSurviveTheReload is the case of the pre-deploy test:
// a Korrektur took the child's Thursday away, nobody clicked "Dort auch
// entfernen", and the hand-maintained Regeltermin keeps the child on every
// Thursday. The read names it again; "Bleibt so" moves it to the decided
// entries and back; "Entfernen" from the first Thursday on clears it.
func TestBookingLeftoversSurviveTheReload(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2c", 2)
	changed, unchanged := children[0], children[1]
	template := s.createHandTemplate(t, "Hausaufgaben 2c", children)
	change := rosterReviewMonday.AddDays(-3)
	s.sources.bookings = thursdayDroppedBookings(changed, unchanged, change)
	firstThursday := rosterReviewMonday.AddDays(3)

	leftovers := s.listLeftovers(t)
	require.Len(t, leftovers, 1, "only the child whose Thursday ended: %+v", leftovers)
	got := leftovers[0]
	assert.Equal(t, template, got.TemplateID)
	assert.Equal(t, "Hausaufgaben 2c", got.TemplateName)
	assert.Equal(t, changed, got.StudentID)
	assert.NotEmpty(t, got.StudentName)
	assert.Equal(t, []int{timetableModule.WeekdayThursday}, got.Weekdays)
	assert.Equal(t, change.String(), got.BookingEndedOn)
	assert.Equal(t, firstThursday.String(), got.From, "the removal starts on the first planned day without a booking, never today or earlier")
	assert.False(t, got.Kept)

	keep := func(kept bool) {
		w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/students/%d/kept-days", template, changed), map[string]any{
			"weekdays": got.Weekdays, "booking_ended_on": got.BookingEndedOn, "kept": kept,
		})
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		assert.Equal(t, 1, decodeTemplateData[keptStudentTemplateDaysResponse](t, w).RosterRows)
	}
	keep(true)
	leftovers = s.listLeftovers(t)
	require.Len(t, leftovers, 1)
	assert.True(t, leftovers[0].Kept, "Bleibt so lists the child as decided")
	keep(false)
	leftovers = s.listLeftovers(t)
	require.Len(t, leftovers, 1)
	assert.False(t, leftovers[0].Kept, "the undo shows the hint again")

	w := doTemplateJSON(t, s.router, http.MethodPost, fmt.Sprintf("/templates/%d/students/%d/remove-days", template, changed), map[string]any{
		"weekdays": got.Weekdays, "effective_from": got.From,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, s.listLeftovers(t), "after the removal nothing is left")
}

// TestBookingLeftoversKeepOneWeekday (quorum review of #5452): one hand row
// plans Monday to Friday; the Monday booking ends on one day, the Thursday
// booking a week later. „Bleibt so“ for Thursday keeps Thursday only, and
// its undo leaves Monday's decision alone.
func TestBookingLeftoversKeepOneWeekday(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2f", 1)
	child := children[0]
	template := s.createHandTemplate(t, "Hausaufgaben 2f", children)
	start, end := rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(121)
	mondayEnds, thursdayEnds := rosterReviewMonday.AddDays(14), rosterReviewMonday.AddDays(21)
	window := func(from, until calendar.Date, ended bool, weekdays ...int) timetableModule.OfferingBookingWindow {
		return timetableModule.OfferingBookingWindow{ValidFrom: &from, ValidUntil: &until, Weekdays: weekdays, Ended: ended}
	}
	s.sources.bookings = []timetableModule.OfferingWeekdayBooking{
		{OfferingID: leftoverGanztagID, StudentID: child, CountsAsCare: true, Weekdays: []int{1, 2, 3, 5},
			Windows: []timetableModule.OfferingBookingWindow{window(start, end, false, 2, 3, 5), window(start, mondayEnds, true, 1)}},
		{OfferingID: leftoverGanztagID + 1, StudentID: child, CountsAsCare: true, Weekdays: []int{4},
			Windows: []timetableModule.OfferingBookingWindow{window(start, thursdayEnds, true, 4)}},
	}
	byWeekday := func() map[int]bookingLeftoverResponse {
		result := map[int]bookingLeftoverResponse{}
		for _, leftover := range s.listLeftovers(t) {
			for _, weekday := range leftover.Weekdays {
				result[weekday] = leftover
			}
		}
		return result
	}
	keep := func(weekday int, endedOn string, kept bool) {
		w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/students/%d/kept-days", template, child), map[string]any{
			"weekdays": []int{weekday}, "booking_ended_on": endedOn, "kept": kept,
		})
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	}

	before := byWeekday()
	require.Len(t, before, 2, "Monday and Thursday each ended: %+v", before)
	keep(1, before[1].BookingEndedOn, true)
	keep(4, before[4].BookingEndedOn, true)
	keep(4, before[4].BookingEndedOn, false)
	after := byWeekday()
	assert.True(t, after[1].Kept, "Monday stays decided")
	assert.False(t, after[4].Kept, "only Thursday's decision was withdrawn")

	keep(1, before[1].BookingEndedOn, false)
	keep(4, before[4].BookingEndedOn, true)
	after = byWeekday()
	assert.False(t, after[1].Kept, "keeping Thursday does not decide Monday")
	assert.True(t, after[4].Kept)
}

func TestBookingLeftoversKeepRefusals(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2d", 1)
	template := s.createHandTemplate(t, "Hausaufgaben 2d", children)
	for name, body := range map[string]map[string]any{
		"no weekdays":    {"weekdays": []int{}, "booking_ended_on": "2030-03-01", "kept": true},
		"no kept":        {"weekdays": []int{4}, "booking_ended_on": "2030-03-01"},
		"bad date":       {"weekdays": []int{4}, "booking_ended_on": "01.03.2030", "kept": true},
		"no iso weekday": {"weekdays": []int{8}, "booking_ended_on": "2030-03-01", "kept": true},
	} {
		w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/students/%d/kept-days", template, children[0]), body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: body=%s", name, w.Body.String())
	}
	w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/students/%d/kept-days", template+100000, children[0]),
		map[string]any{"weekdays": []int{4}, "booking_ended_on": "2030-03-01", "kept": true})
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

// TestRemoveStudentTemplateDaysLeavesToday: a removal asked for today
// starts tomorrow; today's occurrence keeps the child.
func TestRemoveStudentTemplateDaysLeavesToday(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2e", 1)
	template := s.createHandTemplate(t, "Hausaufgaben 2e", children)
	occurrence := func(date calendar.Date) int64 {
		instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
			ActivityGroupID: &template, CalendarPeriodID: &s.period.ID, Title: "Hausaufgaben 2e",
		})
		testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, children[0], "")
		return instance.ID
	}
	today, nextMonday := occurrence(rosterReviewMonday), occurrence(rosterReviewMonday.AddDays(7))

	w := doTemplateJSON(t, s.router, http.MethodPost, fmt.Sprintf("/templates/%d/students/%d/remove-days", template, children[0]), map[string]any{
		"weekdays": []int{timetableModule.WeekdayMonday}, "effective_from": rosterReviewMonday.String(),
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, []int64{children[0]}, s.occurrenceStudents(t, today), "today stays as it is")
	assert.Empty(t, s.occurrenceStudents(t, nextMonday), "next Monday loses the child")
}

func TestBookingLeftoversQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := buildBookingLeftoversModule(t)
	counter := testpkg.CaptureQueries(t, s.db)
	measured := chi.NewRouter()
	measured.Use(render.SetContentType(render.ContentTypeJSON))
	measured.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(testpkg.WithTestTenantRuntime(t, r.Context())))
		})
	})
	measured.Get("/templates/booking-leftovers", s.res.listBookingLeftovers)
	router := testpkg.MeasuredTenantRequest(t, s.db, testpkg.TenantIDFromContext(s.ctx), counter, measured)

	created := 0
	add := func(n int) {
		children := createClassStudents(t, s.db, "3a", 2*n)
		for i := range n {
			s.createHandTemplate(t, fmt.Sprintf("Budget-Hausaufgaben-%d", created), children[2*i:2*i+2])
			s.sources.bookings = append(s.sources.bookings, thursdayDroppedBookings(children[2*i], children[2*i+1], rosterReviewMonday.AddDays(-3))...)
			created++
		}
	}
	run := func() int {
		counter.Reset()
		w := doTemplateJSON(t, router, http.MethodGet, fmt.Sprintf("/templates/booking-leftovers?period_id=%d", s.period.ID), nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.Len(t, decodeTemplateData[bookingLeftoversResponse](t, w).Leftovers, created)
		return counter.Total()
	}

	add(2)
	small := run()
	add(4)
	large := run()

	t.Logf("query budget: 2 templates → %d queries, 6 templates → %d queries", small, large)
	assert.Equal(t, small, large, "query count must not grow with the number of Regeltermine and children")
	querybudgets.AssertQueryBudget(t, "api.timetable.templates.booking_leftovers", counter.Queries())
}
