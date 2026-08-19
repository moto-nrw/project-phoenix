package timetable

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Issue #2226: PUT /timetable/templates/{id} accepts an optional start_date
// that pulls a not-yet-started series forward. The tests below pin the wire
// contract: acceptance moves the schedule envelope, the rejections respond
// with stable codes and user-facing German messages.

// futureMondayForPull returns a Monday at least eight days ahead so the old
// and the pulled-forward start both stay strictly in the future on every
// weekday the suite runs.
func futureMondayForPull() timezone.Date {
	today := timezone.TodayDate()
	daysAhead := (int(time.Monday) - int(today.Weekday()) + 7) % 7
	if daysAhead == 0 {
		daysAhead = 7
	}
	return today.AddDays(daysAhead + 7)
}

func TestTemplateUpdateStartDatePullForward(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	newStart := futureMondayForPull()
	oldStart := newStart.AddDays(7)

	// The period must span both starts; widen it a year past the old start.
	period := createTemplateTestPeriodRange(
		t, s.db, "TplStartPullPeriod",
		timezone.NewDate(newStart.Year, 1, 1),
		oldStart.AddDays(365),
		1, nil,
	)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID)
	})

	createBody := createTemplateBody(s, "Tpl-StartPull")
	createBody["weekdays"] = []int{1}
	createBody["calendar_period_id"] = period.ID
	createBody["start_date"] = oldStart.String()
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", createBody)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	updateBody := func(startDate string) map[string]any {
		body := createTemplateBody(s, "Tpl-StartPull")
		body["weekdays"] = []int{1}
		body["calendar_period_id"] = period.ID
		if startDate != "" {
			body["start_date"] = startDate
		}
		return body
	}
	putPath := fmt.Sprintf("/templates/%d", created.TemplateID)

	// Later than the stored start → German rejection with a stable code.
	w = doTemplateJSON(t, router, http.MethodPut, putPath, updateBody(oldStart.AddDays(7).String()))
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), ErrCodeTemplateStartNotEarlier)
	assert.Contains(t, w.Body.String(), "vorgezogen")

	// In the past → German rejection with a stable code.
	w = doTemplateJSON(t, router, http.MethodPut, putPath, updateBody(timezone.TodayDate().AddDays(-1).String()))
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), ErrCodeTemplateStartInPast)
	assert.Contains(t, w.Body.String(), "Vergangenheit")

	// Outside the pinned period → the shared create/update preflight rejects.
	outsideWithoutPeriod := updateBody(timezone.NewDate(newStart.Year-1, 12, 31).String())
	delete(outsideWithoutPeriod, "calendar_period_id")
	w = doTemplateJSON(t, router, http.MethodPut, putPath, outsideWithoutPeriod)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "start_date must lie within the calendar period")

	// Bad format → precise 400.
	w = doTemplateJSON(t, router, http.MethodPut, putPath, updateBody("13.08.2026"))
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "invalid start_date format")

	// An earlier date within the period pulls the envelope forward.
	pullWithoutPeriod := updateBody(newStart.String())
	delete(pullWithoutPeriod, "calendar_period_id")
	w = doTemplateJSON(t, router, http.MethodPut, putPath, pullWithoutPeriod)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	updated := decodeTemplateData[templateResponse](t, w)
	require.NotNil(t, updated.CalendarPeriodID)
	assert.Equal(t, period.ID, *updated.CalendarPeriodID, "omitted period must keep the stored pin")
	require.NotEmpty(t, updated.Schedules)
	for _, sched := range updated.Schedules {
		assert.Equal(t, newStart.String(), sched.ValidFrom, "schedule must carry the pulled-forward start")
	}
	assertTemplateRosterValidFrom(t, s, created.TemplateID, newStart)

	// Omitting start_date keeps the (moved) envelope untouched.
	w = doTemplateJSON(t, router, http.MethodPut, putPath, updateBody(""))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	afterOmit := decodeTemplateData[templateResponse](t, w)
	require.NotEmpty(t, afterOmit.Schedules)
	for _, sched := range afterOmit.Schedules {
		assert.Equal(t, newStart.String(), sched.ValidFrom, "an omitted start_date must not change the envelope")
	}
}
