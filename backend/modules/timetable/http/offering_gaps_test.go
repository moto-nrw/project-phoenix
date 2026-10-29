package timetablehttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// offeringGapsSetup drives the offering-gap routes (#5140) over the real
// template list. The offering side is the stub: its feeds name which
// offerings reach each Regeltermin, and its decisions travel with the
// bookings like Care Plan's column.
type offeringGapsSetup struct {
	*rosterReviewSetup
	sources *rosterMaintenanceSources
	router  chi.Router
}

func buildOfferingGapsModule(t *testing.T, calendars ...timetableModule.NonWorkingDayCalendar) *offeringGapsSetup {
	t.Helper()
	sources := &rosterMaintenanceSources{results: map[int64]timetableModule.TemplateRosterMaintenance{}}
	s := buildRosterReviewModule(t, sources)
	s.res.OfferingGapDecisions = sources
	var days timetableModule.NonWorkingDayCalendar
	if len(calendars) > 0 {
		days = calendars[0]
	}
	// The gap check compares with what materialization would create, so it
	// runs against the real materialization over the same database.
	s.res.MaterializationService = testTimetableWith(s.db, testTimetableOptions{
		validateCareOfferingSeries: acceptCareOfferingSeries,
		validateOfferingSource:     func(context.Context, []int64, []int64, *int64) error { return nil },
		resyncOfferingRoster:       s.fakeResync,
		nonWorkingDays:             days,
	}, rosterReviewClock).materialization
	s.router.Get("/offering-gaps", s.res.listOfferingGaps)
	s.router.Put("/offering-gaps/{offeringId}/acknowledgement", s.res.setOfferingGapAck)
	return &offeringGapsSetup{rosterReviewSetup: s, sources: sources, router: s.router}
}

// createSourcedTemplate creates a Monday/Wednesday Regeltermin sourced from
// the offering for the given Jahrgänge and tells the stub that the offering
// feeds it.
func (s *offeringGapsSetup) createSourcedTemplate(t *testing.T, name string, periodID, offeringID int64, grades []int, endDate ...calendar.Date) int64 {
	t.Helper()
	body := map[string]any{
		"name":                     name,
		"type":                     timetableModule.GroupTypeCare,
		"weekdays":                 []int{timetableModule.WeekdayMonday, timetableModule.WeekdayWednesday},
		"start_time":               "07:00",
		"end_time":                 "08:00",
		"room_id":                  s.roomID,
		"category_id":              s.category.ID,
		"calendar_period_id":       periodID,
		"target_group_type":        timetableModule.TargetGroupTypeOffering,
		"source_care_offering_ids": []int64{offeringID},
		"source_grade_levels":      grades,
		"student_ids":              []int64{},
		"staff_ids":                []int64{s.staffA},
	}
	if len(endDate) > 0 {
		body["end_date"] = endDate[0].String()
	}
	w := doTemplateJSON(t, s.router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	id := decodeTemplateData[createTemplateResponse](t, w).TemplateID
	s.sources.results[id] = timetableModule.TemplateRosterMaintenance{
		Mode: "automatic", Offerings: []timetableModule.OfferingRef{{ID: offeringID, Name: name}}, GradeLevels: grades,
	}
	return id
}

func (s *offeringGapsSetup) listGaps(t *testing.T, periodID int64) []offeringGapResponse {
	t.Helper()
	w := doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/offering-gaps?period_id=%d", periodID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[offeringGapsResponse](t, w).Gaps
}

func (s *offeringGapsSetup) decide(t *testing.T, offeringID, periodID int64, acknowledged bool) *httptest.ResponseRecorder {
	t.Helper()
	return doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/offering-gaps/%d/acknowledgement", offeringID),
		map[string]any{"period_id": periodID, "acknowledged": acknowledged})
}

func withGrade(bookings []timetableModule.OfferingWeekdayBooking, grade int) []timetableModule.OfferingWeekdayBooking {
	for i := range bookings {
		bookings[i].GradeLevel = &grade
	}
	return bookings
}

