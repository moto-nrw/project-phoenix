package timetablehttp

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

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
	// announced records the planning wake-ups the commands queue.
	announced *leftoverAnnouncements
}

// leftoverAnnouncements records each committed wake-up's source.
type leftoverAnnouncements struct {
	mu      sync.Mutex
	sources []string
}

func (a *leftoverAnnouncements) AnnounceStaffingChanged(_ int64, source string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sources = append(a.sources, source)
	return nil
}

func (a *leftoverAnnouncements) list() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.sources...)
}

func buildBookingLeftoversModule(t *testing.T, calendars ...timetableModule.NonWorkingDayCalendar) *bookingLeftoversSetup {
	t.Helper()
	sources := &rosterMaintenanceSources{results: map[int64]timetableModule.TemplateRosterMaintenance{}}
	s := buildRosterReviewModule(t, sources)
	var days timetableModule.NonWorkingDayCalendar
	if len(calendars) > 0 {
		days = calendars[0]
	}
	// The hint takes the dates materialization plans, so it runs against the
	// real materialization over the same database.
	s.res.MaterializationService = testTimetableWith(s.db, testTimetableOptions{
		validateCareOfferingSeries: acceptCareOfferingSeries,
		validateOfferingSource:     func(context.Context, []int64, []int64, *int64) error { return nil },
		resyncOfferingRoster:       s.fakeResync,
		nonWorkingDays:             days,
	}, rosterReviewClock).materialization
	s.router.Put("/templates/{id}", s.res.updateTemplate)
	setup := &bookingLeftoversSetup{rosterReviewSetup: s, sources: sources, announced: &leftoverAnnouncements{}}
	s.res.StudentTemplateDays = RootCompositions.StudentTemplateDays(s.db, func() calendar.Date { return rosterReviewMonday }, setup.announced)
	s.router.Get("/templates/booking-leftovers", s.res.listBookingLeftovers)
	s.router.Put("/templates/{id}/students/{studentId}/kept-days", s.res.setKeptStudentTemplateDays)
	s.router.Post("/templates/{id}/students/{studentId}/remove-days", s.res.removeStudentTemplateDays)
	period := createTemplateTestPeriodRange(t, s.db, "Leftover-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	setup.period = period
	return setup
}

// createHandTemplate creates a care block Monday to Friday 14:00–16:00
// whose children staff picked by hand, without a Klasse target.
func (s *bookingLeftoversSetup) createHandTemplate(t *testing.T, name string, studentIDs []int64) int64 {
	t.Helper()
	w := doTemplateJSON(t, s.router, http.MethodPost, "/templates", s.handTemplateBody(name, studentIDs))
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[createTemplateResponse](t, w).TemplateID
}

// handTemplateBody is the create, update and split body of the hand
// Regeltermin.
func (s *bookingLeftoversSetup) handTemplateBody(name string, studentIDs []int64) map[string]any {
	return map[string]any{
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

// TestBookingLeftoversRemoveOnlyTheGap is #5497 end to end: the Thursday
// booking ends on the change day and the same offering is booked again two
// weeks later. The hint names the gap with its end; „Entfernen“ takes the
// child off the Thursday in the gap and keeps it planned from the rebooking
// on, in the hand rows and in the occurrences created already.
func TestBookingLeftoversRemoveOnlyTheGap(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2g", 1)
	child := children[0]
	template := s.createHandTemplate(t, "Hausaufgaben 2g", children)
	start, end := rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(121)
	change, rebooked := rosterReviewMonday.AddDays(-3), rosterReviewMonday.AddDays(14)
	window := func(from, until calendar.Date, ended bool, weekdays ...int) timetableModule.OfferingBookingWindow {
		return timetableModule.OfferingBookingWindow{ValidFrom: &from, ValidUntil: &until, Weekdays: weekdays, Ended: ended}
	}
	s.sources.bookings = []timetableModule.OfferingWeekdayBooking{{
		OfferingID: leftoverGanztagID, OfferingName: "Ganztag", StudentID: child, CountsAsCare: true, Weekdays: []int{1, 2, 3, 4, 5},
		Windows: []timetableModule.OfferingBookingWindow{
			window(start, change, true, 1, 2, 3, 4, 5), window(change, end, false, 1, 2, 3, 5), window(rebooked, end, false, 4),
		},
	}}
	occurrence := func(date calendar.Date) int64 {
		instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
			ActivityGroupID: &template, CalendarPeriodID: &s.period.ID, Title: "Hausaufgaben 2g",
		})
		testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, child, "")
		return instance.ID
	}
	inGap, afterRebooking := occurrence(rosterReviewMonday.AddDays(10)), occurrence(rosterReviewMonday.AddDays(17))

	leftovers := s.listLeftovers(t)
	require.Len(t, leftovers, 1, "%+v", leftovers)
	got := leftovers[0]
	assert.Equal(t, rosterReviewMonday.AddDays(3).String(), got.From)
	require.NotNil(t, got.Until, "the rebooking ends the gap")
	assert.Equal(t, rebooked.String(), *got.Until)

	w := doTemplateJSON(t, s.router, http.MethodPost, fmt.Sprintf("/templates/%d/students/%d/remove-days", template, child), map[string]any{
		"weekdays": got.Weekdays, "effective_from": got.From, "effective_until": *got.Until,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, s.occurrenceStudents(t, inGap), "the Thursday in the gap loses the child")
	assert.Equal(t, []int64{child}, s.occurrenceStudents(t, afterRebooking), "the Thursday the new booking carries keeps the child")
	assert.Empty(t, s.listLeftovers(t), "after the removal nothing is left")

	rosters, err := s.res.RosterReview.PlannedRosters(testpkg.Ctx(t), []int64{template})
	require.NoError(t, err)
	for _, day := range []struct {
		date    calendar.Date
		planned bool
	}{{rosterReviewMonday.AddDays(10), false}, {rosterReviewMonday.AddDays(17), true}, {rosterReviewMonday.AddDays(11), true}} {
		assert.Equal(t, day.planned, handRowPlans(rosters[template], child, day.date), "hand rows on %s", day.date)
	}
}

// handRowPlans reports whether a hand row of the roster plans the child on
// the date by validity and weekday scope.
func handRowPlans(roster timetableModule.TemplatePlannedRoster, studentID int64, date calendar.Date) bool {
	weekday := calendar.ISOWeekday(date)
	for _, entry := range roster.Entries {
		switch {
		case entry.StudentID != studentID, entry.FromBooking,
			entry.ValidFrom != nil && entry.ValidFrom.After(date),
			entry.ValidUntil != nil && !entry.ValidUntil.After(date),
			entry.Weekday != nil && *entry.Weekday != weekday,
			len(entry.SelectedWeekdays) > 0 && !slices.Contains(entry.SelectedWeekdays, weekday):
			continue
		}
		return true
	}
	return false
}

// TestRemoveStudentTemplateDaysRefusesAnEndBeforeTheStart: the end must lie
// after the first removed day.
func TestRemoveStudentTemplateDaysRefusesAnEndBeforeTheStart(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2h", 1)
	template := s.createHandTemplate(t, "Hausaufgaben 2h", children)
	for name, until := range map[string]string{
		"same day": rosterReviewMonday.AddDays(3).String(),
		"before":   rosterReviewMonday.AddDays(1).String(),
		"bad date": "19.11.2030",
	} {
		w := doTemplateJSON(t, s.router, http.MethodPost, fmt.Sprintf("/templates/%d/students/%d/remove-days", template, children[0]), map[string]any{
			"weekdays": []int{timetableModule.WeekdayThursday}, "effective_from": rosterReviewMonday.AddDays(3).String(), "effective_until": until,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: body=%s", name, w.Body.String())
	}
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

// leftoverClosedThursday closes the first Thursday after the fixed Monday.
type leftoverClosedThursday struct{}

func (leftoverClosedThursday) TenantHolidayDates(context.Context, string, string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (leftoverClosedThursday) ClosingDayDates(context.Context, string, string) (map[string]bool, error) {
	return map[string]bool{rosterReviewMonday.AddDays(3).String(): true}, nil
}

// TestBookingLeftoversTakeTheMaterializedDays (quorum review round 3 of
// #5452): a closed Thursday is no occurrence, so the gap starts on the next
// Thursday materialization plans.
func TestBookingLeftoversTakeTheMaterializedDays(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t, leftoverClosedThursday{})
	children := createClassStudents(t, s.db, "2g", 2)
	s.createHandTemplate(t, "Hausaufgaben 2g", children)
	s.sources.bookings = thursdayDroppedBookings(children[0], children[1], rosterReviewMonday.AddDays(-3))

	leftovers := s.listLeftovers(t)
	require.Len(t, leftovers, 1)
	assert.Equal(t, rosterReviewMonday.AddDays(10).String(), leftovers[0].From, "the first Thursday is a closing day")
}

// TestBookingLeftoversKeepSurvivesSeriesEdits (quorum review round 3 of
// #5452): saving the Regeltermin and splitting it rewrite the hand rows;
// the school's „Bleibt so“ stays decided.
func TestBookingLeftoversKeepSurvivesSeriesEdits(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2h", 2)
	template := s.createHandTemplate(t, "Hausaufgaben 2h", children)
	s.sources.bookings = thursdayDroppedBookings(children[0], children[1], rosterReviewMonday.AddDays(-3))
	leftover := s.listLeftovers(t)[0]
	w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/students/%d/kept-days", template, children[0]), map[string]any{
		"weekdays": leftover.Weekdays, "booking_ended_on": leftover.BookingEndedOn, "kept": true,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	body := s.handTemplateBody("Hausaufgaben 2h neu", children)
	w = doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d", template), body)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	for _, got := range s.listLeftovers(t) {
		assert.True(t, got.Kept, "after saving the series: %+v", got)
	}

	body["effective_date"] = rosterReviewMonday.AddDays(14).String()
	w = doTemplateJSON(t, s.router, http.MethodPost, fmt.Sprintf("/templates/%d/split", template), body)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	after := s.listLeftovers(t)
	require.NotEmpty(t, after)
	for _, got := range after {
		assert.True(t, got.Kept, "after splitting the series: %+v", got)
	}
}

// TestBookingLeftoversConcurrentKeepsBothSurvive (quorum review round 3 of
// #5452): two decisions for different weekdays wait on the same row; both
// must be stored, neither overwrites the other.
func TestBookingLeftoversConcurrentKeepsBothSurvive(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2i", 1)
	template := s.createHandTemplate(t, "Hausaufgaben 2i", children)
	ctx := context.Background()
	var rowID int64
	require.NoError(t, s.db.NewSelect().TableExpr("activities.student_enrollments").Column("id").
		Where("activity_group_id = ? AND student_id = ?", template, children[0]).Scan(ctx, &rowID))

	lock, err := s.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = lock.ExecContext(ctx, "SELECT id FROM activities.student_enrollments WHERE id = ? FOR UPDATE", rowID)
	require.NoError(t, err)

	codes := make(chan int, 2)
	for _, weekday := range []int{1, 4} {
		go func(weekday int) {
			w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/students/%d/kept-days", template, children[0]), map[string]any{
				"weekdays": []int{weekday}, "booking_ended_on": "2030-03-01", "kept": true,
			})
			codes <- w.Code
		}(weekday)
	}
	// One decision waits on the row; the other waits there too or on the
	// tenant recurrence gate the first one holds.
	require.Eventually(t, func() bool {
		var waiting int
		err := s.db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND query ILIKE '%kept_without_booking%'`).Scan(ctx, &waiting)
		return err == nil && waiting >= 1
	}, 10*time.Second, 20*time.Millisecond, "a decision waits on the row")
	require.NoError(t, lock.Commit())
	assert.Equal(t, http.StatusOK, <-codes)
	assert.Equal(t, http.StatusOK, <-codes)

	var kept string
	require.NoError(t, s.db.NewSelect().TableExpr("activities.student_enrollments").ColumnExpr("kept_without_booking::text").
		Where("id = ?", rowID).Scan(ctx, &kept))
	assert.JSONEq(t, `{"1": "2030-03-01", "4": "2030-03-01"}`, kept)
}

// TestBookingLeftoversKeepCoversEveryRow: the early gap sits on a hand row
// that ends before the later booking end; „Bleibt so“ with that later end
// must decide both rows.
func TestBookingLeftoversKeepCoversEveryRow(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2j", 1)
	child := children[0]
	template := s.createHandTemplate(t, "Hausaufgaben 2j", children)
	ctx := context.Background()
	split := rosterReviewMonday.AddDays(20)
	_, err := s.db.NewRaw(`UPDATE activities.student_enrollments SET valid_until = ?
		WHERE activity_group_id = ? AND student_id = ?`, split, template, child).Exec(ctx)
	require.NoError(t, err)
	_, err = s.db.NewRaw(`INSERT INTO activities.student_enrollments (tenant_id, student_id, activity_group_id, valid_from, calendar_period_id)
		SELECT tenant_id, student_id, activity_group_id, ?, calendar_period_id FROM activities.student_enrollments
		WHERE activity_group_id = ? AND student_id = ?`, split, template, child).Exec(ctx)
	require.NoError(t, err)
	start, end := rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(121)
	mensaFrom, mensaEnds := rosterReviewMonday.AddDays(10), rosterReviewMonday.AddDays(30)
	window := func(from, until calendar.Date, ended bool, weekdays ...int) timetableModule.OfferingBookingWindow {
		return timetableModule.OfferingBookingWindow{ValidFrom: &from, ValidUntil: &until, Weekdays: weekdays, Ended: ended}
	}
	s.sources.bookings = []timetableModule.OfferingWeekdayBooking{
		{OfferingID: leftoverGanztagID, StudentID: child, CountsAsCare: true, Weekdays: []int{4},
			Windows: []timetableModule.OfferingBookingWindow{window(start, rosterReviewMonday.AddDays(-3), true, 4)}},
		{OfferingID: leftoverGanztagID + 1, StudentID: child, CountsAsCare: true, Weekdays: []int{4},
			Windows: []timetableModule.OfferingBookingWindow{window(mensaFrom, mensaEnds, true, 4)}},
		{OfferingID: leftoverGanztagID + 2, StudentID: child, CountsAsCare: true, Weekdays: []int{1},
			Windows: []timetableModule.OfferingBookingWindow{window(start, end, false, 1)}},
	}
	leftovers := s.listLeftovers(t)
	require.Len(t, leftovers, 1, "%+v", leftovers)
	assert.Equal(t, rosterReviewMonday.AddDays(3).String(), leftovers[0].From, "the first gap is on the first row")
	assert.Equal(t, mensaEnds.String(), leftovers[0].BookingEndedOn)

	w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/students/%d/kept-days", template, child), map[string]any{
		"weekdays": leftovers[0].Weekdays, "booking_ended_on": leftovers[0].BookingEndedOn, "kept": true,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, 2, decodeTemplateData[keptStudentTemplateDaysResponse](t, w).RosterRows, "both hand rows plan Thursday")
	after := s.listLeftovers(t)
	require.Len(t, after, 1)
	assert.True(t, after[0].Kept)
}

// TestBookingLeftoversAnnounceTheirWrites (quorum review round 4 of #5452):
// „Bleibt so“, its undo and „Entfernen“ wake the other tabs after commit, so
// an open home page or Betreuungsplan refetches the hint.
func TestBookingLeftoversAnnounceTheirWrites(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2k", 1)
	template := s.createHandTemplate(t, "Hausaufgaben 2k", children)
	for _, kept := range []bool{true, false} {
		w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d/students/%d/kept-days", template, children[0]), map[string]any{
			"weekdays": []int{4}, "booking_ended_on": "2030-03-01", "kept": kept,
		})
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	}
	w := doTemplateJSON(t, s.router, http.MethodPost, fmt.Sprintf("/templates/%d/students/%d/remove-days", template, children[0]), map[string]any{
		"weekdays": []int{4}, "effective_from": rosterReviewMonday.AddDays(1).String(),
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, []string{"student_template_days_keep", "student_template_days_keep", "student_template_days_remove"}, s.announced.list())
}

// TestBookingLeftoversReadEndsBeforeThePeriod (quorum review round 4 of
// #5452): a booking that ended before the planning period started still
// leaves its hand row planned in the period, so the read must ask Care Plan
// for links that ended before the period start.
func TestBookingLeftoversReadEndsBeforeThePeriod(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	children := createClassStudents(t, s.db, "2l", 2)
	s.createHandTemplate(t, "Hausaufgaben 2l", children)
	beforePeriod := s.period.StartDate.AddDays(-10)
	s.sources.bookings = thursdayDroppedBookings(children[0], children[1], beforePeriod)

	require.Len(t, s.listLeftovers(t), 1)
	require.NotEmpty(t, s.sources.bookingsFrom)
	earliest := slices.MinFunc(s.sources.bookingsFrom, func(a, b calendar.Date) int { return a.Compare(b) })
	assert.True(t, earliest.Before(beforePeriod), "bookings are read from %s, the ended link is lost", earliest)
}

// TestBookingLeftoversProjectALaterPeriod (quorum review round 4 of #5452):
// a planning period that starts long after tomorrow is projected from its
// own start; from tomorrow the window would exceed the projection's bound and
// fail the read.
func TestBookingLeftoversProjectALaterPeriod(t *testing.T) {
	t.Parallel()
	s := buildBookingLeftoversModule(t)
	later := createTemplateTestPeriodRange(t, s.db, "Leftover-Later", rosterReviewMonday.AddDays(140), rosterReviewMonday.AddDays(480), 1, nil)
	children := createClassStudents(t, s.db, "2m", 2)
	body := s.handTemplateBody("Hausaufgaben 2m", children)
	body["calendar_period_id"] = later.ID
	w := doTemplateJSON(t, s.router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	s.sources.bookings = thursdayDroppedBookings(children[0], children[1], rosterReviewMonday.AddDays(-3))

	w = doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/templates/booking-leftovers?period_id=%d", later.ID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}
