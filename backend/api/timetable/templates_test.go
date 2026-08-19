package timetable

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
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
	// detectFn drives DetectEditedInWindow; nil returns (nil, nil).
	detectFn func(activityGroupID int64, from, to timezone.Date, includeDeletions bool) ([]scheduleSvc.EditedOccurrence, error)
}

func TestValidateLegacyTemplateWorkdays(t *testing.T) {
	existing := []templateScheduleResponse{
		{Weekday: activitiesModel.WeekdayFriday},
		{Weekday: activitiesModel.WeekdaySaturday},
	}

	assert.NoError(t, validateLegacyTemplateWorkdays(existing, []int{
		activitiesModel.WeekdayFriday,
		activitiesModel.WeekdaySaturday,
	}))
	assert.NoError(t, validateLegacyTemplateWorkdays(existing, []int{
		activitiesModel.WeekdayFriday,
	}))
	assert.Error(t, validateLegacyTemplateWorkdays(existing, []int{
		activitiesModel.WeekdayFriday,
		activitiesModel.WeekdaySunday,
	}))
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

func (m *mockMaterializationService) DetectEditedInWindow(_ context.Context, activityGroupID int64, from, to timezone.Date, includeDeletions bool) ([]scheduleSvc.EditedOccurrence, error) {
	if m.detectFn != nil {
		return m.detectFn(activityGroupID, from, to, includeDeletions)
	}
	return nil, nil
}

func buildTemplateSetup(t *testing.T, mat scheduleSvc.MaterializationService) *templateSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Tpl-Room-%d", suffix))
	category := testpkg.CreateTestActivityCategory(t, db, fmt.Sprintf("Tpl-Cat-%d", suffix))
	staffA := testpkg.CreateTestStaff(t, db, "Tpl", fmt.Sprintf("StaffA-%d", suffix))
	staffB := testpkg.CreateTestStaff(t, db, "Tpl", fmt.Sprintf("StaffB-%d", suffix))
	studentA := testpkg.CreateTestStudent(t, db, "Tpl", fmt.Sprintf("StudentA-%d", suffix), "3a")
	studentB := testpkg.CreateTestStudent(t, db, "Tpl", fmt.Sprintf("StudentB-%d", suffix), "3a")
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)

	res := NewResource(Dependencies{
		TimetableData:          testTimetableData(db),
		CalendarPeriodService:  scheduleSvc.NewCalendarPeriodService(scheduleRepo.NewCalendarPeriodRepository(db), nil),
		MaterializationService: mat,
		InstanceService:        serviceFactory.Instance,
		SettingsService:        templateGradeSettings(schoolclass.DefaultGradeLevelMax, nil),
		DB:                     db,
	})
	res.InstanceSeriesConverter = scheduleSvc.NewInstanceSeriesConversionService(
		scheduleSvc.InstanceSeriesConversionDependencies{
			DB:              db,
			InstanceRepo:    repoFactory.ActivityInstance,
			InstanceService: res.InstanceService,
			TimetableData:   res.TimetableData,
		},
	)

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

func templateGradeSettings(value int, resolveErr error) *configtest.Mock {
	return &configtest.Mock{
		ResolveIntFn: func(_ context.Context, key string) (int, error) {
			if key != configModel.KeyEnrollmentGradeLevelMax {
				return 0, fmt.Errorf("unexpected integer setting %q", key)
			}
			return value, resolveErr
		},
	}
}

func TestResolveTemplateGradeLevelMax_FailsClosed(t *testing.T) {
	t.Run("missing settings service", func(t *testing.T) {
		_, err := (&Resource{}).resolveTemplateGradeLevelMax(context.Background())
		assert.ErrorContains(t, err, "settings service is not configured")
	})

	t.Run("resolve failure", func(t *testing.T) {
		resource := &Resource{Dependencies: Dependencies{
			SettingsService: templateGradeSettings(0, errors.New("settings unavailable")),
		}}
		_, err := resource.resolveTemplateGradeLevelMax(context.Background())
		assert.ErrorContains(t, err, "settings unavailable")
	})

	t.Run("out-of-range stored value", func(t *testing.T) {
		resource := &Resource{Dependencies: Dependencies{
			SettingsService: templateGradeSettings(14, nil),
		}}
		_, err := resource.resolveTemplateGradeLevelMax(context.Background())
		assert.ErrorContains(t, err, "outside 1..13")
	})
}

func cleanupTemplatesByPrefix(t *testing.T, db *bun.DB, prefix string) {
	t.Helper()
	ctx := testpkg.Ctx(t)
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

func TestTemplateCreateRejectsArchivedCategory(t *testing.T) {
	s := buildTemplateSetup(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	_, err := s.db.NewUpdate().
		Table("activities.categories").
		Set("archived_at = ?", time.Now()).
		Where("id = ?", s.category.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", createTemplateBody(s, "Tpl-ArchivedCategory"))
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "category is archived or unavailable")
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
	assert.Contains(t, listW.Body.String(), `"weekday_assignments":null`,
		"a period-free catalog read must not claim an editable shared roster")
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
	assert.Nil(t, tpl.WeekdayAssignments,
		"a period-free catalog read must mark weekday rosters as not loaded")

	missingPeriodW := doTemplateJSON(t, router, http.MethodGet, fmt.Sprintf("/templates/%d", created.TemplateID), nil)
	require.Equal(t, http.StatusBadRequest, missingPeriodW.Code, "body=%s", missingPeriodW.Body.String())

	period := createTemplateTestPeriod(t, s.db, "Tpl-Editable-Read")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
	getW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
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

// #1565 review: changing a series' Listenart must reach the occurrences that
// were already materialized for future dates — otherwise the classified daily
// list omits the series until someone re-plans the week. Per-occurrence
// classification overrides and past/today rows must survive the propagation.
func TestTemplateUpdatePropagatesListKindToFutureInstances(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	// Create the series already classified as "mensa".
	body := createTemplateBody(s, fmt.Sprintf("Tpl-ListKind-%d", time.Now().UnixNano()))
	body["list_kind"] = activitiesModel.ListKindMensa
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)
	require.NotZero(t, created.TemplateID)

	instanceRepo := scheduleRepo.NewActivityInstanceRepository(s.db)
	today := timezone.TodayDate()
	mkInstance := func(name string, date timezone.Date, hour int, listKind *string) *scheduleModel.ActivityInstance {
		tmplID := created.TemplateID
		inst := &scheduleModel.ActivityInstance{
			Date:            date,
			ActivityGroupID: &tmplID,
			Title:           name,
			StartTime:       time.Date(2024, 1, 1, hour, 0, 0, 0, time.UTC),
			EndTime:         time.Date(2024, 1, 1, hour+1, 0, 0, 0, time.UTC),
			RoomID:          s.roomID,
			Status:          scheduleModel.InstanceStatusPlanned,
			ListKind:        listKind,
		}
		inst.SetTenantID(tenant.FromContext(s.ctx))
		require.NoError(t, instanceRepo.Create(s.ctx, inst))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })
		return inst
	}

	// futureRow still carries the series value → should adopt the new kind.
	futureRow := mkInstance("Future", today.AddDays(7), 8, testpkg.StrPtr(activitiesModel.ListKindMensa))
	// overriddenRow was individually re-classified → must be preserved.
	overriddenRow := mkInstance("Overridden", today.AddDays(7), 9, testpkg.StrPtr(activitiesModel.ListKindActivity))
	// pastRow is elapsed → must be preserved.
	pastRow := mkInstance("Past", today.AddDays(-7), 8, testpkg.StrPtr(activitiesModel.ListKindMensa))

	// Re-classify the series to "learning_time" via the template PUT.
	updateBody := createTemplateBody(s, "Tpl-ListKind-Updated")
	updateBody["list_kind"] = activitiesModel.ListKindLearningTime
	updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())

	gotFuture, err := instanceRepo.FindByID(s.ctx, futureRow.ID)
	require.NoError(t, err)
	require.NotNil(t, gotFuture.ListKind)
	assert.Equal(t, activitiesModel.ListKindLearningTime, *gotFuture.ListKind,
		"future occurrence must adopt the series' new Listenart")

	gotOverridden, err := instanceRepo.FindByID(s.ctx, overriddenRow.ID)
	require.NoError(t, err)
	require.NotNil(t, gotOverridden.ListKind)
	assert.Equal(t, activitiesModel.ListKindActivity, *gotOverridden.ListKind,
		"per-occurrence override must survive the series edit")

	gotPast, err := instanceRepo.FindByID(s.ctx, pastRow.ID)
	require.NoError(t, err)
	require.NotNil(t, gotPast.ListKind)
	assert.Equal(t, activitiesModel.ListKindMensa, *gotPast.ListKind,
		"past occurrence must be left untouched")
}

