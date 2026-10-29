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
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildClassDayGapsModule is the offering-gap module with Timetable's real
// class arrival plans (#5343).
func buildClassDayGapsModule(t *testing.T) (*offeringGapsSetup, timetableModule.ClassArrivals) {
	t.Helper()
	s := buildOfferingGapsModule(t)
	arrivals, err := timetableCompose.NewClassArrivals(s.db, func(timetableCompose.Observation) {})
	require.NoError(t, err)
	s.res.ClassArrivals = arrivals
	return s, arrivals
}

func saveClassArrivals(t *testing.T, s *offeringGapsSetup, arrivals timetableModule.ClassArrivals, class string, times map[string]string) {
	t.Helper()
	_, err := arrivals.UpsertClassArrivalPlan(s.ctx, timetableModule.ClassArrivalPlanInput{SchoolClass: class, ArrivalTimes: times})
	require.NoError(t, err)
}

// createKlassenRandstunde creates a Klasse Regeltermin without offering
// source from 11:45 to 12:45; its target holds the class.
func (s *offeringGapsSetup) createKlassenRandstunde(t *testing.T, class string, periodID int64, weekdays ...int) int64 {
	t.Helper()
	body := map[string]any{
		"name":                "Randstunde " + class,
		"type":                timetableModule.GroupTypeCare,
		"weekdays":            weekdays,
		"start_time":          "11:45",
		"end_time":            "12:45",
		"room_id":             s.roomID,
		"category_id":         s.category.ID,
		"calendar_period_id":  periodID,
		"target_group_type":   timetableModule.TargetGroupTypeSchoolClass,
		"target_school_class": class,
		"student_ids":         []int64{},
		"staff_ids":           []int64{s.staffA},
	}
	w := doTemplateJSON(t, s.router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[createTemplateResponse](t, w).TemplateID
}

// classDayRandstunde books Randstunde (pickup 12:45) as a school-day
// offering; without pickup times the Klassentag cannot apply.
func classDayRandstunde(class string, studentIDs []int64, withPickup bool, weekdays ...int) []timetableModule.OfferingWeekdayBooking {
	bookings := rosterReviewBookings(rosterReviewOfferingID, "Randstunde", class, studentIDs, weekdays...)
	for i := range bookings {
		bookings[i].ClassDayApplies = true
		if withPickup {
			bookings[i].PickupTimes = map[int]string{1: "12:45", 2: "12:45", 3: "12:45"}
		}
	}
	return bookings
}

// TestOfferingGapsClassDay walks #5343 over real Regeltermine and the real
// Klassentag: Klasse 1a has a Klassen-Randstunde on Monday and Wednesday and
// arrives at 11:45 on Monday to Wednesday, Klasse 3a arrives at 12:45, the
// Randstunde's pickup. Without pickup times the strict day rule shows 1a on
// Tuesday and 3a on every day, and creating a Regeltermin would put 1a on a
// second list. The school decides that these children need none. With the
// pickup times only 1a on Tuesday is left, a new Regeltermin for 1a on
// Tuesday plans nobody twice, and the stored decision still covers it.
func TestOfferingGapsClassDay(t *testing.T) {
	t.Parallel()
	s, arrivals := buildClassDayGapsModule(t)
	period := createTemplateTestPeriodRange(t, s.db, "Gap-ClassDay", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class1a := createClassStudents(t, s.db, "1a", 3)
	class3a := createClassStudents(t, s.db, "3a", 2)
	saveClassArrivals(t, s, arrivals, "1a", map[string]string{"mon": "11:45", "tue": "11:45", "wed": "11:45"})
	saveClassArrivals(t, s, arrivals, "3a", map[string]string{"mon": "12:45", "tue": "12:45", "wed": "12:45"})
	s.createKlassenRandstunde(t, "1a", period.ID, timetableModule.WeekdayMonday, timetableModule.WeekdayWednesday)

	s.sources.bookings = append(withGrade(classDayRandstunde("1a", class1a, false, 1, 2, 3), 1), withGrade(classDayRandstunde("3a", class3a, false, 1, 2, 3), 3)...)
	gaps := s.listGaps(t, period.ID)
	require.Len(t, gaps, 1, "%+v", gaps)
	assert.Equal(t, offeringGapResponse{
		Offering:     templateRosterMaintenanceOfferingResponse{ID: rosterReviewOfferingID, Name: "Randstunde"},
		StudentCount: 5, SchoolClasses: []string{"1a", "3a"}, Weekdays: []int{1, 2, 3}, AcknowledgedClasses: []string{},
		NewSeriesClasses: []string{"1a", "3a"}, NewSeriesPlansCovered: true,
	}, gaps[0], "without pickup times the Klassen-Randstunde covers 1a on its own days only")

	w := s.decide(t, rosterReviewOfferingID, period.ID, true)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	s.sources.bookings = append(withGrade(classDayRandstunde("1a", class1a, true, 1, 2, 3), 1), withGrade(classDayRandstunde("3a", class3a, true, 1, 2, 3), 3)...)
	for i := range s.sources.bookings {
		s.sources.bookings[i].TemplateGapAckClasses = s.sources.decided(rosterReviewOfferingID, period.ID)
	}
	gaps = s.listGaps(t, period.ID)
	require.Len(t, gaps, 1, "%+v", gaps)
	assert.Equal(t, offeringGapResponse{
		Offering:     templateRosterMaintenanceOfferingResponse{ID: rosterReviewOfferingID, Name: "Randstunde"},
		StudentCount: 3, SchoolClasses: []string{"1a"}, Weekdays: []int{2},
		Acknowledged: true, AcknowledgedClasses: []string{"1a", "3a"},
		NewSeriesClasses: []string{"1a"},
	}, gaps[0], "3a arrives at the pickup; the stored decision covers what is left")
}

// TestOfferingGapsClassDayMovedOccurrence: the check takes each date's
// time from the materializer, so a Klassen-Randstunde moved to the afternoon
// on one Monday leaves that Monday open (#5343). Cancelling it does too.
func TestOfferingGapsClassDayMovedOccurrence(t *testing.T) {
	t.Parallel()
	s, arrivals := buildClassDayGapsModule(t)
	period := createTemplateTestPeriodRange(t, s.db, "Gap-ClassDay-Moved", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class1a := createClassStudents(t, s.db, "1a", 2)
	saveClassArrivals(t, s, arrivals, "1a", map[string]string{"mon": "11:45", "wed": "11:45"})
	templateID := s.createKlassenRandstunde(t, "1a", period.ID, timetableModule.WeekdayMonday, timetableModule.WeekdayWednesday)
	s.sources.bookings = withGrade(classDayRandstunde("1a", class1a, true, 1, 3), 1)
	require.Empty(t, s.listGaps(t, period.ID), "the Klassen-Randstunde covers 1a")

	createTestActivityException(t, s.db, templateID, rosterReviewMonday.AddDays(14), timetableModule.ActivityExceptionModified, "14:00", "15:00", "Ausflug")
	gaps := s.listGaps(t, period.ID)
	require.Len(t, gaps, 1, "%+v", gaps)
	assert.Equal(t, []int{1}, gaps[0].Weekdays, "only the moved Monday is open")
	assert.Equal(t, 2, gaps[0].StudentCount)
	assert.True(t, gaps[0].NewSeriesPlansCovered, "a new Monday Regeltermin would plan 1a twice on the other Mondays")

	createTestActivityException(t, s.db, templateID, rosterReviewMonday.AddDays(23), timetableModule.ActivityExceptionCancelled, "", "", "Konferenz")
	gaps = s.listGaps(t, period.ID)
	require.Len(t, gaps, 1, "%+v", gaps)
	assert.Equal(t, []int{1, 3}, gaps[0].Weekdays, "the cancelled Wednesday is open as well")
}

// TestOfferingGapsClassDayQueryBudget measures the check with clock times:
// it adds the Klassentag (plans, changes) and one batch of rosters to the
// scenario without them, flat in the number of Regeltermine.
func TestOfferingGapsClassDayQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s, arrivals := buildClassDayGapsModule(t)
	period := createTemplateTestPeriodRange(t, s.db, "Gap-ClassDay-Budget", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	saveClassArrivals(t, s, arrivals, "2c", map[string]string{"mon": "11:45", "tue": "11:45", "wed": "11:45"})
	s.sources.bookings = withGrade(classDayRandstunde("2c", createClassStudents(t, s.db, "2c", 4), true, 1, 2, 3), 2)
	counter := testpkg.CaptureQueries(t, s.db)
	measured := chi.NewRouter()
	measured.Use(render.SetContentType(render.ContentTypeJSON))
	measured.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(testpkg.WithTestTenantRuntime(t, r.Context())))
		})
	})
	measured.Get("/offering-gaps", s.res.listOfferingGaps)
	router := testpkg.MeasuredTenantRequest(t, s.db, testpkg.TenantIDFromContext(s.ctx), counter, measured)

	created := 0
	addTemplates := func(n int) {
		for range n {
			s.createKlassenRandstunde(t, fmt.Sprintf("2c-%d", created), period.ID, timetableModule.WeekdayMonday, timetableModule.WeekdayWednesday)
			created++
		}
	}
	run := func() int {
		counter.Reset()
		w := doTemplateJSON(t, router, http.MethodGet, fmt.Sprintf("/offering-gaps?period_id=%d", period.ID), nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.Len(t, decodeTemplateData[offeringGapsResponse](t, w).Gaps, 1)
		return counter.Total()
	}

	addTemplates(3)
	small := run()
	addTemplates(5)
	large := run()

	t.Logf("query budget: 3 templates → %d queries, 8 templates → %d queries", small, large)
	assert.Equal(t, small, large, "query count must not grow with the number of Regeltermine")
	testpkg.AssertQueryBudget(t, "api.timetable.offering_gaps.class_day", counter.Queries())
}
