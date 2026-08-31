// Handler tests for the #2187 split-series editor contract:
//
//   - GET /templates/{id}?period_id=N resolves an id that belongs to an
//     already-capped predecessor segment to the living successor and reports
//     the resolution via resolved_from_template_id; a genuine miss stays a 404
//     carrying the stable code "template_not_found".
//   - PUT /templates/{id} accepts series_roster_from (YYYY-MM-DD) plus the
//     scope of changed people and mirrors exactly that change onto the
//     predecessor segment; a malformed date is a 400.
//   - POST /templates/{id}/split rejects series_roster_from outright.
//
// Hermetic: real repos + real split service against the test DB, fixtures via
// buildTemplateModule, driven through the same router the split tests use.
package timetable

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// splitSeriesSetup builds the shared fixture of these tests: a source template
// split once, so the source id is a fully capped predecessor of the successor.
type splitSeriesSetup struct {
	*templateSetup
	router    chi.Router
	oldID     int64
	newID     int64
	effective timezone.Date
	periodID  int64
}

func buildSplitSeriesSetup(t *testing.T, name string) *splitSeriesSetup {
	t.Helper()
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateModule(t, mat, fixedTemplateClock)
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, name+"-Quelle")
	effective := timezone.NewDate(2026, 8, 24).AddDays(7)
	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", created.TemplateID),
		splitBody(s, name+"-Nachfolger", effective))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	split := decodeTemplateData[splitTemplateResponse](t, w)

	period := createTemplateTestPeriod(t, s.db, name+"-Zeitraum")

	return &splitSeriesSetup{
		templateSetup: s,
		router:        router,
		oldID:         created.TemplateID,
		newID:         split.NewTemplateID,
		effective:     effective,
		periodID:      period.ID,
	}
}

// decodeTemplateError reads the error envelope of a non-2xx response.
func decodeTemplateError(t *testing.T, w *httptest.ResponseRecorder) struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Code   string `json:"code"`
} {
	t.Helper()
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Code   string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), "body=%s", w.Body.String())
	return out
}

func TestGetTemplate_ResolvesCappedPredecessorToLivingSuccessor(t *testing.T) {
	t.Parallel()

	s := buildSplitSeriesSetup(t, "Tpl-SeriesGet")
	defer s.cleanupFn()

	w := doTemplateJSON(t, s.router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", s.oldID, s.periodID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	resp := decodeTemplateData[templateResponse](t, w)
	assert.Equal(t, s.newID, resp.ID, "the editor must be handed the living segment")
	require.NotNil(t, resp.ResolvedFromTemplateID)
	assert.Equal(t, s.oldID, *resp.ResolvedFromTemplateID,
		"the response reports which id was resolved away")
}

func TestGetTemplate_UnknownTemplateReturns404WithCode(t *testing.T) {
	t.Parallel()

	s := buildSplitSeriesSetup(t, "Tpl-SeriesMiss")
	defer s.cleanupFn()

	unknownID := s.newID + 1_000_000
	w := doTemplateJSON(t, s.router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", unknownID, s.periodID), nil)
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, templateNotFoundCode, decodeTemplateError(t, w).Code,
		"a genuine miss must carry the stable error code the editor maps")
}

func TestUpdateTemplate_RejectsInvalidSeriesRosterFrom(t *testing.T) {
	t.Parallel()

	s := buildSplitSeriesSetup(t, "Tpl-SeriesBadDate")
	defer s.cleanupFn()

	body := createTemplateBody(s.templateSetup, "Tpl-SeriesBadDate-Update")
	body["series_roster_from"] = "not-a-date"
	w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d", s.newID), body)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, decodeTemplateError(t, w).Error, "series_roster_from")
}

func TestSplitTemplate_RejectsSeriesRosterFrom(t *testing.T) {
	t.Parallel()

	s := buildSplitSeriesSetup(t, "Tpl-SeriesSplitReject")
	defer s.cleanupFn()

	body := splitBody(s.templateSetup, "Tpl-SeriesSplitReject-Zweiter", s.effective.AddDays(7))
	body["series_roster_from"] = timezone.NewDate(2026, 8, 24).String()
	w := doTemplateJSON(t, s.router, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", s.newID), body)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, decodeTemplateError(t, w).Error, "series_roster_from is not supported on a split")
}

// TestUpdateTemplate_SeriesRosterFromReachesPredecessor is the API-level happy
// path: a child added on the living segment with an anchor inside the capped
// predecessor's window gets a bounded predecessor enrollment.
func TestUpdateTemplate_SeriesRosterFromReachesPredecessor(t *testing.T) {
	t.Parallel()

	s := buildSplitSeriesSetup(t, "Tpl-SeriesRoster")
	defer s.cleanupFn()

	suffix := time.Now().UnixNano()
	latecomer := testpkg.CreateTestStudent(t, s.db, "Tpl", fmt.Sprintf("Nachzuegler-%d", suffix), "3a")

	anchor := timezone.NewDate(2026, 8, 24).AddDays(3)
	body := createTemplateBody(s.templateSetup, "Tpl-SeriesRoster-Update")
	body["student_ids"] = []int64{s.studentA, s.studentB, latecomer.ID}
	body["series_roster_from"] = anchor.String()
	body["series_roster_scope_student_ids"] = []int64{latecomer.ID}

	w := doTemplateJSON(t, s.router, http.MethodPut, fmt.Sprintf("/templates/%d", s.newID), body)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	rows, err := activitiesRepo.NewStudentEnrollmentRepository(s.db).FindByGroupID(s.ctx, s.oldID)
	require.NoError(t, err)
	weekdays := make([]int, 0, 2)
	for _, row := range rows {
		if row.StudentID != latecomer.ID {
			continue
		}
		assert.Equal(t, anchor, row.ValidFrom, "the predecessor row starts at the anchor")
		require.NotNil(t, row.ValidUntil)
		assert.Equal(t, s.effective, *row.ValidUntil, "and ends with the predecessor segment")
		require.NotNil(t, row.Weekday, "predecessor rows are written weekday-explicit")
		weekdays = append(weekdays, *row.Weekday)
	}
	assert.ElementsMatch(t,
		[]int{activitiesModel.WeekdayMonday, activitiesModel.WeekdayWednesday}, weekdays,
		"one bounded predecessor row per scheduled weekday")
}