func TestListTemplates_CapacityFields(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()

	// Tenant-override ratio of 1 child per staff member, exercised via the
	// same HasTenantOverride -> ResolveInt chain the real settings service
	// uses (settings-system.md), not just the registry default.
	s.res.SettingsService = &configtest.Mock{
		HasTenantOverrideFn: func(context.Context, string) (bool, error) { return true, nil },
		ResolveIntFn:        func(context.Context, string) (int, error) { return 1, nil },
	}
	today := timezone.TodayDate()
	period := createTemplateTestPeriodRange(
		t,
		s.db,
		"TplCapacityPeriod",
		today.AddDays(-7),
		today.AddDays(14),
		1,
		nil,
	)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })

	router := templateRouter(s.ctx, s.res)
	body := createTemplateBody(s, "Tpl-Capacity")
	// Only one staff member assigned against two enrolled students, so at a
	// ratio of 1 the required count (2) exceeds the assigned count (1).
	body["staff_ids"] = []int64{s.staffA}
	body["primary_staff_id"] = s.staffA

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

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
	assert.Equal(t, 2, tpl.EnrollmentCount)
	assert.Equal(t, 1, tpl.SupervisorCount)
	assert.Equal(t, 2, tpl.RequiredStaffCount, "ceil(2 children / ratio 1) = 2")
	assert.Equal(t, 1, tpl.AssignedStaffCount)
	assert.Equal(t, activitiesModel.TargetGroupTypeNone, tpl.TargetGroupType, "default target group type for templates predating Zielgruppe")
	assert.Nil(t, tpl.TargetGradeLevel)
	assert.Nil(t, tpl.TargetSchoolClass)
	assert.Nil(t, tpl.CalendarPeriodID, "no calendar period set on this template")
}

func TestTemplateCreateUpdate_ZielgruppeRoundTrip(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	body := createTemplateBody(s, "Tpl-Zielgruppe")
	body["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
	body["target_grade_level"] = 3

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	period := createTemplateTestPeriod(t, s.db, "Tpl-Zielgruppe-Read")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
	getW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
	require.Equal(t, http.StatusOK, getW.Code, "body=%s", getW.Body.String())
	got := decodeTemplateData[templateResponse](t, getW)
	assert.Equal(t, activitiesModel.TargetGroupTypeJahrgang, got.TargetGroupType)
	require.NotNil(t, got.TargetGradeLevel)
	assert.EqualValues(t, 3, *got.TargetGradeLevel)
	assert.Nil(t, got.TargetSchoolClass)

	// Switch to Klasse on update; grade level must clear (mutually exclusive).
	updateBody := createTemplateBody(s, "Tpl-Zielgruppe-Updated")
	updateBody["target_group_type"] = activitiesModel.TargetGroupTypeKlasse
	updateBody["target_school_class"] = "3a"
	updateBody["targets"] = []map[string]any{
		{"type": activitiesModel.TargetGroupTypeKlasse, "school_class": "3a"},
	}
	updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())
	updated := decodeTemplateData[templateResponse](t, updateW)
	assert.Equal(t, activitiesModel.TargetGroupTypeKlasse, updated.TargetGroupType)
	assert.Nil(t, updated.TargetGradeLevel)
	require.NotNil(t, updated.TargetSchoolClass)
	assert.Equal(t, "3a", *updated.TargetSchoolClass)
}

func TestTemplateCreate_MultipleTargetsRoundTrip(t *testing.T) {
	s := buildTemplateSetup(t, &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)
	period := createTemplateTestPeriod(t, s.db, "Tpl-Multiple-Targets-Read")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })

	body := createTemplateBody(s, fmt.Sprintf("Tpl-Multiple-Targets-%d", time.Now().UnixNano()))
	body["target_group_type"] = activitiesModel.TargetGroupTypeKlasse
	body["target_school_class"] = "1a"
	body["targets"] = []map[string]any{
		{"type": activitiesModel.TargetGroupTypeKlasse, "school_class": "1a"},
		{"type": activitiesModel.TargetGroupTypeKlasse, "school_class": "2a"},
	}

	createdResponse := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, createdResponse.Code, "body=%s", createdResponse.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createdResponse)

	getResponse := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
	require.Equal(t, http.StatusOK, getResponse.Code, "body=%s", getResponse.Body.String())
	template := decodeTemplateData[templateResponse](t, getResponse)
	require.Len(t, template.Targets, 2)
	assert.Equal(t, "1a", *template.Targets[0].SchoolClass)
	assert.Equal(t, "2a", *template.Targets[1].SchoolClass)

	updateBody := createTemplateBody(s, fmt.Sprintf("Tpl-Multiple-Targets-Updated-%d", time.Now().UnixNano()))
	updateBody["target_group_type"] = activitiesModel.TargetGroupTypeKlasse
	updateBody["target_school_class"] = "2a"
	updateBody["targets"] = []map[string]any{
		{"type": activitiesModel.TargetGroupTypeKlasse, "school_class": "2a"},
		{"type": activitiesModel.TargetGroupTypeKlasse, "school_class": "3a"},
	}
	updatedResponse := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updatedResponse.Code, "body=%s", updatedResponse.Body.String())
	updated := decodeTemplateData[templateResponse](t, updatedResponse)
	require.Len(t, updated.Targets, 2)
	assert.Equal(t, "2a", *updated.Targets[0].SchoolClass)
	assert.Equal(t, "3a", *updated.Targets[1].SchoolClass)

	legacyUpdateBody := createTemplateBody(s, fmt.Sprintf("Tpl-Multiple-Targets-Legacy-%d", time.Now().UnixNano()))
	legacyUpdateBody["target_group_type"] = activitiesModel.TargetGroupTypeKlasse
	legacyUpdateBody["target_school_class"] = "2a"
	legacyResponse := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", created.TemplateID), legacyUpdateBody)
	require.Equal(t, http.StatusOK, legacyResponse.Code, "body=%s", legacyResponse.Body.String())
	legacyUpdated := decodeTemplateData[templateResponse](t, legacyResponse)
	require.Len(t, legacyUpdated.Targets, 2)
	assert.Equal(t, "2a", *legacyUpdated.Targets[0].SchoolClass)
	assert.Equal(t, "3a", *legacyUpdated.Targets[1].SchoolClass)
}

func TestTemplateCreate_MultipleTargetsRejectsCrossTenantEducationGroup(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	otherTenant := testpkg.NewTenantScope(t, s.db)
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, s.db, otherTenant.TenantID) })
	otherGroup := testpkg.CreateTestEducationGroupForTenant(t, s.db, otherTenant.TenantID, "Tpl-Cross-Tenant")

	body := createTemplateBody(s, fmt.Sprintf("Tpl-Cross-Tenant-Target-%d", time.Now().UnixNano()))
	body["education_group_id"] = otherGroup.ID
	body["target_group_type"] = activitiesModel.TargetGroupTypeKlasse
	body["targets"] = []map[string]any{
		{"type": activitiesModel.TargetGroupTypeKlasse, "school_class": "1a"},
		{"type": activitiesModel.TargetGroupTypeKlasse, "school_class": "2a"},
	}

	response := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	assert.Equal(t, http.StatusBadRequest, response.Code, "body=%s", response.Body.String())
	assert.Contains(t, response.Body.String(), "education_group_id does not reference a group in this tenant")
}