// TestOfferingGapsSchuleAmBerg walks the decision of #5140: Randstunde has a
// Regeltermin for Jahrgang 1 and 2, so the booked children of 3a show up.
// "Kein eigener Regeltermin nötig" stores the classes the hint names and
// hides it; a booked child of 4a brings it back; withdrawing shows it again.
func TestOfferingGapsSchuleAmBerg(t *testing.T) {
	t.Parallel()
	s := buildOfferingGapsModule(t)
	period := createTemplateTestPeriodRange(t, s.db, "Gap-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	randstunde := rosterReviewOfferingID
	s.sources.bookings = append(
		withGrade(rosterReviewBookings(randstunde, "Randstunde", "1a", createClassStudents(t, s.db, "1a", 2), 1, 3), 1),
		withGrade(rosterReviewBookings(randstunde, "Randstunde", "3a", createClassStudents(t, s.db, "3a", 3), 1, 3), 3)...)
	s.createSourcedTemplate(t, "Randstunde 1/2", period.ID, randstunde, []int{1, 2})

	gaps := s.listGaps(t, period.ID)
	require.Len(t, gaps, 1, "%+v", gaps)
	assert.Equal(t, offeringGapResponse{
		Offering:     templateRosterMaintenanceOfferingResponse{ID: randstunde, Name: "Randstunde"},
		StudentCount: 3, SchoolClasses: []string{"3a"}, Weekdays: []int{1, 3}, AcknowledgedClasses: []string{},
	}, gaps[0])

	w := s.decide(t, randstunde, period.ID, true)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, offeringGapAckResponse{Acknowledged: true, AcknowledgedClasses: []string{"3a"}}, decodeTemplateData[offeringGapAckResponse](t, w))
	assert.Equal(t, []string{"3a"}, s.sources.decided(randstunde, period.ID), "the server's classes are stored")
	gaps = s.listGaps(t, period.ID)
	require.Len(t, gaps, 1)
	assert.True(t, gaps[0].Acknowledged, "the decision hides the hint")

	s.sources.bookings = append(s.sources.bookings,
		withGrade(rosterReviewBookings(randstunde, "Randstunde", "4a", createClassStudents(t, s.db, "4a", 1), 3), 4)...)
	gaps = s.listGaps(t, period.ID)
	require.Len(t, gaps, 1)
	assert.False(t, gaps[0].Acknowledged, "a child of another class brings the hint back")
	assert.Equal(t, []string{"3a", "4a"}, gaps[0].SchoolClasses)
	assert.Equal(t, []string{"3a"}, gaps[0].AcknowledgedClasses)

	w = s.decide(t, randstunde, period.ID, true)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.True(t, s.listGaps(t, period.ID)[0].Acknowledged, "deciding again covers the new class")

	w = s.decide(t, randstunde, period.ID, false)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, s.sources.decided(randstunde, period.ID))
	assert.False(t, s.listGaps(t, period.ID)[0].Acknowledged, "withdrawing shows the hint again")
}

// TestOfferingGapsRefusals: a decision needs an open gap and a complete
// request; a failed offering read fails the list instead of flagging every
// booking.
func TestOfferingGapsRefusals(t *testing.T) {
	t.Parallel()
	s := buildOfferingGapsModule(t)
	period := createTemplateTestPeriodRange(t, s.db, "Gap-Refusals", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	randstunde := rosterReviewOfferingID
	s.sources.bookings = withGrade(rosterReviewBookings(randstunde, "Randstunde", "2b", createClassStudents(t, s.db, "2b", 2), 1, 3), 2)
	s.createSourcedTemplate(t, "Randstunde 1/2", period.ID, randstunde, []int{1, 2})
	require.Empty(t, s.listGaps(t, period.ID))

	w := s.decide(t, randstunde, period.ID, true)
	assert.Equal(t, http.StatusNotFound, w.Code, "every booked child has a Regeltermin: body=%s", w.Body.String())
	assert.Empty(t, s.sources.decisions)
	w = doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/offering-gaps/%d/acknowledgement", randstunde), map[string]any{"period_id": period.ID})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	w = doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/offering-gaps/%d/acknowledgement", randstunde), map[string]any{"acknowledged": true})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	w = doTemplateJSON(t, s.router, http.MethodGet, "/offering-gaps", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	s.sources.err = errors.New("offering side down")
	w = doTemplateJSON(t, s.router, http.MethodGet, fmt.Sprintf("/offering-gaps?period_id=%d", period.ID), nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
}

func TestOfferingGapsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := buildOfferingGapsModule(t)
	period := createTemplateTestPeriodRange(t, s.db, "Gap-Budget", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	s.sources.bookings = withGrade(rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "3c", createClassStudents(t, s.db, "3c", 4), 1, 3), 3)
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
			s.createSourcedTemplate(t, fmt.Sprintf("Budget-Gap-%d", created), period.ID, rosterReviewOfferingID, []int{1, 2})
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
	testpkg.AssertQueryBudget(t, "api.timetable.offering_gaps", counter.Queries())
}
