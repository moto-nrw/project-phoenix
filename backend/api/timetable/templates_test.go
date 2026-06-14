package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type templateSetup struct {
	res       *Resource
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	category  *activitiesModel.Category
	staffA    int64
	staffB    int64
	studentA  int64
	studentB  int64
	cleanupFn func()
}

type mockMaterializationService struct {
	result *scheduleSvc.MaterializationResult
	err    error
	from   timezone.Date
	to     timezone.Date
	source scheduleSvc.MaterializationSource
}

func (m *mockMaterializationService) MaterializeForTenant(_ context.Context, from, to timezone.Date, source scheduleSvc.MaterializationSource) (*scheduleSvc.MaterializationResult, error) {
	m.from = from
	m.to = to
	m.source = source
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockMaterializationService) ResolveWindow(baseDate timezone.Date, weeksAhead int) (timezone.Date, timezone.Date) {
	return baseDate, baseDate.AddDays(weeksAhead*7 - 1)
}

func buildTemplateSetup(t *testing.T, mat scheduleSvc.MaterializationService) *templateSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Tpl-Room-%d", suffix))
	category := testpkg.CreateTestActivityCategory(t, db, fmt.Sprintf("Tpl-Cat-%d", suffix))
	staffA := testpkg.CreateTestStaff(t, db, "Tpl", fmt.Sprintf("StaffA-%d", suffix))
	staffB := testpkg.CreateTestStaff(t, db, "Tpl", fmt.Sprintf("StaffB-%d", suffix))
	studentA := testpkg.CreateTestStudent(t, db, "Tpl", fmt.Sprintf("StudentA-%d", suffix), "3a")
	studentB := testpkg.CreateTestStudent(t, db, "Tpl", fmt.Sprintf("StudentB-%d", suffix), "3a")

	res := NewResource(Dependencies{
		TimetableData:          testTimetableData(db),
		CalendarPeriodService:  scheduleSvc.NewCalendarPeriodService(scheduleRepo.NewCalendarPeriodRepository(db), nil),
		MaterializationService: mat,
		DB:                     db,
	})

	cleanup := func() {
		cleanupTemplatesByPrefix(t, db, "Tpl-")
		testpkg.CleanupTableRecords(t, db, "users.students", studentA.ID, studentB.ID)
		testpkg.CleanupTableRecords(t, db, "users.staff", staffA.ID, staffB.ID)
		testpkg.CleanupTableRecords(t, db, "users.persons", studentA.PersonID, studentB.PersonID, staffA.PersonID, staffB.PersonID)
		testpkg.CleanupTableRecords(t, db, "activities.categories", category.ID)
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	}

	return &templateSetup{
		res:       res,
		db:        db,
		ctx:       ctx,
		roomID:    room.ID,
		category:  category,
		staffA:    staffA.ID,
		staffB:    staffB.ID,
		studentA:  studentA.ID,
		studentB:  studentB.ID,
		cleanupFn: cleanup,
	}
}

func cleanupTemplatesByPrefix(t *testing.T, db *bun.DB, prefix string) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	_, err := db.NewDelete().
		Table("activities.student_enrollments").
		Where("activity_group_id IN (SELECT id FROM activities.groups WHERE name LIKE ?)", prefix+"%").
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewDelete().
		Table("activities.supervisors").
		Where("group_id IN (SELECT id FROM activities.groups WHERE name LIKE ?)", prefix+"%").
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewDelete().
		Table("activities.schedules").
		Where("activity_group_id IN (SELECT id FROM activities.groups WHERE name LIKE ?)", prefix+"%").
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewDelete().
		Table("activities.groups").
		Where("name LIKE ?", prefix+"%").
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewDelete().
		Table("schedule.timeframes").
		Where("description LIKE ?", prefix+"%").
		Exec(ctx)
	require.NoError(t, err)
}

func templateRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/templates", res.listTemplates)
	r.Get("/templates/{id}", res.getTemplate)
	r.Post("/templates", res.createTemplate)
	r.Put("/templates/{id}", res.updateTemplate)
	r.Delete("/templates/{id}", res.archiveTemplate)
	return r
}