func TestTemplateUpdate_MultipleTargetsRejectsCrossTenantEducationGroup(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	currentGroup := testpkg.CreateTestEducationGroup(t, s.db, "Tpl-Update-Current-Group")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "education.groups", currentGroup.ID) })
	otherTenant := testpkg.NewTenantScope(t, s.db)
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, s.db, otherTenant.TenantID) })
	otherGroup := testpkg.CreateTestEducationGroupForTenant(t, s.db, otherTenant.TenantID, "Tpl-Update-Cross-Tenant")

	body := createTemplateBody(s, fmt.Sprintf("Tpl-Update-Target-%d", time.Now().UnixNano()))
	body["education_group_id"] = currentGroup.ID
	body["target_group_type"] = activitiesModel.TargetGroupTypeGruppe
	body["targets"] = []map[string]any{
		{"type": activitiesModel.TargetGroupTypeGruppe, "education_group_id": currentGroup.ID},
	}
	createdResponse := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, createdResponse.Code, "body=%s", createdResponse.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createdResponse)

	body["targets"] = []map[string]any{
		{"type": activitiesModel.TargetGroupTypeGruppe, "education_group_id": currentGroup.ID},
		{"type": activitiesModel.TargetGroupTypeGruppe, "education_group_id": otherGroup.ID},
	}
	response := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), body)

	assert.Equal(t, http.StatusBadRequest, response.Code, "body=%s", response.Body.String())
	assert.Contains(t, response.Body.String(), "education_group_id does not reference a group in this tenant")
}

func TestTemplateCreate_RejectsInvalidZielgruppe(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	body := createTemplateBody(s, "Tpl-BadZielgruppe")
	body["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
	// target_grade_level intentionally omitted — jahrgang requires it.

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestTemplateCreateRejectsForeignTopLevelEducationGroupWithDynamicTargets(t *testing.T) {
	s := buildTemplateSetup(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	const foreignTenantID = int64(42)
	testpkg.EnsureTestTenant(t, s.db, foreignTenantID)
	foreignGroup := testpkg.CreateTestEducationGroupForTenant(t, s.db, foreignTenantID, "Tpl-ForeignTarget")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "education.groups", foreignGroup.ID)
	})

	name := fmt.Sprintf("Tpl-ForeignTopLevelTarget-%d", time.Now().UnixNano())
	body := createTemplateBody(s, name)
	body["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
	body["target_grade_level"] = 3
	body["education_group_id"] = foreignGroup.ID
	body["targets"] = []map[string]any{{
		"type":        activitiesModel.TargetGroupTypeJahrgang,
		"grade_level": 3,
	}}

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "education_group_id does not reference a group in this tenant")

	count, err := s.db.NewSelect().
		TableExpr(`activities.groups AS "group"`).
		Where(`"group".tenant_id = ?`, tenant.FromContext(s.ctx)).
		Where(`"group".name = ?`, name).
		Count(s.ctx)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestTemplateCreate_EnforcesTenantGradeLevelMax(t *testing.T) {
	t.Run("rejects an above-cap Jahrgang before writing", func(t *testing.T) {
		s := buildTemplateSetup(t, nil)
		defer s.cleanupFn()
		s.res.SettingsService = templateGradeSettings(4, nil)
		router := templateRouter(s.ctx, s.res)

		name := fmt.Sprintf("Tpl-GradeCap-Create-%d", time.Now().UnixNano())
		body := createTemplateBody(s, name)
		body["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
		body["target_grade_level"] = 5

		w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
		require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		assert.Contains(t, w.Body.String(), "target_grade_level 5 exceeds tenant maximum 4")

		count, err := s.db.NewSelect().
			TableExpr(`activities.groups AS "group"`).
			Where(`"group".tenant_id = ?`, tenant.FromContext(s.ctx)).
			Where(`"group".name = ?`, name).
			Count(s.ctx)
		require.NoError(t, err)
		assert.Zero(t, count)
		timeframes, err := scheduleRepo.NewTimeframeRepository(s.db).FindByDescription(s.ctx, name)
		require.NoError(t, err)
		assert.Empty(t, timeframes, "grade validation must run before timeframe creation")
	})

	t.Run("settings failure returns 500 before writing", func(t *testing.T) {
		s := buildTemplateSetup(t, nil)
		defer s.cleanupFn()
		s.res.SettingsService = templateGradeSettings(0, errors.New("settings unavailable"))
		router := templateRouter(s.ctx, s.res)

		name := fmt.Sprintf("Tpl-GradeCap-Settings-%d", time.Now().UnixNano())
		body := createTemplateBody(s, name)
		w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
		require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())

		count, err := s.db.NewSelect().
			TableExpr(`activities.groups AS "group"`).
			Where(`"group".tenant_id = ?`, tenant.FromContext(s.ctx)).
			Where(`"group".name = ?`, name).
			Count(s.ctx)
		require.NoError(t, err)
		assert.Zero(t, count)
		timeframes, err := scheduleRepo.NewTimeframeRepository(s.db).FindByDescription(s.ctx, name)
		require.NoError(t, err)
		assert.Empty(t, timeframes)
	})
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
		{name: "weekend weekday", mutate: func(b map[string]any) { b["weekdays"] = []int{activitiesModel.WeekdaySaturday} }},
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
		{name: "weekend weekday", path: "/templates/500", mutate: func(b map[string]any) { b["weekdays"] = []int{activitiesModel.WeekdaySunday} }, want: http.StatusBadRequest},
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

	missingPeriod := doTemplateJSON(t, router, http.MethodGet, "/templates/500", nil)
	assert.Equal(t, http.StatusBadRequest, missingPeriod.Code)

	period := createTemplateTestPeriod(t, s.db, "Tpl-Missing-Read")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
	getMissing := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/500?period_id=%d", period.ID), nil)
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
	periodBEnrollment.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, activitiesRepo.NewStudentEnrollmentRepository(s.db).Create(s.ctx, periodBEnrollment))

	globalEnrollment := &activitiesModel.StudentEnrollment{
		StudentID:       studentC.ID,
		ActivityGroupID: created.TemplateID,
		ValidFrom:       timezone.NewDate(2026, time.January, 1),
	}
	globalEnrollment.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, activitiesRepo.NewStudentEnrollmentRepository(s.db).Create(s.ctx, globalEnrollment))

	periodBSupervisor := &activitiesModel.SupervisorPlanned{
		StaffID:          s.staffB,
		GroupID:          created.TemplateID,
		ValidFrom:        periodB.StartDate,
		CalendarPeriodID: &periodB.ID,
	}
	periodBSupervisor.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, activitiesRepo.NewSupervisorPlannedRepository(s.db).Create(s.ctx, periodBSupervisor))

	globalSupervisor := &activitiesModel.SupervisorPlanned{
		StaffID:   staffC.ID,
		GroupID:   created.TemplateID,
		ValidFrom: timezone.NewDate(2026, time.January, 1),
	}
	globalSupervisor.SetTenantID(testpkg.Tenant(t))
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
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", created.TemplateID).
		Where("calendar_period_id = ?", periodB.ID).
		Where("valid_until IS NULL").
		Scan(s.ctx, &periodBOtherStudents))
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.student_enrollments").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", created.TemplateID).
		Where("calendar_period_id IS NULL").
		Where("valid_until IS NULL").
		Scan(s.ctx, &globalStudents))
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.supervisors").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("group_id = ?", created.TemplateID).
		Where("calendar_period_id = ?", periodB.ID).
		Where("valid_until IS NULL").
		Scan(s.ctx, &periodBOtherStaff))
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.supervisors").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", testpkg.Tenant(t)).
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
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", created.TemplateID).
		Where("student_id = ?", s.studentA).
		Where("calendar_period_id = ?", periodA.ID).
		Scan(s.ctx, &oldStudentUntil))
	require.NoError(t, s.db.NewSelect().
		TableExpr("activities.student_enrollments").
		Column("valid_from").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", created.TemplateID).
		Where("student_id = ?", studentD.ID).
		Where("calendar_period_id = ?", periodA.ID).
		Where("valid_until IS NULL").
		Scan(s.ctx, &newStudentFrom))
	assert.Equal(t, periodA.StartDate.Format(dateLayout), oldStudentUntil.Format(dateLayout))
	assert.Equal(t, periodA.StartDate.Format(dateLayout), newStudentFrom.Format(dateLayout))
}

