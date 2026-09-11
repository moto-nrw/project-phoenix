// Issue #3165: a Regeltermin with an offering as its child source may still
// staff each weekday differently. The wire contract: weekday_assignments next
// to source_care_offering_ids may carry staff_ids / primary_staff_id, never
// student_ids, because the child list comes from the offering.
package timetable

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sourcedOfferingID is never resolved: the offering callbacks below accept
// every source, so the handler contract is tested without the enrollment
// module (same approach as the convert-to-series source test).
const sourcedOfferingID int64 = 17

func buildSourcedTemplateModule(t *testing.T) *templateSetup {
	t.Helper()
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateModule(t, mat, fixedTemplateClock)
	s.res.TimetableData = testTimetableDataWithOfferingCallbacks(
		s.db,
		nil,
		func(context.Context, []int64, []int64, *int64) error { return nil },
		func(context.Context, scheduleSvc.OfferingRosterResyncInput) error { return nil },
		fixedTemplateClock,
	)
	attachSplitService(s, mat)
	return s
}

// sourcedWeekdayStaffBody is what the editor sends for a sourced Mo/Mi series
// with a different supervisor on each day.
func sourcedWeekdayStaffBody(s *templateSetup, name string) map[string]any {
	body := createTemplateBody(s, name)
	body["target_group_type"] = activitiesModel.TargetGroupTypeAngebot
	body["source_care_offering_ids"] = []int64{sourcedOfferingID}
	body["student_ids"] = []int64{}
	body["staff_ids"] = []int64{}
	delete(body, "primary_staff_id")
	body["weekday_assignments"] = []map[string]any{
		{"weekday": activitiesModel.WeekdayMonday, "staff_ids": []int64{s.staffA}, "student_ids": []int64{}, "primary_staff_id": s.staffA},
		{"weekday": activitiesModel.WeekdayWednesday, "staff_ids": []int64{s.staffB}, "student_ids": []int64{}},
	}
	return body
}

func withWeekdayChild(body map[string]any, studentID int64) map[string]any {
	assignments := body["weekday_assignments"].([]map[string]any)
	assignments[1]["student_ids"] = []int64{studentID}
	return body
}

func TestTemplateOfferingSource_WeekdayStaffRoundTripsThroughCreateUpdateSplit(t *testing.T) {
	t.Parallel()

	s := buildSourcedTemplateModule(t)
	defer s.cleanupFn()
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})
	period := createTemplateTestPeriod(t, s.db, "Tpl-Sourced-Weekday-Staff")

	createW := doTemplateJSON(t, router, http.MethodPost, "/templates", sourcedWeekdayStaffBody(s, "Tpl-Randstunde"))
	require.Equal(t, http.StatusCreated, createW.Code, "body=%s", createW.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createW)

	getW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
	require.Equal(t, http.StatusOK, getW.Code, "body=%s", getW.Body.String())
	got := decodeTemplateData[templateResponse](t, getW)
	assert.Equal(t, []int64{sourcedOfferingID}, got.SourceCareOfferingIDs)
	require.Len(t, got.WeekdayAssignments, 2, "reopening the editor must show the per-weekday staffing")
	assert.Equal(t, activitiesModel.WeekdayMonday, got.WeekdayAssignments[0].Weekday)
	assert.Equal(t, []int64{s.staffA}, got.WeekdayAssignments[0].StaffIDs)
	require.NotNil(t, got.WeekdayAssignments[0].PrimaryStaffID)
	assert.Equal(t, s.staffA, *got.WeekdayAssignments[0].PrimaryStaffID)
	assert.Equal(t, activitiesModel.WeekdayWednesday, got.WeekdayAssignments[1].Weekday)
	assert.Equal(t, []int64{s.staffB}, got.WeekdayAssignments[1].StaffIDs)

	updateW := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", created.TemplateID), sourcedWeekdayStaffBody(s, "Tpl-Randstunde"))
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())

	splitReq := sourcedWeekdayStaffBody(s, "Tpl-Randstunde")
	splitReq["effective_date"] = timezone.NewDate(2026, 8, 31).String()
	splitW := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", created.TemplateID), splitReq)
	require.Equal(t, http.StatusOK, splitW.Code, "body=%s", splitW.Body.String())
}

func TestTemplateOfferingSource_WeekdayChildrenAreRejected(t *testing.T) {
	t.Parallel()

	s := buildSourcedTemplateModule(t)
	defer s.cleanupFn()
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	createW := doTemplateJSON(t, router, http.MethodPost, "/templates", sourcedWeekdayStaffBody(s, "Tpl-Randstunde-Kinder"))
	require.Equal(t, http.StatusCreated, createW.Code, "body=%s", createW.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createW)

	splitReq := withWeekdayChild(sourcedWeekdayStaffBody(s, "Tpl-Randstunde-Kinder"), s.studentA)
	splitReq["effective_date"] = timezone.NewDate(2026, 8, 31).String()

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{"create", http.MethodPost, "/templates", withWeekdayChild(sourcedWeekdayStaffBody(s, "Tpl-Randstunde-Neu"), s.studentA)},
		{"update", http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), withWeekdayChild(sourcedWeekdayStaffBody(s, "Tpl-Randstunde-Kinder"), s.studentA)},
		{"split", http.MethodPost, fmt.Sprintf("/templates/%d/split", created.TemplateID), splitReq},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := doTemplateJSON(t, router, tc.method, tc.path, tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
			assert.Contains(t, w.Body.String(), "weekday_assignments must not contain student_ids")
		})
	}
}