func doTemplateJSON(t *testing.T, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeTemplateData[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	var out T
	require.NoError(t, json.Unmarshal(env.Data, &out))
	return out
}

func createTemplateBody(s *templateSetup, name string) map[string]any {
	return map[string]any{
		"name":             name,
		"type":             activitiesModel.GroupTypeCare,
		"weekdays":         []int{activitiesModel.WeekdayMonday, activitiesModel.WeekdayWednesday},
		"start_time":       "12:00",
		"end_time":         "12:50",
		"room_id":          s.roomID,
		"category_id":      s.category.ID,
		"max_participants": 25,
		"week_pattern":     1,
		"student_ids":      []int64{s.studentA, s.studentA, 0, s.studentB},
		"staff_ids":        []int64{s.staffA, s.staffB, s.staffA, -50},
		"primary_staff_id": s.staffB,
	}
}

func TestTemplateCreateListGetUpdateArchive(t *testing.T) {
	mat := &mockMaterializationService{
		result: &scheduleSvc.MaterializationResult{InstancesCreated: 3},
	}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)
	educationGroup := testpkg.CreateTestEducationGroup(t, s.db, "Tpl-EducationGroup")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "education.groups", educationGroup.ID)
	})

	body := createTemplateBody(s, "Tpl-CreateListUpdate")
	body["materialize_from"] = "2026-05-04"
	body["materialize_to"] = "2026-05-08"
	body["education_group_id"] = educationGroup.ID

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)
	require.NotZero(t, created.TemplateID)
	assert.Len(t, created.ScheduleIDs, 2)
	assert.Equal(t, 3, created.InstancesCreated)
	assert.Equal(t, scheduleSvc.MaterializationSourceManual, mat.source)

	listW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
	require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
	list := decodeTemplateData[listTemplatesResponse](t, listW)
	var tpl templateResponse
	for _, candidate := range list.Templates {
		if candidate.ID == created.TemplateID {
			tpl = candidate
			break
		}
	}
	require.Equal(t, created.TemplateID, tpl.ID, "created template missing from list")
	assert.Equal(t, "Tpl-CreateListUpdate", tpl.Name)
	assert.Equal(t, activitiesModel.GroupTypeCare, tpl.Type)
	assert.Equal(t, s.roomID, *tpl.RoomID)
	assert.Equal(t, s.category.ID, tpl.CategoryID)
	require.NotNil(t, tpl.EducationGroupID)
	assert.Equal(t, educationGroup.ID, *tpl.EducationGroupID)
	assert.Equal(t, educationGroup.Name, tpl.EducationGroupName)
	assert.Equal(t, []int64{s.studentA, s.studentB}, tpl.StudentIDs)
	assert.Equal(t, []int64{s.staffB, s.staffA}, tpl.StaffIDs)
	require.NotNil(t, tpl.PrimaryStaffID)
	assert.Equal(t, s.staffB, *tpl.PrimaryStaffID)
	assert.Len(t, tpl.Schedules, 2)
	assert.Equal(t, "12:00", tpl.Schedules[0].StartTime)
	assert.Equal(t, "12:50", tpl.Schedules[0].EndTime)
	assert.Equal(t, 1, tpl.Schedules[0].WeekPattern)

	getW := doTemplateJSON(t, router, http.MethodGet, fmt.Sprintf("/templates/%d", created.TemplateID), nil)
	require.Equal(t, http.StatusOK, getW.Code, "body=%s", getW.Body.String())
	got := decodeTemplateData[templateResponse](t, getW)
	assert.Equal(t, created.TemplateID, got.ID)
	require.NotNil(t, got.EducationGroupID)
	assert.Equal(t, educationGroup.ID, *got.EducationGroupID)
	assert.Equal(t, educationGroup.Name, got.EducationGroupName)

	updateBody := createTemplateBody(s, "Tpl-Updated")
	updateBody["type"] = activitiesModel.GroupTypeActivity
	updateBody["education_group_id"] = educationGroup.ID
	updateBody["weekdays"] = []int{activitiesModel.WeekdayFriday}
	updateBody["start_time"] = "13:15"
	updateBody["end_time"] = "14:00"
	updateBody["student_ids"] = []int64{s.studentB}
	updateBody["staff_ids"] = []int64{s.staffA}
	updateBody["primary_staff_id"] = s.staffA
	updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())
	updated := decodeTemplateData[templateResponse](t, updateW)
	assert.Equal(t, "Tpl-Updated", updated.Name)
	assert.Equal(t, activitiesModel.GroupTypeActivity, updated.Type)
	assert.Equal(t, []int64{s.studentB}, updated.StudentIDs)
	assert.Equal(t, []int64{s.staffA}, updated.StaffIDs)
	require.Len(t, updated.Schedules, 1)
	assert.Equal(t, activitiesModel.WeekdayFriday, updated.Schedules[0].Weekday)
	assert.Equal(t, "13:15", updated.Schedules[0].StartTime)

	delW := doTemplateJSON(t, router, http.MethodDelete, fmt.Sprintf("/templates/%d", created.TemplateID), nil)
	require.Equal(t, http.StatusOK, delW.Code, "body=%s", delW.Body.String())
	secondDelW := doTemplateJSON(t, router, http.MethodDelete, fmt.Sprintf("/templates/%d", created.TemplateID), nil)
	assert.Equal(t, http.StatusNotFound, secondDelW.Code)
}