func TestUpdateTemplateCanMoveToAnotherCalendarPeriod(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	periodA := createTemplateTestPeriod(t, s.db, "Tpl-Move-Period-A")
	periodB := createTemplateTestPeriod(t, s.db, "Tpl-Move-Period-B")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", periodA.ID, periodB.ID)
	})

	createBody := createTemplateBody(s, "Tpl-Move-Period")
	createBody["calendar_period_id"] = periodA.ID
	createdW := doTemplateJSON(t, router, http.MethodPost, "/templates", createBody)
	require.Equal(t, http.StatusCreated, createdW.Code, "body=%s", createdW.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createdW)

	updateBody := createTemplateBody(s, "Tpl-Moved-Period")
	updateBody["calendar_period_id"] = periodB.ID
	updatedW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updatedW.Code, "body=%s", updatedW.Body.String())
	updated := decodeTemplateData[templateResponse](t, updatedW)
	require.NotEmpty(t, updated.Schedules)
	for _, schedule := range updated.Schedules {
		require.NotNil(t, schedule.CalendarPeriodID)
		assert.Equal(t, periodB.ID, *schedule.CalendarPeriodID)
	}
}

func TestGetTemplateExposesProtectedStudentWeekdays(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	period := createTemplateTestPeriod(t, s.db, "Tpl-Protected-Read")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID)
	})

	body := createTemplateBody(s, "Tpl-Protected-Read")
	body["calendar_period_id"] = period.ID
	body["student_ids"] = []int64{}
	createdW := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, createdW.Code, "body=%s", createdW.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createdW)

	protected := &activitiesModel.StudentEnrollment{
		StudentID:        s.studentA,
		ActivityGroupID:  created.TemplateID,
		ValidFrom:        period.StartDate,
		CalendarPeriodID: &period.ID,
		SelectedWeekdays: []int{activitiesModel.WeekdayMonday},
	}
	protected.SetTenantID(tenant.FromContext(s.ctx))
	require.NoError(t, activitiesRepo.NewStudentEnrollmentRepository(s.db).Create(s.ctx, protected))

	getW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
	require.Equal(t, http.StatusOK, getW.Code, "body=%s", getW.Body.String())
	got := decodeTemplateData[templateResponse](t, getW)
	assert.Equal(t, []templateProtectedStudentAssignmentResponse{{
		Weekday:    activitiesModel.WeekdayMonday,
		StudentIDs: []int64{s.studentA},
	}}, got.ProtectedStudentAssignments)
}

// TestListTemplatesEnrollmentCountIsPeriodTolerant is the regression test for
// the "0 Kinder" bug (WP-B6): template cards lost their headcount when the
// list was filtered by a period other than the one the roster was written
// for. The people subqueries must NOT be period-filtered — the roster of a
// template that is shown is always shown; rosters are period-scoped at write
// time only.
func TestListTemplatesEnrollmentCountIsPeriodTolerant(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	studentC := testpkg.CreateTestStudent(t, s.db, "Tpl", fmt.Sprintf("StudentC-%d", time.Now().UnixNano()), "3a")
	staffC := testpkg.CreateTestStaff(t, s.db, "Tpl", fmt.Sprintf("StaffC-%d", time.Now().UnixNano()))
	defer func() {
		testpkg.CleanupTableRecords(t, s.db, "users.students", studentC.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.staff", staffC.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.persons", studentC.PersonID, staffC.PersonID)
	}()
	// Register this defer last so template roster rows are removed before the
	// additional student/staff records above.
	defer s.cleanupFn()

	s.res.SettingsService = &configtest.Mock{
		HasTenantOverrideFn: func(context.Context, string) (bool, error) { return true, nil },
		ResolveIntFn:        func(context.Context, string) (int, error) { return 1, nil },
	}
	router := templateRouter(s.ctx, s.res)

	// Two OVERLAPPING active periods (createTemplateTestPeriod uses the same
	// 2026-01-01..2026-12-31 range for both, is_active=true).
	periodP := createTemplateTestPeriod(t, s.db, "TplTolerantP")
	periodQ := createTemplateTestPeriod(t, s.db, "TplTolerantQ")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", periodP.ID, periodQ.ID)
	})

	// Template 1: schedules AND roster scoped to P via the regular create path.
	bodyP := createTemplateBody(s, "Tpl-Tolerant-P")
	bodyP["calendar_period_id"] = periodP.ID
	bodyP["student_ids"] = []int64{s.studentA, s.studentB}
	wP := doTemplateJSON(t, router, http.MethodPost, "/templates", bodyP)
	require.Equal(t, http.StatusCreated, wP.Code, "body=%s", wP.Body.String())
	createdP := decodeTemplateData[createTemplateResponse](t, wP)
	// Legacy templates can carry the period only on the group. The
	// materializer falls back to that group pin when a schedule is NULL; the
	// list filter must use the same precedence instead of treating it as
	// globally visible.
	_, err := s.db.NewUpdate().
		Table("activities.schedules").
		Set("calendar_period_id = NULL").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", createdP.TemplateID).
		Exec(s.ctx)
	require.NoError(t, err)

	// Template 2: unscoped schedules (card visible under EVERY period filter)
	// but a roster written for period P — the exact "0 Kinder" constellation.
	bodyGlobal := createTemplateBody(s, "Tpl-Tolerant-Global")
	bodyGlobal["student_ids"] = []int64{}
	bodyGlobal["staff_ids"] = []int64{}
	delete(bodyGlobal, "primary_staff_id")
	wG := doTemplateJSON(t, router, http.MethodPost, "/templates", bodyGlobal)
	require.Equal(t, http.StatusCreated, wG.Code, "body=%s", wG.Body.String())
	createdGlobal := decodeTemplateData[createTemplateResponse](t, wG)

	// Template 3: one phase-bounded enrollment. Decision-approved care offers
	// use a finite valid_until, so it is intentionally absent from the editor's
	// open-row display roster but must still drive capacity while its window
	// overlaps the selected period.
	bodyBounded := createTemplateBody(s, "Tpl-Tolerant-Bounded")
	bodyBounded["calendar_period_id"] = periodP.ID
	bodyBounded["student_ids"] = []int64{}
	bodyBounded["staff_ids"] = []int64{}
	delete(bodyBounded, "primary_staff_id")
	wBounded := doTemplateJSON(t, router, http.MethodPost, "/templates", bodyBounded)
	require.Equal(t, http.StatusCreated, wBounded.Code, "body=%s", wBounded.Body.String())
	createdBounded := decodeTemplateData[createTemplateResponse](t, wBounded)
	boundedUntil := periodP.EndDate
	boundedEnrollment := &activitiesModel.StudentEnrollment{
		StudentID:        s.studentA,
		ActivityGroupID:  createdBounded.TemplateID,
		ValidFrom:        periodP.StartDate,
		ValidUntil:       &boundedUntil,
		CalendarPeriodID: &periodP.ID,
	}
	boundedEnrollment.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.res.TimetableData.CreateStudentEnrollment(s.ctx, boundedEnrollment))

	for _, roster := range []struct {
		studentID int64
		periodID  *int64
	}{
		{studentID: s.studentA, periodID: &periodP.ID},
		{studentID: s.studentB, periodID: &periodP.ID},
		// Unscoped roster rows apply to the occurrence period selected by
		// materialization. With overlapping periods that is the lowest-ID
		// active period (P), never both P and Q.
		{studentID: studentC.ID, periodID: nil},
	} {
		enrollment := &activitiesModel.StudentEnrollment{
			StudentID:        roster.studentID,
			ActivityGroupID:  createdGlobal.TemplateID,
			ValidFrom:        periodP.StartDate,
			CalendarPeriodID: roster.periodID,
		}
		enrollment.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, s.res.TimetableData.CreateStudentEnrollment(s.ctx, enrollment))
	}
	supervisor := &activitiesModel.SupervisorPlanned{
		StaffID:          s.staffA,
		GroupID:          createdGlobal.TemplateID,
		ValidFrom:        periodP.StartDate,
		CalendarPeriodID: &periodP.ID,
	}
	supervisor.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.res.TimetableData.CreatePlannedSupervisor(s.ctx, supervisor))
	// A bounded staff assignment contributes only on dates inside its own
	// validity window. Occurrence-level capacity must count it there without
	// smearing it across the rest of the period.
	boundedSupervisorUntil := periodP.EndDate
	boundedSupervisor := &activitiesModel.SupervisorPlanned{
		StaffID:          s.staffB,
		GroupID:          createdGlobal.TemplateID,
		ValidFrom:        periodP.StartDate,
		ValidUntil:       &boundedSupervisorUntil,
		CalendarPeriodID: &periodP.ID,
	}
	boundedSupervisor.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.res.TimetableData.CreatePlannedSupervisor(s.ctx, boundedSupervisor))
	// Staff assigned only to overlapping period Q must stay visible in the
	// period-tolerant roster, but must not make period P's 1/3 capacity look
	// fully staffed.
	for _, staffID := range []int64{s.staffB, staffC.ID} {
		periodQSupervisor := &activitiesModel.SupervisorPlanned{
			StaffID:          staffID,
			GroupID:          createdGlobal.TemplateID,
			ValidFrom:        periodQ.StartDate,
			CalendarPeriodID: &periodQ.ID,
		}
		periodQSupervisor.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, s.res.TimetableData.CreatePlannedSupervisor(s.ctx, periodQSupervisor))
	}

	listFor := func(t *testing.T, periodID int64) map[int64]templateResponse {
		t.Helper()
		w := doTemplateJSON(t, router, http.MethodGet, fmt.Sprintf("/templates?period_id=%d", periodID), nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		list := decodeTemplateData[listTemplatesResponse](t, w)
		byID := make(map[int64]templateResponse, len(list.Templates))
		for _, tpl := range list.Templates {
			byID[tpl.ID] = tpl
		}
		return byID
	}

	t.Run("filter by the roster's own period P", func(t *testing.T) {
		byID := listFor(t, periodP.ID)

		tplP, ok := byID[createdP.TemplateID]
		require.True(t, ok, "P-scoped template must appear under its own period")
		assert.Equal(t, 2, tplP.EnrollmentCount)
		assert.ElementsMatch(t, []int64{s.studentA, s.studentB}, tplP.StudentIDs)

		tplGlobal, ok := byID[createdGlobal.TemplateID]
		require.True(t, ok, "unscoped template must appear under every period")
		assert.Equal(t, 3, tplGlobal.EnrollmentCount)
		assert.Equal(t, 3, tplGlobal.SupervisorCount,
			"display roster remains the union across periods")
		assert.Equal(t, 3, tplGlobal.RequiredStaffCount)
		assert.Equal(t, 2, tplGlobal.AssignedStaffCount,
			"bounded period P staff counts on its actual dates; period Q staff stays excluded")
		globalDetailW := doTemplateJSON(t, router, http.MethodGet,
			fmt.Sprintf("/templates/%d?period_id=%d", createdGlobal.TemplateID, periodP.ID), nil)
		require.Equal(t, http.StatusOK, globalDetailW.Code, "body=%s", globalDetailW.Body.String())
		globalDetail := decodeTemplateData[templateResponse](t, globalDetailW)
		assert.Equal(t, 3, globalDetail.RequiredStaffCount,
			"fully unpinned detail must expose the worst actual active-period occurrence")
		assert.Equal(t, 2, globalDetail.AssignedStaffCount)

		tplBounded, ok := byID[createdBounded.TemplateID]
		require.True(t, ok, "unscoped template with bounded roster must appear")
		assert.Equal(t, 0, tplBounded.EnrollmentCount,
			"bounded decision roster stays out of the open editor roster")
		assert.Equal(t, 1, tplBounded.RequiredStaffCount,
			"bounded enrollment overlapping P must still drive capacity")

		getW := doTemplateJSON(t, router, http.MethodGet,
			fmt.Sprintf("/templates/%d?period_id=%d", createdBounded.TemplateID, periodP.ID), nil)
		require.Equal(t, http.StatusOK, getW.Code, "body=%s", getW.Body.String())
		boundedDetail := decodeTemplateData[templateResponse](t, getW)
		assert.Equal(t, 1, boundedDetail.RequiredStaffCount,
			"single-template GET must use its unambiguous period pin")
	})

	t.Run("filter by overlapping period Q keeps display roster but has no occurrences", func(t *testing.T) {
		byID := listFor(t, periodQ.ID)
		_, pVisible := byID[createdP.TemplateID]
		assert.False(t, pVisible,
			"an unpinned schedule must inherit its group-level P pin, not appear in Q")

		// The card with unscoped schedules appears under Q — and must carry
		// its P-scoped roster instead of "0 Kinder".
		tplGlobal, ok := byID[createdGlobal.TemplateID]
		require.True(t, ok, "unscoped template must appear under period Q")
		assert.Equal(t, 3, tplGlobal.EnrollmentCount,
			"roster written for overlapping period P must still be counted (the 0-Kinder bug)")
		assert.Equal(t, 3, tplGlobal.SupervisorCount)
		assert.ElementsMatch(t, []int64{s.studentA, s.studentB, studentC.ID}, tplGlobal.StudentIDs)
		assert.Zero(t, tplGlobal.RequiredStaffCount,
			"materialization assigns globally unpinned overlapping dates to lower-ID period P")
		assert.Zero(t, tplGlobal.AssignedStaffCount,
			"period-Q roster must not imply coverage for occurrences owned by P")

		_, boundedVisible := byID[createdBounded.TemplateID]
		assert.False(t, boundedVisible,
			"P-pinned template must not appear under period Q")
	})
}

