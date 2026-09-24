package timetable

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A series may end before its planning period (#3594): the planner names the
// inclusive last day on create, changes or removes it on a whole-series
// update, and reads it back from the template list.
func TestTemplateSeriesEndDateRoundTrip(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)
	period := createTemplateTestPeriod(t, s.db, "Tpl-Serienende")

	body := func(name string) map[string]any {
		b := createTemplateBody(s, name)
		b["calendar_period_id"] = period.ID
		b["start_date"] = "2026-10-19"
		return b
	}

	create := body("Tpl-Ferienwoche")
	create["end_date"] = "2026-10-23"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", create)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	endDateOf := func() *string {
		t.Helper()
		listW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
		require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
		for _, candidate := range decodeTemplateData[listTemplatesResponse](t, listW).Templates {
			if candidate.ID == created.TemplateID {
				return candidate.EndDate
			}
		}
		t.Fatalf("series %d missing from the list: an end date must not hide it", created.TemplateID)
		return nil
	}
	require.NotNil(t, endDateOf())
	assert.Equal(t, "2026-10-23", *endDateOf())

	update := func(b map[string]any) *httpResult {
		t.Helper()
		updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), b)
		return &httpResult{code: updateW.Code, body: updateW.Body.String()}
	}

	longer := body("Tpl-Ferienwoche")
	longer["end_date"] = "2026-10-30"
	res := update(longer)
	require.Equal(t, http.StatusOK, res.code, "body=%s", res.body)
	assert.Equal(t, "2026-10-30", *endDateOf(), "the end can move later")

	res = update(body("Tpl-Ferienwoche"))
	require.Equal(t, http.StatusOK, res.code, "body=%s", res.body)
	assert.Equal(t, "2026-10-30", *endDateOf(), "omitted keeps the stored end")

	cleared := body("Tpl-Ferienwoche")
	cleared["end_date"] = nil
	res = update(cleared)
	require.Equal(t, http.StatusOK, res.code, "body=%s", res.body)
	assert.Nil(t, endDateOf(), "null removes the end")
}

type httpResult struct {
	code int
	body string
}

func TestTemplateSeriesEndDateValidation(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)
	period := createTemplateTestPeriod(t, s.db, "Tpl-Serienende-Pruefung")

	for name, endDate := range map[string]string{
		"before the start": "2026-10-16",
		"after the period": "2027-01-05",
		"malformed":        "23.10.2026",
	} {
		t.Run(name, func(t *testing.T) {
			b := createTemplateBody(s, "Tpl-Serienende-"+name)
			b["calendar_period_id"] = period.ID
			b["start_date"] = "2026-10-19"
			b["end_date"] = endDate
			w := doTemplateJSON(t, router, http.MethodPost, "/templates", b)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestTemplateSplitRejectsSeriesEndDate(t *testing.T) {
	t.Parallel()

	mat := &mockMaterializationService{result: &timetableModule.MaterializationResult{}}
	s := buildTemplateModule(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})
	created := createSourceTemplate(t, router, s, "Tpl-Split-Serienende")

	b := splitBody(s, "Tpl-Split-Serienende-Neu", calendar.TodayDate().AddDays(7))
	b["end_date"] = "2099-01-01"
	w := doTemplateJSON(t, router, http.MethodPost, fmt.Sprintf("/templates/%d/split", created.TemplateID), b)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "end_date is not supported on a split")
}

// The stored schedule rows arrive in weekday order, so a series whose
// weekdays start on different days must still be bounded by its earliest
// recurrence start — otherwise a valid last day between the two starts is
// rejected (#3594).
func TestUpdateSeriesFirstDayUsesEarliestValidFrom(t *testing.T) {
	t.Parallel()

	stored := templateResponse{Schedules: []templateScheduleResponse{
		{Weekday: 1, ValidFrom: "2026-10-19"},
		{Weekday: 3, ValidFrom: "2026-09-01"},
	}}
	assert.Equal(t, calendar.Date("2026-09-01"), updateSeriesFirstDay(nil, stored))

	pulled := calendar.Date("2026-08-17")
	assert.Equal(t, pulled, updateSeriesFirstDay(&pulled, stored),
		"a pulled forward start stays the lower bound")

	withPeriodStart := templateResponse{Schedules: []templateScheduleResponse{
		{Weekday: 1, ValidFrom: "2026-10-19"},
		{Weekday: 3},
	}}
	assert.True(t, updateSeriesFirstDay(nil, withPeriodStart).IsZero(),
		"a row without valid_from starts with the planning period, so no split bounds the last day")
}