func TestTemplateCreateValidationAndMaterializationFailure(t *testing.T) {
	mat := &mockMaterializationService{err: errors.New("materializer unavailable")}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	valid := createTemplateBody(s, "Tpl-MaterializeFailure")
	valid["materialize_from"] = "2026-05-04"
	valid["materialize_to"] = "2026-05-08"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", valid)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)
	assert.NotZero(t, created.TemplateID)
	assert.Zero(t, created.InstancesCreated, "template save must survive materialization warning path")

	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "invalid type", mutate: func(b map[string]any) { b["type"] = "party" }},
		{name: "invalid weekday", mutate: func(b map[string]any) { b["weekdays"] = []int{8} }},
		{name: "invalid start time", mutate: func(b map[string]any) { b["start_time"] = "bad" }},
		{name: "end before start", mutate: func(b map[string]any) { b["end_time"] = "11:00" }},
		{name: "invalid week pattern", mutate: func(b map[string]any) { b["week_pattern"] = 9 }},
		{name: "missing category", mutate: func(b map[string]any) { b["category_id"] = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := createTemplateBody(s, "Tpl-Invalid-"+tc.name)
			tc.mutate(body)
			w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestTemplateCreateReusesExistingTimeframe(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	first := createTemplateBody(s, "Tpl-Reuse-A")
	first["weekdays"] = []int{activitiesModel.WeekdayMonday}
	w1 := doTemplateJSON(t, router, http.MethodPost, "/templates", first)
	require.Equal(t, http.StatusCreated, w1.Code, "body=%s", w1.Body.String())
	createdA := decodeTemplateData[createTemplateResponse](t, w1)

	second := createTemplateBody(s, "Tpl-Reuse-B")
	second["weekdays"] = []int{activitiesModel.WeekdayTuesday}
	w2 := doTemplateJSON(t, router, http.MethodPost, "/templates", second)
	require.Equal(t, http.StatusCreated, w2.Code, "body=%s", w2.Body.String())
	createdB := decodeTemplateData[createTemplateResponse](t, w2)

	assert.Equal(t, createdA.TimeframeID, createdB.TimeframeID,
		"same start/end clock window should reuse the existing timeframe")
}

func TestTemplateUpdateValidationAndNotFound(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	valid := createTemplateBody(s, "Tpl-UpdateValidation")
	cases := []struct {
		name   string
		path   string
		mutate func(map[string]any)
		want   int
	}{
		{name: "missing name", path: "/templates/500", mutate: func(b map[string]any) { b["name"] = "" }, want: http.StatusBadRequest},
		{name: "invalid type", path: "/templates/500", mutate: func(b map[string]any) { b["type"] = "broken" }, want: http.StatusBadRequest},
		{name: "missing room", path: "/templates/500", mutate: func(b map[string]any) { b["room_id"] = 0 }, want: http.StatusBadRequest},
		{name: "invalid start", path: "/templates/500", mutate: func(b map[string]any) { b["start_time"] = "nope" }, want: http.StatusBadRequest},
		{name: "invalid end", path: "/templates/500", mutate: func(b map[string]any) { b["end_time"] = "nope" }, want: http.StatusBadRequest},
		{name: "end before start", path: "/templates/500", mutate: func(b map[string]any) { b["end_time"] = "11:00" }, want: http.StatusBadRequest},
		{name: "invalid week pattern", path: "/templates/500", mutate: func(b map[string]any) { b["week_pattern"] = -1 }, want: http.StatusBadRequest},
		{name: "not found", path: "/templates/500", mutate: func(_ map[string]any) {}, want: http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := make(map[string]any, len(valid))
			for key, value := range valid {
				body[key] = value
			}
			tc.mutate(body)
			w := doTemplateJSON(t, router, http.MethodPut, tc.path, body)
			assert.Equal(t, tc.want, w.Code, "body=%s", w.Body.String())
		})
	}

	getMissing := doTemplateJSON(t, router, http.MethodGet, "/templates/500", nil)
	assert.Equal(t, http.StatusNotFound, getMissing.Code)

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Put("/templates/{id}", s.res.updateTemplate)
	noTenant := doTemplateJSON(t, r, http.MethodPut, "/templates/500", valid)
	assert.Equal(t, http.StatusInternalServerError, noTenant.Code)

	unwired := templateRouter(s.ctx, NewResource(Dependencies{DB: s.db}))
	unwiredW := doTemplateJSON(t, unwired, http.MethodPut, "/templates/500", valid)
	assert.Equal(t, http.StatusInternalServerError, unwiredW.Code)
}

func TestTemplateRoutesRejectBadIDsAndMissingContext(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	assert.Equal(t, http.StatusBadRequest, doTemplateJSON(t, router, http.MethodGet, "/templates/not-number", nil).Code)
	assert.Equal(t, http.StatusBadRequest, doTemplateJSON(t, router, http.MethodPut, "/templates/0", createTemplateBody(s, "Tpl-BadID")).Code)
	assert.Equal(t, http.StatusBadRequest, doTemplateJSON(t, router, http.MethodDelete, "/templates/bad", nil).Code)
	assert.Equal(t, http.StatusBadRequest, doTemplateJSON(t, router, http.MethodGet, "/templates?period_id=nope", nil).Code)

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/templates", s.res.listTemplates)
	noTenantW := doTemplateJSON(t, r, http.MethodGet, "/templates", nil)
	assert.Equal(t, http.StatusInternalServerError, noTenantW.Code)

	unwired := NewResource(Dependencies{})
	unwiredRouter := templateRouter(s.ctx, unwired)
	assert.Equal(t, http.StatusInternalServerError, doTemplateJSON(t, unwiredRouter, http.MethodPost, "/templates", createTemplateBody(s, "Tpl-Unwired")).Code)
	assert.Equal(t, http.StatusInternalServerError, doTemplateJSON(t, unwiredRouter, http.MethodGet, "/templates", nil).Code)
}

func TestListTemplatesFiltersByPeriod(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	periodA := createTemplateTestPeriod(t, s.db, "TplPeriodA")
	periodB := createTemplateTestPeriod(t, s.db, "TplPeriodB")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", periodA.ID, periodB.ID)
	})

	bodyA := createTemplateBody(s, "Tpl-Period-A")
	bodyA["calendar_period_id"] = periodA.ID
	require.Equal(t, http.StatusCreated, doTemplateJSON(t, router, http.MethodPost, "/templates", bodyA).Code)

	bodyB := createTemplateBody(s, "Tpl-Period-B")
	bodyB["calendar_period_id"] = periodB.ID
	require.Equal(t, http.StatusCreated, doTemplateJSON(t, router, http.MethodPost, "/templates", bodyB).Code)

	bodyGlobal := createTemplateBody(s, "Tpl-Period-Global")
	require.Equal(t, http.StatusCreated, doTemplateJSON(t, router, http.MethodPost, "/templates", bodyGlobal).Code)

	w := doTemplateJSON(t, router, http.MethodGet, fmt.Sprintf("/templates?period_id=%d", periodA.ID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	list := decodeTemplateData[listTemplatesResponse](t, w)
	require.NotEmpty(t, list.Templates)
	var sawPeriodA, sawGlobal bool
	for _, tpl := range list.Templates {
		switch tpl.Name {
		case "Tpl-Period-A":
			sawPeriodA = true
			for _, sched := range tpl.Schedules {
				require.NotNil(t, sched.CalendarPeriodID)
				assert.Equal(t, periodA.ID, *sched.CalendarPeriodID)
			}
		case "Tpl-Period-B":
			assert.Fail(t, "period B template must not appear when filtering for period A")
		case "Tpl-Period-Global":
			sawGlobal = true
			for _, sched := range tpl.Schedules {
				assert.Nil(t, sched.CalendarPeriodID)
			}
		}
	}
	assert.True(t, sawPeriodA, "period-scoped template missing from period-filtered list")
	assert.True(t, sawGlobal, "unscoped template missing from period-filtered list")
}

func TestUpdateTemplatePeopleScopesReplacementToSelectedPeriod(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	suffix := time.Now().UnixNano()
	studentC := testpkg.CreateTestStudent(t, s.db, "Tpl", fmt.Sprintf("StudentC-%d", suffix), "3a")
	studentD := testpkg.CreateTestStudent(t, s.db, "Tpl", fmt.Sprintf("StudentD-%d", suffix), "3a")
	staffC := testpkg.CreateTestStaff(t, s.db, "Tpl", fmt.Sprintf("StaffC-%d", suffix))
	staffD := testpkg.CreateTestStaff(t, s.db, "Tpl", fmt.Sprintf("StaffD-%d", suffix))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "users.students", studentC.ID, studentD.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.staff", staffC.ID, staffD.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.persons", studentC.PersonID, studentD.PersonID, staffC.PersonID, staffD.PersonID)
	})

	periodA := createTemplateTestPeriod(t, s.db, "TplPeoplePeriodA")
	periodB := createTemplateTestPeriod(t, s.db, "TplPeoplePeriodB")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", periodA.ID, periodB.ID)
	})

	body := createTemplateBody(s, "Tpl-People-Period-A")
	body["calendar_period_id"] = periodA.ID
	body["student_ids"] = []int64{s.studentA}
	body["staff_ids"] = []int64{s.staffA}
	body["primary_staff_id"] = s.staffA
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	periodBEnrollment := &activitiesModel.StudentEnrollment{
		StudentID:        s.studentB,
		ActivityGroupID:  created.TemplateID,
		ValidFrom:        periodB.StartDate,
		CalendarPeriodID: &periodB.ID,
	}
	periodBEnrollment.SetTenantID(1)
	require.NoError(t, activitiesRepo.NewStudentEnrollmentRepository(s.db).Create(s.ctx, periodBEnrollment))

	globalEnrollment := &activitiesModel.StudentEnrollment{
		StudentID:       studentC.ID,
		ActivityGroupID: created.TemplateID,
		ValidFrom:       timezone.NewDate(2026, time.January, 1),
	}
	globalEnrollment.SetTenantID(1)
	require.NoError(t, activitiesRepo.NewStudentEnrollmentRepository(s.db).Create(s.ctx, globalEnrollment))

	periodBSupervisor := &activitiesModel.SupervisorPlanned{
		StaffID:          s.staffB,
		GroupID:          created.TemplateID,
		ValidFrom:        periodB.StartDate,
		CalendarPeriodID: &periodB.ID,
	}
	periodBSupervisor.SetTenantID(1)
	require.NoError(t, activitiesRepo.NewSupervisorPlannedRepository(s.db).Create(s.ctx, periodBSupervisor))

	globalSupervisor := &activitiesModel.SupervisorPlanned{
		StaffID:   staffC.ID,
		GroupID:   created.TemplateID,
		ValidFrom: timezone.NewDate(2026, time.January, 1),
	}
	globalSupervisor.SetTenantID(1)
	require.NoError(t, activitiesRepo.NewSupervisorPlannedRepository(s.db).Create(s.ctx, globalSupervisor))

	updateBody := createTemplateBody(s, "Tpl-People-Period-A")
	updateBody["calendar_period_id"] = periodA.ID
	updateBody["student_ids"] = []int64{studentD.ID}
	updateBody["staff_ids"] = []int64{staffD.ID}
	updateBody["primary_staff_id"] = staffD.ID
	updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())

	var periodBOtherStudents, globalStudents, periodBOtherStaff, globalStaff int
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.student_enrollments").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", 1).
		Where("activity_group_id = ?", created.TemplateID).
		Where("calendar_period_id = ?", periodB.ID).
		Where("valid_until IS NULL").
		Scan(s.ctx, &periodBOtherStudents))
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.student_enrollments").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", 1).
		Where("activity_group_id = ?", created.TemplateID).
		Where("calendar_period_id IS NULL").
		Where("valid_until IS NULL").
		Scan(s.ctx, &globalStudents))
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.supervisors").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", 1).
		Where("group_id = ?", created.TemplateID).
		Where("calendar_period_id = ?", periodB.ID).
		Where("valid_until IS NULL").
		Scan(s.ctx, &periodBOtherStaff))
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.supervisors").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", 1).
		Where("group_id = ?", created.TemplateID).
		Where("calendar_period_id IS NULL").
		Where("valid_until IS NULL").
		Scan(s.ctx, &globalStaff))
	assert.Equal(t, 1, periodBOtherStudents)
	assert.Equal(t, 1, globalStudents)
	assert.Equal(t, 1, periodBOtherStaff)
	assert.Equal(t, 1, globalStaff)

	var oldStudentUntil, newStudentFrom time.Time
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.student_enrollments").
		Column("valid_until").
		Where("tenant_id = ?", 1).
		Where("activity_group_id = ?", created.TemplateID).
		Where("student_id = ?", s.studentA).
		Where("calendar_period_id = ?", periodA.ID).
		Scan(s.ctx, &oldStudentUntil))
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.student_enrollments").
		Column("valid_from").
		Where("tenant_id = ?", 1).
		Where("activity_group_id = ?", created.TemplateID).
		Where("student_id = ?", studentD.ID).
		Where("calendar_period_id = ?", periodA.ID).
		Where("valid_until IS NULL").
		Scan(s.ctx, &newStudentFrom))
	assert.Equal(t, periodA.StartDate.Format(dateLayout), oldStudentUntil.Format(dateLayout))
	assert.Equal(t, periodA.StartDate.Format(dateLayout), newStudentFrom.Format(dateLayout))
}

func createTemplateTestPeriod(t *testing.T, db *bun.DB, name string) *scheduleModel.CalendarPeriod {
	t.Helper()
	period := &scheduleModel.CalendarPeriod{
		Name:            fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		PeriodType:      scheduleModel.PeriodTypeCustom,
		StartDate:       timezone.NewDate(2026, 1, 1),
		EndDate:         timezone.NewDate(2026, 12, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(1)
	_, err := db.NewInsert().
		Model(period).
		ModelTableExpr("schedule.calendar_periods").
		Exec(testpkg.TenantContext(1))
	require.NoError(t, err)
	return period
}

func TestTemplatePeopleHelpersDeduplicateAndNoopWithoutTenant(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()

	require.Equal(t, []int64{50, 60}, uniquePositiveIDs([]int64{50, 0, 60, 50, -1}))
	validFrom := timezone.NewDate(2026, time.January, 1)
	assert.NoError(t, s.res.replaceTemplateStudents(context.Background(), 12345, []int64{s.studentA}, nil, validFrom))
	assert.NoError(t, s.res.replaceTemplateStaff(context.Background(), 12345, []int64{s.staffA}, &s.staffA, nil, validFrom))
}