func TestListTemplatesCapacityUsesActualOccurrences(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	s.res.SettingsService = &configtest.Mock{
		HasTenantOverrideFn: func(context.Context, string) (bool, error) { return true, nil },
		ResolveIntFn:        func(context.Context, string) (int, error) { return 1, nil },
	}
	router := templateRouter(s.ctx, s.res)

	t.Run("deduplicates explicit and dynamic class membership", func(t *testing.T) {
		period := createTemplateTestPeriodRange(
			t,
			s.db,
			"TplOccurrenceDynamicOverlap",
			timezone.NewDate(2025, 9, 1),
			timezone.NewDate(2025, 9, 7),
			1,
			nil,
		)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Dynamic-Overlap", period.ID,
			[]int{activitiesModel.WeekdayMonday}, 0)
		class := " 3A "
		targetRepo, ok := activitiesRepo.NewGroupRepository(s.db).(activitiesModel.GroupTargetRepository)
		require.True(t, ok)
		require.NoError(t, targetRepo.ReplaceTargets(s.ctx, templateID, []*activitiesModel.GroupTarget{
			{TargetGroupType: activitiesModel.TargetGroupTypeKlasse, TargetSchoolClass: &class},
		}))
		start := timezone.NewDate(2025, 9, 1)
		end := timezone.NewDate(2025, 9, 8)
		createCapacityEnrollment(t, s, templateID, s.studentA, start, &end, &period.ID, nil)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Equal(t, 2, got.EnrollmentCount,
			"the displayed child count must union explicit and dynamic students")
		assert.Equal(t, 2, got.RequiredStaffCount,
			"the explicit student must be counted once while both 3a students match case-insensitively")
	})

	t.Run("does not combine weekday cohorts or validity windows", func(t *testing.T) {
		period := createTemplateTestPeriodRange(
			t,
			s.db,
			"TplOccurrenceRoster",
			timezone.NewDate(2025, 9, 1),
			timezone.NewDate(2025, 9, 7),
			1,
			nil,
		)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Roster", period.ID,
			[]int{activitiesModel.WeekdayMonday, activitiesModel.WeekdayWednesday}, 0)

		start := timezone.NewDate(2025, 9, 1)
		end := timezone.NewDate(2025, 9, 7).AddDays(1)
		createCapacityEnrollment(t, s, templateID, s.studentA, start, &end, &period.ID, []int{activitiesModel.WeekdayMonday})
		createCapacityEnrollment(t, s, templateID, s.studentB, start, &end, &period.ID, []int{activitiesModel.WeekdayWednesday})
		mondayEnd := timezone.NewDate(2025, 9, 2)
		wednesdayStart := timezone.NewDate(2025, 9, 3)
		wednesdayEnd := timezone.NewDate(2025, 9, 4)
		createCapacitySupervisor(t, s, templateID, s.staffA, start, &mondayEnd, &period.ID)
		createCapacitySupervisor(t, s, templateID, s.staffB, wednesdayStart, &wednesdayEnd, &period.ID)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Equal(t, 1, got.RequiredStaffCount,
			"Monday and Wednesday children must not be unioned into one requirement")
		assert.Equal(t, 1, got.AssignedStaffCount,
			"staff validity must be evaluated on the chosen occurrence")

		unfiltered := listCapacityTemplateFromListPath(t, router, "/templates", templateID)
		assert.Equal(t, 1, unfiltered.RequiredStaffCount,
			"the unfiltered list must use actual occurrences instead of the display-roster union")
		assert.Equal(t, 1, unfiltered.AssignedStaffCount)

		detailW := doTemplateJSON(t, router, http.MethodGet,
			fmt.Sprintf("/templates/%d?period_id=%d", templateID, period.ID), nil)
		require.Equal(t, http.StatusOK, detailW.Code, "body=%s", detailW.Body.String())
		detail := decodeTemplateData[templateResponse](t, detailW)
		assert.Equal(t, 1, detail.RequiredStaffCount,
			"template detail must use actual occurrences instead of the display-roster union")
		assert.Equal(t, 1, detail.AssignedStaffCount)
	})

	t.Run("honors inclusive schedule start and exclusive schedule end", func(t *testing.T) {
		period := createTemplateTestPeriodRange(
			t,
			s.db,
			"TplOccurrenceScheduleWindow",
			timezone.NewDate(2025, 9, 1),
			timezone.NewDate(2025, 9, 21),
			1,
			nil,
		)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Schedule-Window", period.ID,
			[]int{activitiesModel.WeekdayMonday, activitiesModel.WeekdayWednesday}, 0)
		windowStart := timezone.NewDate(2025, 9, 10)
		windowEnd := timezone.NewDate(2025, 9, 11)
		setCapacityScheduleWindow(t, s, templateID, activitiesModel.WeekdayWednesday, &windowStart, &windowEnd)

		phantomStart := timezone.NewDate(2025, 9, 3)
		phantomEnd := timezone.NewDate(2025, 9, 4)
		futureStart := timezone.NewDate(2025, 9, 17)
		futureEnd := timezone.NewDate(2025, 9, 18)
		createCapacityEnrollment(t, s, templateID, s.studentA, phantomStart, &phantomEnd, &period.ID, []int{activitiesModel.WeekdayWednesday})
		createCapacityEnrollment(t, s, templateID, s.studentB, windowStart, &windowEnd, &period.ID, []int{activitiesModel.WeekdayWednesday})
		createCapacityEnrollment(t, s, templateID, s.studentA, futureStart, &futureEnd, &period.ID, []int{activitiesModel.WeekdayWednesday})
		createCapacitySupervisor(t, s, templateID, s.staffA, windowStart, &windowEnd, &period.ID)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Equal(t, 1, got.RequiredStaffCount)
		assert.Equal(t, 1, got.AssignedStaffCount,
			"out-of-window unstaffed Wednesdays must not become candidates")
	})

	t.Run("applies the calendar period week cycle", func(t *testing.T) {
		anchor := timezone.NewDate(2025, 9, 1)
		period := createTemplateTestPeriodRange(
			t,
			s.db,
			"TplOccurrenceABWeek",
			anchor,
			timezone.NewDate(2025, 9, 14),
			2,
			&anchor,
		)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-AB-Week", period.ID,
			[]int{activitiesModel.WeekdayMonday}, 2)
		weekAEnd := timezone.NewDate(2025, 9, 2)
		createCapacityEnrollment(t, s, templateID, s.studentA, anchor, &weekAEnd, &period.ID, nil)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Zero(t, got.RequiredStaffCount,
			"the week-A roster must not create demand for a week-B-only schedule")
		assert.Zero(t, got.AssignedStaffCount)
	})

	t.Run("removes cancelled recurrence dates from staffing", func(t *testing.T) {
		date := timezone.NewDate(2025, 9, 3)
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceCancelled",
			timezone.NewDate(2025, 9, 1), timezone.NewDate(2025, 9, 7), 1, nil)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Cancelled", period.ID,
			[]int{activitiesModel.WeekdayWednesday}, 0)
		end := date.AddDays(1)
		createCapacityEnrollment(t, s, templateID, s.studentA, date, &end, &period.ID, nil)
		exception := &scheduleModel.ActivityException{
			ActivityGroupID: templateID,
			ExceptionDate:   date,
			ExceptionType:   scheduleModel.ActivityExceptionCancelled,
		}
		exception.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, scheduleRepo.NewActivityExceptionRepository(s.db).Create(s.ctx, exception))

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Zero(t, got.RequiredStaffCount)
		assert.Zero(t, got.AssignedStaffCount)
	})

	t.Run("ignores schedules without a materializable timeframe", func(t *testing.T) {
		date := timezone.NewDate(2025, 9, 1)
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceNoTimeframe",
			date, timezone.NewDate(2025, 9, 7), 1, nil)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-No-Timeframe", period.ID,
			[]int{activitiesModel.WeekdayMonday}, 0)
		createCapacityEnrollment(t, s, templateID, s.studentA, date, nil, &period.ID, nil)

		_, err := s.db.NewUpdate().
			Table("activities.schedules").
			Set("timeframe_id = NULL").
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Where("activity_group_id = ?", templateID).
			Exec(s.ctx)
		require.NoError(t, err)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Equal(t, 1, got.EnrollmentCount, "the display roster remains visible")
		assert.Zero(t, got.RequiredStaffCount,
			"materialization skips schedules whose timeframe was removed")
		assert.Zero(t, got.AssignedStaffCount)
	})

	t.Run("ignores dates without an effective room", func(t *testing.T) {
		date := timezone.NewDate(2025, 9, 1)
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceNoRoom",
			date, timezone.NewDate(2025, 9, 7), 1, nil)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-No-Room", period.ID,
			[]int{activitiesModel.WeekdayMonday}, 0)
		createCapacityEnrollment(t, s, templateID, s.studentA, date, nil, &period.ID, nil)

		_, err := s.db.NewUpdate().
			Table("activities.groups").
			Set("planned_room_id = NULL").
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Where("id = ?", templateID).
			Exec(s.ctx)
		require.NoError(t, err)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Equal(t, 1, got.EnrollmentCount, "the display roster remains visible")
		assert.Zero(t, got.RequiredStaffCount,
			"materialization skips dates with neither a template room nor an override")
		assert.Zero(t, got.AssignedStaffCount)
	})

	t.Run("accepts a date-specific room override when the template room is missing", func(t *testing.T) {
		date := timezone.NewDate(2025, 9, 1)
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceRoomOverride",
			date, timezone.NewDate(2025, 9, 7), 1, nil)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Room-Override", period.ID,
			[]int{activitiesModel.WeekdayMonday}, 0)
		createCapacityEnrollment(t, s, templateID, s.studentA, date, nil, &period.ID, nil)

		_, err := s.db.NewUpdate().
			Table("activities.groups").
			Set("planned_room_id = NULL").
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Where("id = ?", templateID).
			Exec(s.ctx)
		require.NoError(t, err)
		exception := &scheduleModel.ActivityException{
			ActivityGroupID: templateID,
			ExceptionDate:   date,
			ExceptionType:   scheduleModel.ActivityExceptionModified,
			RoomID:          &s.roomID,
		}
		exception.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, scheduleRepo.NewActivityExceptionRepository(s.db).Create(s.ctx, exception))

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Equal(t, 1, got.RequiredStaffCount,
			"the materializer uses the exception room as the effective room")
		assert.Zero(t, got.AssignedStaffCount)
	})

	t.Run("keeps simultaneous explicit periods as separate occurrences", func(t *testing.T) {
		start := timezone.NewDate(2025, 9, 1)
		end := timezone.NewDate(2025, 9, 7)
		periodP := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceExplicitP", start, end, 1, nil)
		periodQ := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceExplicitQ", start, end, 1, nil)
		t.Cleanup(func() {
			testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", periodP.ID, periodQ.ID)
		})

		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Explicit-Periods", periodP.ID,
			[]int{activitiesModel.WeekdayMonday}, 0)
		_, err := s.db.NewUpdate().
			Table("activities.groups").
			Set("calendar_period_id = NULL").
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Where("id = ?", templateID).
			Exec(s.ctx)
		require.NoError(t, err)

		startTime, err := parseClockTime("13:00")
		require.NoError(t, err)
		endTime, err := parseClockTime("13:50")
		require.NoError(t, err)
		timeframe := &scheduleModel.Timeframe{
			StartTime:   startTime,
			EndTime:     &endTime,
			IsActive:    true,
			Description: "Tpl-Occurrence-Explicit-Periods-Q",
		}
		timeframe.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, s.res.TimetableData.CreateTimeframe(s.ctx, timeframe))

		timeframeID := timeframe.ID
		scheduleQ := &activitiesModel.Schedule{
			Weekday:          activitiesModel.WeekdayMonday,
			TimeframeID:      &timeframeID,
			ActivityGroupID:  templateID,
			WeekPattern:      0,
			CalendarPeriodID: &periodQ.ID,
		}
		scheduleQ.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, s.res.TimetableData.CreateActivitySchedule(s.ctx, scheduleQ))

		createCapacityEnrollment(t, s, templateID, s.studentA, start, nil, &periodP.ID, nil)
		createCapacityEnrollment(t, s, templateID, s.studentB, start, nil, &periodQ.ID, nil)
		createCapacitySupervisor(t, s, templateID, s.staffA, start, nil, &periodP.ID)

		periodPResult := listCapacityTemplate(t, router, periodP.ID, templateID)
		assert.Equal(t, 1, periodPResult.RequiredStaffCount)
		assert.Equal(t, 1, periodPResult.AssignedStaffCount)
		periodQResult := listCapacityTemplate(t, router, periodQ.ID, templateID)
		assert.Equal(t, 1, periodQResult.RequiredStaffCount)
		assert.Zero(t, periodQResult.AssignedStaffCount)

		unfiltered := listCapacityTemplateFromListPath(t, router, "/templates", templateID)
		assert.Equal(t, 1, unfiltered.RequiredStaffCount,
			"all-period capacity must not union period-specific rosters on the same date")
		assert.Zero(t, unfiltered.AssignedStaffCount,
			"the unstaffed period-Q occurrence is the real worst case")

		periodPDetailW := doTemplateJSON(t, router, http.MethodGet,
			fmt.Sprintf("/templates/%d?period_id=%d", templateID, periodP.ID), nil)
		require.Equal(t, http.StatusOK, periodPDetailW.Code, "body=%s", periodPDetailW.Body.String())
		periodPDetail := decodeTemplateData[templateResponse](t, periodPDetailW)
		assert.Equal(t, 1, periodPDetail.EnrollmentCount)
		assert.Equal(t, []int64{s.studentA}, periodPDetail.StudentIDs)
		assert.Equal(t, 1, periodPDetail.SupervisorCount)
		assert.Equal(t, []int64{s.staffA}, periodPDetail.StaffIDs)
		assert.Equal(t, 1, periodPDetail.RequiredStaffCount)
		assert.Equal(t, 1, periodPDetail.AssignedStaffCount)

		periodQDetailW := doTemplateJSON(t, router, http.MethodGet,
			fmt.Sprintf("/templates/%d?period_id=%d", templateID, periodQ.ID), nil)
		require.Equal(t, http.StatusOK, periodQDetailW.Code, "body=%s", periodQDetailW.Body.String())
		periodQDetail := decodeTemplateData[templateResponse](t, periodQDetailW)
		assert.Equal(t, 1, periodQDetail.EnrollmentCount)
		assert.Equal(t, []int64{s.studentB}, periodQDetail.StudentIDs)
		assert.Zero(t, periodQDetail.SupervisorCount)
		assert.Empty(t, periodQDetail.StaffIDs)
		assert.Equal(t, 1, periodQDetail.RequiredStaffCount)
		assert.Zero(t, periodQDetail.AssignedStaffCount)
	})

	t.Run("inactive period has no materializable staffing occurrences", func(t *testing.T) {
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceInactive",
			timezone.NewDate(2025, 9, 1), timezone.NewDate(2025, 9, 7), 1, nil)
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Inactive", period.ID,
			[]int{activitiesModel.WeekdayMonday}, 0)
		end := period.StartDate.AddDays(1)
		createCapacityEnrollment(t, s, templateID, s.studentA, period.StartDate, &end, &period.ID, nil)
		_, err := s.db.NewUpdate().Table("schedule.calendar_periods").
			Set("is_active = FALSE").
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Where("id = ?", period.ID).
			Exec(s.ctx)
		require.NoError(t, err)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Zero(t, got.RequiredStaffCount)
		assert.Zero(t, got.AssignedStaffCount)
	})
}

func createCapacityTemplate(
	t *testing.T,
	router chi.Router,
	s *templateSetup,
	name string,
	periodID int64,
	weekdays []int,
	weekPattern int,
) int64 {
	t.Helper()
	body := createTemplateBody(s, name)
	body["calendar_period_id"] = periodID
	body["weekdays"] = weekdays
	body["week_pattern"] = weekPattern
	body["student_ids"] = []int64{}
	body["staff_ids"] = []int64{}
	delete(body, "primary_staff_id")
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[createTemplateResponse](t, w).TemplateID
}

func createCapacityEnrollment(
	t *testing.T,
	s *templateSetup,
	templateID, studentID int64,
	validFrom timezone.Date,
	validUntil *timezone.Date,
	periodID *int64,
	selectedWeekdays []int,
) {
	t.Helper()
	enrollment := &activitiesModel.StudentEnrollment{
		StudentID:        studentID,
		ActivityGroupID:  templateID,
		ValidFrom:        validFrom,
		ValidUntil:       validUntil,
		CalendarPeriodID: periodID,
		SelectedWeekdays: selectedWeekdays,
	}
	enrollment.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, activitiesRepo.NewStudentEnrollmentRepository(s.db).Create(s.ctx, enrollment))
}

func createCapacitySupervisor(
	t *testing.T,
	s *templateSetup,
	templateID, staffID int64,
	validFrom timezone.Date,
	validUntil *timezone.Date,
	periodID *int64,
) {
	t.Helper()
	supervisor := &activitiesModel.SupervisorPlanned{
		StaffID:          staffID,
		GroupID:          templateID,
		ValidFrom:        validFrom,
		ValidUntil:       validUntil,
		CalendarPeriodID: periodID,
	}
	supervisor.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, activitiesRepo.NewSupervisorPlannedRepository(s.db).Create(s.ctx, supervisor))
}

func setCapacityScheduleWindow(
	t *testing.T,
	s *templateSetup,
	templateID int64,
	weekday int,
	validFrom, validUntil *timezone.Date,
) {
	t.Helper()
	_, err := s.db.NewUpdate().
		Table("activities.schedules").
		Set("valid_from = ?", validFrom).
		Set("valid_until = ?", validUntil).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", templateID).
		Where("weekday = ?", weekday).
		Exec(s.ctx)
	require.NoError(t, err)
}

func TestTemplateScheduleResponseIncludesValidityBounds(t *testing.T) {
	row := templateRow{
		ScheduleID:         9,
		Weekday:            1,
		StartTime:          sql.NullString{String: "14:00", Valid: true},
		EndTime:            sql.NullString{String: "15:00", Valid: true},
		WeekPattern:        0,
		ScheduleValidFrom:  sql.NullString{String: "2026-05-04", Valid: true},
		ScheduleValidUntil: sql.NullString{String: "2026-06-01", Valid: true},
	}

	response := templateScheduleResponseFromRow(row)

	assert.Equal(t, "2026-05-04", response.ValidFrom)
	assert.Equal(t, "2026-06-01", response.ValidUntil)
}

func listCapacityTemplate(t *testing.T, router chi.Router, periodID, templateID int64) templateResponse {
	t.Helper()
	return listCapacityTemplateFromListPath(
		t,
		router,
		fmt.Sprintf("/templates?period_id=%d", periodID),
		templateID,
	)
}

func listCapacityTemplateFromListPath(t *testing.T, router chi.Router, path string, templateID int64) templateResponse {
	t.Helper()
	w := doTemplateJSON(t, router, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	list := decodeTemplateData[listTemplatesResponse](t, w)
	for _, template := range list.Templates {
		if template.ID == templateID {
			return template
		}
	}
	require.Failf(t, "template missing", "template %d not present in %s", templateID, path)
	return templateResponse{}
}

func createTemplateTestPeriod(t *testing.T, db *bun.DB, name string) *scheduleModel.CalendarPeriod {
	t.Helper()
	return createTemplateTestPeriodRange(
		t,
		db,
		name,
		timezone.NewDate(2026, 1, 1),
		timezone.NewDate(2026, 12, 31),
		1,
		nil,
	)
}

func createTemplateTestPeriodRange(
	t *testing.T,
	db *bun.DB,
	name string,
	startDate, endDate timezone.Date,
	weekCycleLength int,
	weekCycleAnchor *timezone.Date,
) *scheduleModel.CalendarPeriod {
	t.Helper()
	period := &scheduleModel.CalendarPeriod{
		Name:            fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		PeriodType:      scheduleModel.PeriodTypeCustom,
		StartDate:       startDate,
		EndDate:         endDate,
		WeekCycleLength: weekCycleLength,
		WeekCycleAnchor: weekCycleAnchor,
		IsActive:        true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().
		Model(period).
		ModelTableExpr("schedule.calendar_periods").
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	return period
}

// TestTemplate_WochennotizRoundTrip covers the durable series note through
// create -> list/get -> update -> clear, plus the create-time length guard
// (#1837 follow-up).
func TestTemplate_WochennotizRoundTrip(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	notesOf := func(templateID int64) *string {
		listW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
		require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
		list := decodeTemplateData[listTemplatesResponse](t, listW)
		for _, c := range list.Templates {
			if c.ID == templateID {
				return c.Notes
			}
		}
		t.Fatalf("template %d not found in list", templateID)
		return nil
	}

	// Create with a Wochennotiz.
	body := createTemplateBody(s, "Tpl-Wochennotiz")
	body["notes"] = "Raum erst ab 14 Uhr offen"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "activities.groups", created.TemplateID) })

	got := notesOf(created.TemplateID)
	require.NotNil(t, got, "series note must round-trip onto the template response")
	assert.Equal(t, "Raum erst ab 14 Uhr offen", *got)

	// Update the note.
	upd := createTemplateBody(s, "Tpl-Wochennotiz")
	upd["notes"] = "Neuer Hinweis"
	uw := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), upd)
	require.Equal(t, http.StatusOK, uw.Code, "body=%s", uw.Body.String())
	got = notesOf(created.TemplateID)
	require.NotNil(t, got)
	assert.Equal(t, "Neuer Hinweis", *got)

	// Update omitting notes clears the series note.
	clear := createTemplateBody(s, "Tpl-Wochennotiz")
	cw := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), clear)
	require.Equal(t, http.StatusOK, cw.Code, "body=%s", cw.Body.String())
	assert.Nil(t, notesOf(created.TemplateID), "omitted note clears the series note")
}

func TestTemplate_CreateRejectsOverlongNotes(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	body := createTemplateBody(s, "Tpl-LongNote")
	body["notes"] = strings.Repeat("x", 2001)
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	assert.Equal(t, http.StatusBadRequest, w.Code, "notes over 2000 chars must be rejected; body=%s", w.Body.String())
}

// TestTemplateList_IncludesShiftTypeBadge verifies a template whose category is
// mapped to a Dienstplan shift type exposes shift_type_name/shift_type_color on
// the list response (#1837 follow-up, timetable-view visibility).
func TestTemplateList_IncludesShiftTypeBadge(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	stRepo := repositories.NewFactory(s.db).ShiftType
	catRepo := repositories.NewFactory(s.db).ActivityCategory
	st := &scheduleModel.ShiftType{Name: fmt.Sprintf("Betreuung-%d", time.Now().UnixNano()), Color: "#83CD2D", IsActive: true}
	require.NoError(t, stRepo.Create(s.ctx, st))
	t.Cleanup(func() { _ = stRepo.Delete(s.ctx, st.ID) })
	require.NoError(t, catRepo.SetShiftTypeForCategories(s.ctx, st.ID, []int64{s.category.ID}))

	body := createTemplateBody(s, "Tpl-ShiftBadge")
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "activities.groups", created.TemplateID) })

	listW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
	require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
	list := decodeTemplateData[listTemplatesResponse](t, listW)
	var tpl templateResponse
	for _, c := range list.Templates {
		if c.ID == created.TemplateID {
			tpl = c
			break
		}
	}
	require.Equal(t, created.TemplateID, tpl.ID)
	assert.Equal(t, st.Name, tpl.ShiftTypeName, "template list must expose the mapped shift type name")
	assert.Equal(t, "#83CD2D", tpl.ShiftTypeColor)
}

// #2135: the optional start_date on POST /templates becomes the series start.
// Every schedule row gets it as valid_from (the materializer skips earlier
// dates) and the initial roster is valid from that date instead of the
// period start.
func TestTemplateCreateWithStartDateStampsValidity(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	period := createTemplateTestPeriod(t, s.db, "TplStartDatePeriod")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID)
	})

	body := createTemplateBody(s, "Tpl-StartDate")
	body["calendar_period_id"] = period.ID
	body["start_date"] = "2026-08-13"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	tpl := listCapacityTemplate(t, router, period.ID, created.TemplateID)
	require.Len(t, tpl.Schedules, 2)
	for _, sched := range tpl.Schedules {
		assert.Equal(t, "2026-08-13", sched.ValidFrom, "schedule valid_from must be the series start")
		assert.Empty(t, sched.ValidUntil)
	}

	assertTemplateRosterValidFrom(t, s, created.TemplateID, timezone.NewDate(2026, 8, 13))
}

// #2135: without a pinned calendar period the start_date still stamps the
// schedule and roster validity (schedules resolve their period per date).
func TestTemplateCreateWithStartDateWithoutPeriod(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	body := createTemplateBody(s, "Tpl-StartDate-NoPeriod")
	body["start_date"] = "2026-08-13"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	tpl := listCapacityTemplateFromListPath(t, router, "/templates", created.TemplateID)
	require.Len(t, tpl.Schedules, 2)
	for _, sched := range tpl.Schedules {
		assert.Equal(t, "2026-08-13", sched.ValidFrom)
	}

	assertTemplateRosterValidFrom(t, s, created.TemplateID, timezone.NewDate(2026, 8, 13))
}

// #2135: start_date format and period-bounds violations are 400s, and the
// omitted field keeps the legacy behavior (roster anchored on the period
// start, schedules open-ended).
func TestTemplateCreateStartDateValidation(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	period := createTemplateTestPeriod(t, s.db, "TplStartDateValidationPeriod")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID)
	})

	badFormat := createTemplateBody(s, "Tpl-StartDate-BadFormat")
	badFormat["start_date"] = "13.08.2026"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", badFormat)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "invalid start_date format")

	// Period runs 2026-01-01 to 2026-12-31 (createTemplateTestPeriod).
	outside := createTemplateBody(s, "Tpl-StartDate-Outside")
	outside["calendar_period_id"] = period.ID
	outside["start_date"] = "2027-01-01"
	w = doTemplateJSON(t, router, http.MethodPost, "/templates", outside)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "start_date must lie within the calendar period")

	before := createTemplateBody(s, "Tpl-StartDate-Before")
	before["calendar_period_id"] = period.ID
	before["start_date"] = "2025-12-31"
	w = doTemplateJSON(t, router, http.MethodPost, "/templates", before)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	omitted := createTemplateBody(s, "Tpl-StartDate-Omitted")
	omitted["calendar_period_id"] = period.ID
	w = doTemplateJSON(t, router, http.MethodPost, "/templates", omitted)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)
	tpl := listCapacityTemplate(t, router, period.ID, created.TemplateID)
	require.NotEmpty(t, tpl.Schedules)
	for _, sched := range tpl.Schedules {
		assert.Empty(t, sched.ValidFrom, "omitted start_date must leave schedules open-started")
	}
	assertTemplateRosterValidFrom(t, s, created.TemplateID, period.StartDate)
}

// assertTemplateRosterValidFrom checks that every enrollment and supervisor
// row of the template carries the expected valid_from anchor.
func assertTemplateRosterValidFrom(
	t *testing.T,
	s *templateSetup,
	templateID int64,
	expected timezone.Date,
) {
	t.Helper()
	var enrollmentFroms []string
	require.NoError(t, s.db.NewSelect().
		Table("activities.student_enrollments").
		ColumnExpr("valid_from::text").
		Where("activity_group_id = ?", templateID).
		Scan(s.ctx, &enrollmentFroms))
	require.NotEmpty(t, enrollmentFroms, "template must have enrollments")
	for _, from := range enrollmentFroms {
		assert.Equal(t, expected.String(), from, "enrollment valid_from")
	}

	var supervisorFroms []string
	require.NoError(t, s.db.NewSelect().
		Table("activities.supervisors").
		ColumnExpr("valid_from::text").
		Where("group_id = ?", templateID).
		Scan(s.ctx, &supervisorFroms))
	require.NotEmpty(t, supervisorFroms, "template must have supervisors")
	for _, from := range supervisorFroms {
		assert.Equal(t, expected.String(), from, "supervisor valid_from")
	}
}
