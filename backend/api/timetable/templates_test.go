package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func templateDateStringPtr(date *calendar.Date) *string {
	if date == nil {
		return nil
	}
	value := date.String()
	return &value
}

// templateCategory is the category fixture the template bodies reference.
type templateCategory struct {
	ID int64
}

type templateSetup struct {
	res *Resource
	// owner is the Timetable owner capability the retained schedule,
	// enrollment, supervisor and timeframe adapters delegate to; the suites
	// arrange and read those rows through it.
	owner     timetable.Capability
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	category  templateCategory
	staffA    int64
	staffB    int64
	studentA  int64
	studentB  int64
	cleanupFn func()
}

// listTimeframesByDescription lists the timeframes whose description contains
// description (case-insensitive), through the Timetable owner.
func listTimeframesByDescription(
	t *testing.T,
	s *templateSetup,
	ctx context.Context,
	description string,
) []timetable.Timeframe {
	t.Helper()
	timeframes, err := s.owner.ListTimeframes(ctx, timetable.TimeframeFilter{DescriptionContains: description})
	require.NoError(t, err)
	return timeframes
}

// templateSchedules lists the template's schedule rows through the owner.
func templateSchedules(t *testing.T, s *templateSetup, templateID int64) []timetable.Schedule {
	t.Helper()
	rows, err := s.owner.ListSchedules(s.ctx, timetable.ScheduleFilter{GroupIDs: []int64{templateID}})
	require.NoError(t, err)
	return rows
}

// templateEnrollments lists the template's enrollment rows, oldest first.
func templateEnrollments(t *testing.T, s *templateSetup, templateID int64) []timetable.StudentEnrollment {
	t.Helper()
	rows, err := s.owner.ListStudentEnrollments(s.ctx, timetable.StudentEnrollmentFilter{
		ActivityGroupIDs: []int64{templateID},
		OrderByValidFrom: true,
	})
	require.NoError(t, err)
	return rows
}

// templateSupervisors lists the template's planned supervisor rows.
func templateSupervisors(t *testing.T, s *templateSetup, templateID int64) []timetable.PlannedSupervisor {
	t.Helper()
	rows, err := s.owner.ListPlannedSupervisors(s.ctx, timetable.PlannedSupervisorFilter{GroupIDs: []int64{templateID}})
	require.NoError(t, err)
	return rows
}

// replaceTemplateTargets writes a template's dynamic target list the way the
// retained group adapter did: every target is validated (trimming its class)
// before the owner replaces the list.
func replaceTemplateTargets(t *testing.T, s *templateSetup, templateID int64, targets ...timetable.GroupTargetInput) {
	t.Helper()
	for index := range targets {
		require.NoError(t, targets[index].ValidateDynamicTarget())
	}
	require.NoError(t, s.owner.ReplaceGroupTargets(s.ctx, templateID, targets))
}

// newCreateArg allocates the row a retained Create method accepts, so a
// suite can arrange through that adapter without naming its model type.
func newCreateArg[T any](func(context.Context, *T) error) *T { return new(T) }

type mockMaterializationService struct {
	result *timetable.MaterializationResult
	err    error
	from   calendar.Date
	to     calendar.Date
	source timetable.MaterializationSource
	// detectFn drives DetectEditedInWindow; nil returns (nil, nil).
	detectFn func(activityGroupID int64, from, to calendar.Date, includeDeletions bool) ([]timetable.EditedOccurrence, error)
}

func TestValidateLegacyTemplateWorkdays(t *testing.T) {
	t.Parallel()

	existing := []templateScheduleResponse{
		{Weekday: timetable.WeekdayFriday},
		{Weekday: timetable.WeekdaySaturday},
	}

	assert.NoError(t, validateLegacyTemplateWorkdays(existing, []int{
		timetable.WeekdayFriday,
		timetable.WeekdaySaturday,
	}))
	assert.NoError(t, validateLegacyTemplateWorkdays(existing, []int{
		timetable.WeekdayFriday,
	}))
	assert.Error(t, validateLegacyTemplateWorkdays(existing, []int{
		timetable.WeekdayFriday,
		timetable.WeekdaySunday,
	}))
}

func (m *mockMaterializationService) MaterializeForTenant(_ context.Context, from, to calendar.Date, source timetable.MaterializationSource) (*timetable.MaterializationResult, error) {
	m.from = from
	m.to = to
	m.source = source
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockMaterializationService) ResolveWindow(baseDate calendar.Date, weeksAhead int) (calendar.Date, calendar.Date) {
	return baseDate, baseDate.AddDays(weeksAhead*7 - 1)
}

func (m *mockMaterializationService) DetectEditedInWindow(_ context.Context, activityGroupID int64, from, to calendar.Date, includeDeletions bool) ([]timetable.EditedOccurrence, error) {
	if m.detectFn != nil {
		return m.detectFn(activityGroupID, from, to, includeDeletions)
	}
	return nil, nil
}

func buildTemplateModule(t *testing.T, mat timetable.MaterializationCapability, clocks ...func() time.Time) *templateSetup {
	t.Helper()
	db, serviceFactory := testutil.SetupTimetableModule(t, clocks...)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Tpl-Room-%d", suffix))
	category := testpkg.CreateTestActivityCategory(t, db, fmt.Sprintf("Tpl-Cat-%d", suffix))
	staffA := testpkg.CreateTestStaff(t, db, "Tpl", fmt.Sprintf("StaffA-%d", suffix))
	staffB := testpkg.CreateTestStaff(t, db, "Tpl", fmt.Sprintf("StaffB-%d", suffix))
	studentA := testpkg.CreateTestStudent(t, db, "Tpl", fmt.Sprintf("StudentA-%d", suffix), "3a")
	studentB := testpkg.CreateTestStudent(t, db, "Tpl", fmt.Sprintf("StudentB-%d", suffix), "3a")
	repoFactory := mustTimetableTestRepositories(db)

	data := testTimetableData(db, clocks...)
	res := NewResource(Dependencies{
		Templates:              data,
		TimetableData:          data.TimetableData(),
		CalendarPeriods:        repoFactory.SchoolCalendar(),
		CalendarPeriodUsage:    calendarPeriodUsageFor(repoFactory),
		MaterializationService: mat,
		InstanceService:        serviceFactory.Instance,
		// 4 is the settings registry default of the tenant grade-level maximum.
		SettingsService: templateGradeSettings(4, nil),
		Now:             firstTemplateClock(clocks),
		DB:              db,
	})
	converter, err := timetableCompose.NewInstanceSeriesConversion(timetableCompose.InstanceSeriesConversionDependencies{
		DB:             db,
		InstanceRepo:   repoFactory.ActivityInstance,
		Lifecycle:      serviceFactory.Instance,
		Templates:      res.Templates,
		RecurrenceLock: data.RecurrenceLock(),
	})
	require.NoError(t, err)
	res.InstanceSeriesConverter = converter

	cleanup := func() {
	}

	return &templateSetup{
		res:       res,
		owner:     repoFactory.Timetable,
		db:        db,
		ctx:       ctx,
		roomID:    room.ID,
		category:  templateCategory{ID: category.ID},
		staffA:    staffA.ID,
		staffB:    staffB.ID,
		studentA:  studentA.ID,
		studentB:  studentB.ID,
		cleanupFn: cleanup,
	}
}

func firstTemplateClock(clocks []func() time.Time) func() time.Time {
	if len(clocks) == 0 {
		return nil
	}
	return clocks[0]
}

func fixedTemplateClock() time.Time {
	return calendar.NewDate(2026, 8, 24).BerlinMidnight().Add(12 * time.Hour)
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
	t.Parallel()

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

func templateRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
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
		"type":             timetable.GroupTypeCare,
		"weekdays":         []int{timetable.WeekdayMonday, timetable.WeekdayWednesday},
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
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
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
	t.Parallel()

	mat := &mockMaterializationService{
		result: &timetable.MaterializationResult{InstancesCreated: 3},
	}
	s := buildTemplateModule(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)
	educationGroup := testpkg.CreateTestEducationGroup(t, s.db, "Tpl-EducationGroup")

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
	assert.Equal(t, timetable.MaterializationSourceManual, mat.source)

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
	assert.Equal(t, timetable.GroupTypeCare, tpl.Type)
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
	getW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
	require.Equal(t, http.StatusOK, getW.Code, "body=%s", getW.Body.String())
	got := decodeTemplateData[templateResponse](t, getW)
	assert.Equal(t, created.TemplateID, got.ID)
	require.NotNil(t, got.EducationGroupID)
	assert.Equal(t, educationGroup.ID, *got.EducationGroupID)
	assert.Equal(t, educationGroup.Name, got.EducationGroupName)

	updateBody := createTemplateBody(s, "Tpl-Updated")
	updateBody["type"] = timetable.GroupTypeActivity
	updateBody["education_group_id"] = educationGroup.ID
	updateBody["weekdays"] = []int{timetable.WeekdayFriday}
	updateBody["start_time"] = "13:15"
	updateBody["end_time"] = "14:00"
	updateBody["student_ids"] = []int64{s.studentB}
	updateBody["staff_ids"] = []int64{s.staffA}
	updateBody["primary_staff_id"] = s.staffA
	updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())
	updated := decodeTemplateData[templateResponse](t, updateW)
	assert.Equal(t, "Tpl-Updated", updated.Name)
	assert.Equal(t, timetable.GroupTypeActivity, updated.Type)
	assert.Equal(t, []int64{s.studentB}, updated.StudentIDs)
	assert.Equal(t, []int64{s.staffA}, updated.StaffIDs)
	require.Len(t, updated.Schedules, 1)
	assert.Equal(t, timetable.WeekdayFriday, updated.Schedules[0].Weekday)
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
	t.Parallel()

	mat := &mockMaterializationService{result: &timetable.MaterializationResult{}}
	clock := func() time.Time {
		return calendar.NewDate(2026, 8, 24).BerlinMidnight().Add(12 * time.Hour)
	}
	s := buildTemplateModule(t, mat, clock)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	// Create the series already classified as "mensa".
	body := createTemplateBody(s, fmt.Sprintf("Tpl-ListKind-%d", time.Now().UnixNano()))
	body["list_kind"] = timetable.ListKindMensa
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)
	require.NotZero(t, created.TemplateID)

	today := calendar.NewDate(2026, 8, 24)
	mkInstance := func(name string, date calendar.Date, hour int, listKind *string) timetable.ActivityInstance {
		tmplID := created.TemplateID
		inst, err := s.owner.CreateActivityInstance(s.ctx, timetable.ActivityInstanceInput{
			Date:            date.String(),
			ActivityGroupID: &tmplID,
			Title:           name,
			StartTime:       fmt.Sprintf("%02d:00:00", hour),
			EndTime:         fmt.Sprintf("%02d:00:00", hour+1),
			RoomID:          s.roomID,
			Status:          timetable.InstanceStatusPlanned,
			ListKind:        listKind,
		})
		require.NoError(t, err)
		return inst
	}

	// futureRow still carries the series value → should adopt the new kind.
	futureRow := mkInstance("Future", today.AddDays(7), 8, testpkg.StrPtr(timetable.ListKindMensa))
	// overriddenRow was individually re-classified → must be preserved.
	overriddenRow := mkInstance("Overridden", today.AddDays(7), 9, testpkg.StrPtr(timetable.ListKindActivity))
	// pastRow is elapsed → must be preserved.
	pastRow := mkInstance("Past", today.AddDays(-7), 8, testpkg.StrPtr(timetable.ListKindMensa))

	// Re-classify the series to "learning_time" via the template PUT.
	updateBody := createTemplateBody(s, "Tpl-ListKind-Updated")
	updateBody["list_kind"] = timetable.ListKindLearningTime
	updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())

	gotFuture, err := s.owner.FindActivityInstance(s.ctx, futureRow.ID)
	require.NoError(t, err)
	require.NotNil(t, gotFuture.ListKind)
	assert.Equal(t, timetable.ListKindLearningTime, *gotFuture.ListKind,
		"future occurrence must adopt the series' new Listenart")

	gotOverridden, err := s.owner.FindActivityInstance(s.ctx, overriddenRow.ID)
	require.NoError(t, err)
	require.NotNil(t, gotOverridden.ListKind)
	assert.Equal(t, timetable.ListKindActivity, *gotOverridden.ListKind,
		"per-occurrence override must survive the series edit")

	gotPast, err := s.owner.FindActivityInstance(s.ctx, pastRow.ID)
	require.NoError(t, err)
	require.NotNil(t, gotPast.ListKind)
	assert.Equal(t, timetable.ListKindMensa, *gotPast.ListKind,
		"past occurrence must be left untouched")
}

func TestListTemplates_CapacityFields(t *testing.T) {
	t.Parallel()

	mat := &mockMaterializationService{result: &timetable.MaterializationResult{}}
	s := buildTemplateModule(t, mat, fixedTemplateClock)
	defer s.cleanupFn()

	// Tenant-override ratio of 1 child per staff member, exercised via the
	// same HasTenantOverride -> ResolveInt chain the real settings service
	// uses (settings-system.md), not just the registry default.
	s.res.SettingsService = &configtest.Mock{
		HasTenantOverrideFn: func(context.Context, string) (bool, error) { return true, nil },
		ResolveIntFn:        func(context.Context, string) (int, error) { return 1, nil },
	}
	today := calendar.NewDate(2030, 8, 26)
	createTemplateTestPeriodRange(
		t,
		s.db,
		"TplCapacityPeriod",
		today.AddDays(-7),
		today.AddDays(14),
		1,
		nil,
	)

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
	assert.Equal(t, timetable.TargetGroupTypeNone, tpl.TargetGroupType, "default target group type for templates predating Zielgruppe")
	assert.Nil(t, tpl.TargetGradeLevel)
	assert.Nil(t, tpl.TargetSchoolClass)
	assert.Nil(t, tpl.CalendarPeriodID, "no calendar period set on this template")
}

func TestTemplateCreateUpdate_ZielgruppeRoundTrip(t *testing.T) {
	t.Parallel()

	mat := &mockMaterializationService{result: &timetable.MaterializationResult{}}
	s := buildTemplateModule(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	body := createTemplateBody(s, "Tpl-Zielgruppe")
	body["target_group_type"] = timetable.TargetGroupTypeGrade
	body["target_grade_level"] = 3

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	period := createTemplateTestPeriod(t, s.db, "Tpl-Zielgruppe-Read")
	getW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
	require.Equal(t, http.StatusOK, getW.Code, "body=%s", getW.Body.String())
	got := decodeTemplateData[templateResponse](t, getW)
	assert.Equal(t, timetable.TargetGroupTypeGrade, got.TargetGroupType)
	require.NotNil(t, got.TargetGradeLevel)
	assert.EqualValues(t, 3, *got.TargetGradeLevel)
	assert.Nil(t, got.TargetSchoolClass)

	// Switch to Klasse on update; grade level must clear (mutually exclusive).
	updateBody := createTemplateBody(s, "Tpl-Zielgruppe-Updated")
	updateBody["target_group_type"] = timetable.TargetGroupTypeSchoolClass
	updateBody["target_school_class"] = "3a"
	updateBody["targets"] = []map[string]any{
		{"type": timetable.TargetGroupTypeSchoolClass, "school_class": "3a"},
	}
	updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())
	updated := decodeTemplateData[templateResponse](t, updateW)
	assert.Equal(t, timetable.TargetGroupTypeSchoolClass, updated.TargetGroupType)
	assert.Nil(t, updated.TargetGradeLevel)
	require.NotNil(t, updated.TargetSchoolClass)
	assert.Equal(t, "3a", *updated.TargetSchoolClass)
}

func TestTemplateCreate_MultipleTargetsRoundTrip(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{result: &timetable.MaterializationResult{}})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)
	period := createTemplateTestPeriod(t, s.db, "Tpl-Multiple-Targets-Read")

	body := createTemplateBody(s, fmt.Sprintf("Tpl-Multiple-Targets-%d", time.Now().UnixNano()))
	body["target_group_type"] = timetable.TargetGroupTypeSchoolClass
	body["target_school_class"] = "1a"
	body["targets"] = []map[string]any{
		{"type": timetable.TargetGroupTypeSchoolClass, "school_class": "1a"},
		{"type": timetable.TargetGroupTypeSchoolClass, "school_class": "2a"},
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
	updateBody["target_group_type"] = timetable.TargetGroupTypeSchoolClass
	updateBody["target_school_class"] = "2a"
	updateBody["targets"] = []map[string]any{
		{"type": timetable.TargetGroupTypeSchoolClass, "school_class": "2a"},
		{"type": timetable.TargetGroupTypeSchoolClass, "school_class": "3a"},
	}
	updatedResponse := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusOK, updatedResponse.Code, "body=%s", updatedResponse.Body.String())
	updated := decodeTemplateData[templateResponse](t, updatedResponse)
	require.Len(t, updated.Targets, 2)
	assert.Equal(t, "2a", *updated.Targets[0].SchoolClass)
	assert.Equal(t, "3a", *updated.Targets[1].SchoolClass)

	legacyUpdateBody := createTemplateBody(s, fmt.Sprintf("Tpl-Multiple-Targets-Legacy-%d", time.Now().UnixNano()))
	legacyUpdateBody["target_group_type"] = timetable.TargetGroupTypeSchoolClass
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
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	otherTenant := testpkg.NewTenantScope(t, s.db)
	otherGroup := testpkg.CreateTestEducationGroupForTenant(t, s.db, otherTenant.TenantID, "Tpl-Cross-Tenant")

	body := createTemplateBody(s, fmt.Sprintf("Tpl-Cross-Tenant-Target-%d", time.Now().UnixNano()))
	body["education_group_id"] = otherGroup.ID
	body["target_group_type"] = timetable.TargetGroupTypeSchoolClass
	body["targets"] = []map[string]any{
		{"type": timetable.TargetGroupTypeSchoolClass, "school_class": "1a"},
		{"type": timetable.TargetGroupTypeSchoolClass, "school_class": "2a"},
	}

	response := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	assert.Equal(t, http.StatusBadRequest, response.Code, "body=%s", response.Body.String())
	assert.Contains(t, response.Body.String(), "education_group_id does not reference a group in this tenant")
}

func TestTemplateUpdate_MultipleTargetsRejectsCrossTenantEducationGroup(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	currentGroup := testpkg.CreateTestEducationGroup(t, s.db, "Tpl-Update-Current-Group")
	otherTenant := testpkg.NewTenantScope(t, s.db)
	otherGroup := testpkg.CreateTestEducationGroupForTenant(t, s.db, otherTenant.TenantID, "Tpl-Update-Cross-Tenant")

	body := createTemplateBody(s, fmt.Sprintf("Tpl-Update-Target-%d", time.Now().UnixNano()))
	body["education_group_id"] = currentGroup.ID
	body["target_group_type"] = timetable.TargetGroupTypeEducationGroup
	body["targets"] = []map[string]any{
		{"type": timetable.TargetGroupTypeEducationGroup, "education_group_id": currentGroup.ID},
	}
	createdResponse := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, createdResponse.Code, "body=%s", createdResponse.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createdResponse)

	body["targets"] = []map[string]any{
		{"type": timetable.TargetGroupTypeEducationGroup, "education_group_id": currentGroup.ID},
		{"type": timetable.TargetGroupTypeEducationGroup, "education_group_id": otherGroup.ID},
	}
	response := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), body)

	assert.Equal(t, http.StatusBadRequest, response.Code, "body=%s", response.Body.String())
	assert.Contains(t, response.Body.String(), "education_group_id does not reference a group in this tenant")
}

func TestTemplateCreate_RejectsInvalidZielgruppe(t *testing.T) {
	t.Parallel()

	mat := &mockMaterializationService{result: &timetable.MaterializationResult{}}
	s := buildTemplateModule(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	body := createTemplateBody(s, "Tpl-BadZielgruppe")
	body["target_group_type"] = timetable.TargetGroupTypeGrade
	// target_grade_level intentionally omitted — jahrgang requires it.

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestTemplateCreateRejectsForeignTopLevelEducationGroupWithDynamicTargets(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, s.db, foreignTenantID)
	foreignGroup := testpkg.CreateTestEducationGroupForTenant(t, s.db, foreignTenantID, "Tpl-ForeignTarget")

	name := fmt.Sprintf("Tpl-ForeignTopLevelTarget-%d", time.Now().UnixNano())
	body := createTemplateBody(s, name)
	body["target_group_type"] = timetable.TargetGroupTypeGrade
	body["target_grade_level"] = 3
	body["education_group_id"] = foreignGroup.ID
	body["targets"] = []map[string]any{{
		"type":        timetable.TargetGroupTypeGrade,
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
	t.Parallel()

	t.Run("rejects an above-cap Jahrgang before writing", func(t *testing.T) {
		s := buildTemplateModule(t, nil)
		defer s.cleanupFn()
		s.res.SettingsService = templateGradeSettings(4, nil)
		router := templateRouter(s.ctx, s.res)

		name := fmt.Sprintf("Tpl-GradeCap-Create-%d", time.Now().UnixNano())
		body := createTemplateBody(s, name)
		body["target_group_type"] = timetable.TargetGroupTypeGrade
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
		timeframes := listTimeframesByDescription(t, s, s.ctx, name)
		assert.Empty(t, timeframes, "grade validation must run before timeframe creation")
	})

	t.Run("settings failure returns 500 before writing", func(t *testing.T) {
		s := buildTemplateModule(t, nil)
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
		timeframes := listTimeframesByDescription(t, s, s.ctx, name)
		assert.Empty(t, timeframes)
	})
}

func TestTemplateCreateValidationAndMaterializationFailure(t *testing.T) {
	t.Parallel()

	mat := &mockMaterializationService{err: errors.New("materializer unavailable")}
	s := buildTemplateModule(t, mat)
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
		{name: "weekend weekday", mutate: func(b map[string]any) { b["weekdays"] = []int{timetable.WeekdaySaturday} }},
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
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	first := createTemplateBody(s, "Tpl-Reuse-A")
	first["weekdays"] = []int{timetable.WeekdayMonday}
	w1 := doTemplateJSON(t, router, http.MethodPost, "/templates", first)
	require.Equal(t, http.StatusCreated, w1.Code, "body=%s", w1.Body.String())
	createdA := decodeTemplateData[createTemplateResponse](t, w1)

	second := createTemplateBody(s, "Tpl-Reuse-B")
	second["weekdays"] = []int{timetable.WeekdayTuesday}
	w2 := doTemplateJSON(t, router, http.MethodPost, "/templates", second)
	require.Equal(t, http.StatusCreated, w2.Code, "body=%s", w2.Body.String())
	createdB := decodeTemplateData[createTemplateResponse](t, w2)

	assert.Equal(t, createdA.TimeframeID, createdB.TimeframeID,
		"same start/end clock window should reuse the existing timeframe")
}

func TestTemplateUpdateValidationAndNotFound(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, nil)
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
		{name: "weekend weekday", path: "/templates/500", mutate: func(b map[string]any) { b["weekdays"] = []int{timetable.WeekdaySunday} }, want: http.StatusBadRequest},
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
	t.Parallel()

	s := buildTemplateModule(t, nil)
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
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	periodA := createTemplateTestPeriod(t, s.db, "TplPeriodA")
	periodB := createTemplateTestPeriod(t, s.db, "TplPeriodB")

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
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	suffix := time.Now().UnixNano()
	studentC := testpkg.CreateTestStudent(t, s.db, "Tpl", fmt.Sprintf("StudentC-%d", suffix), "3a")
	studentD := testpkg.CreateTestStudent(t, s.db, "Tpl", fmt.Sprintf("StudentD-%d", suffix), "3a")
	staffC := testpkg.CreateTestStaff(t, s.db, "Tpl", fmt.Sprintf("StaffC-%d", suffix))
	staffD := testpkg.CreateTestStaff(t, s.db, "Tpl", fmt.Sprintf("StaffD-%d", suffix))

	periodA := createTemplateTestPeriod(t, s.db, "TplPeoplePeriodA")
	periodB := createTemplateTestPeriod(t, s.db, "TplPeoplePeriodB")

	body := createTemplateBody(s, "Tpl-People-Period-A")
	body["calendar_period_id"] = periodA.ID
	body["student_ids"] = []int64{s.studentA}
	body["staff_ids"] = []int64{s.staffA}
	body["primary_staff_id"] = s.staffA
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	createTemplateEnrollment(t, s, timetable.StudentEnrollmentInput{
		StudentID:        s.studentB,
		ActivityGroupID:  created.TemplateID,
		ValidFrom:        periodB.StartDate.String(),
		CalendarPeriodID: &periodB.ID,
	})
	createTemplateEnrollment(t, s, timetable.StudentEnrollmentInput{
		StudentID:       studentC.ID,
		ActivityGroupID: created.TemplateID,
		ValidFrom:       calendar.NewDate(2026, time.January, 1).String(),
	})
	createTemplateSupervisor(t, s, timetable.PlannedSupervisorInput{
		StaffID:          s.staffB,
		GroupID:          created.TemplateID,
		ValidFrom:        periodB.StartDate.String(),
		CalendarPeriodID: &periodB.ID,
	})
	createTemplateSupervisor(t, s, timetable.PlannedSupervisorInput{
		StaffID:   staffC.ID,
		GroupID:   created.TemplateID,
		ValidFrom: calendar.NewDate(2026, time.January, 1).String(),
	})

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
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	periodA := createTemplateTestPeriod(t, s.db, "Tpl-Move-Period-A")
	periodB := createTemplateTestPeriod(t, s.db, "Tpl-Move-Period-B")

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
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	period := createTemplateTestPeriod(t, s.db, "Tpl-Protected-Read")

	body := createTemplateBody(s, "Tpl-Protected-Read")
	body["calendar_period_id"] = period.ID
	body["student_ids"] = []int64{}
	createdW := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, createdW.Code, "body=%s", createdW.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createdW)

	createTemplateEnrollment(t, s, timetable.StudentEnrollmentInput{
		StudentID:        s.studentA,
		ActivityGroupID:  created.TemplateID,
		ValidFrom:        period.StartDate.String(),
		CalendarPeriodID: &period.ID,
		SelectedWeekdays: []int{timetable.WeekdayMonday},
	})

	getW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
	require.Equal(t, http.StatusOK, getW.Code, "body=%s", getW.Body.String())
	got := decodeTemplateData[templateResponse](t, getW)
	assert.Equal(t, []templateProtectedStudentAssignmentResponse{{
		Weekday:    timetable.WeekdayMonday,
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
	t.Parallel()

	s := buildTemplateModule(t, nil)
	studentC := testpkg.CreateTestStudent(t, s.db, "Tpl", fmt.Sprintf("StudentC-%d", time.Now().UnixNano()), "3a")
	staffC := testpkg.CreateTestStaff(t, s.db, "Tpl", fmt.Sprintf("StaffC-%d", time.Now().UnixNano()))
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
	boundedUntil := periodP.EndDate.String()
	createTemplateEnrollment(t, s, timetable.StudentEnrollmentInput{
		StudentID:        s.studentA,
		ActivityGroupID:  createdBounded.TemplateID,
		ValidFrom:        periodP.StartDate.String(),
		ValidUntil:       &boundedUntil,
		CalendarPeriodID: &periodP.ID,
	})

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
		createTemplateEnrollment(t, s, timetable.StudentEnrollmentInput{
			StudentID:        roster.studentID,
			ActivityGroupID:  createdGlobal.TemplateID,
			ValidFrom:        periodP.StartDate.String(),
			CalendarPeriodID: roster.periodID,
		})
	}
	createTemplateSupervisor(t, s, timetable.PlannedSupervisorInput{
		StaffID:          s.staffA,
		GroupID:          createdGlobal.TemplateID,
		ValidFrom:        periodP.StartDate.String(),
		CalendarPeriodID: &periodP.ID,
	})
	// A bounded staff assignment contributes only on dates inside its own
	// validity window. Occurrence-level capacity must count it there without
	// smearing it across the rest of the period.
	boundedSupervisorUntil := periodP.EndDate.String()
	createTemplateSupervisor(t, s, timetable.PlannedSupervisorInput{
		StaffID:          s.staffB,
		GroupID:          createdGlobal.TemplateID,
		ValidFrom:        periodP.StartDate.String(),
		ValidUntil:       &boundedSupervisorUntil,
		CalendarPeriodID: &periodP.ID,
	})
	// Staff assigned only to overlapping period Q must stay visible in the
	// period-tolerant roster, but must not make period P's 1/3 capacity look
	// fully staffed.
	for _, staffID := range []int64{s.staffB, staffC.ID} {
		createTemplateSupervisor(t, s, timetable.PlannedSupervisorInput{
			StaffID:          staffID,
			GroupID:          createdGlobal.TemplateID,
			ValidFrom:        periodQ.StartDate.String(),
			CalendarPeriodID: &periodQ.ID,
		})
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
	t.Parallel()

	s := buildTemplateModule(t, nil)
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
			calendar.NewDate(2025, 9, 1),
			calendar.NewDate(2025, 9, 7),
			1,
			nil,
		)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Dynamic-Overlap", period.ID,
			[]int{timetable.WeekdayMonday}, 0)
		class := " 3A "
		replaceTemplateTargets(t, s, templateID,
			timetable.GroupTargetInput{TargetGroupType: timetable.TargetGroupTypeSchoolClass, TargetSchoolClass: &class},
		)
		start := calendar.NewDate(2025, 9, 1)
		end := calendar.NewDate(2025, 9, 8)
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
			calendar.NewDate(2025, 9, 1),
			calendar.NewDate(2025, 9, 7),
			1,
			nil,
		)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Roster", period.ID,
			[]int{timetable.WeekdayMonday, timetable.WeekdayWednesday}, 0)

		start := calendar.NewDate(2025, 9, 1)
		end := calendar.NewDate(2025, 9, 7).AddDays(1)
		createCapacityEnrollment(t, s, templateID, s.studentA, start, &end, &period.ID, []int{timetable.WeekdayMonday})
		createCapacityEnrollment(t, s, templateID, s.studentB, start, &end, &period.ID, []int{timetable.WeekdayWednesday})
		mondayEnd := calendar.NewDate(2025, 9, 2)
		wednesdayStart := calendar.NewDate(2025, 9, 3)
		wednesdayEnd := calendar.NewDate(2025, 9, 4)
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
			calendar.NewDate(2025, 9, 1),
			calendar.NewDate(2025, 9, 21),
			1,
			nil,
		)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Schedule-Window", period.ID,
			[]int{timetable.WeekdayMonday, timetable.WeekdayWednesday}, 0)
		windowStart := calendar.NewDate(2025, 9, 10)
		windowEnd := calendar.NewDate(2025, 9, 11)
		setCapacityScheduleWindow(t, s, templateID, timetable.WeekdayWednesday, &windowStart, &windowEnd)

		phantomStart := calendar.NewDate(2025, 9, 3)
		phantomEnd := calendar.NewDate(2025, 9, 4)
		futureStart := calendar.NewDate(2025, 9, 17)
		futureEnd := calendar.NewDate(2025, 9, 18)
		createCapacityEnrollment(t, s, templateID, s.studentA, phantomStart, &phantomEnd, &period.ID, []int{timetable.WeekdayWednesday})
		createCapacityEnrollment(t, s, templateID, s.studentB, windowStart, &windowEnd, &period.ID, []int{timetable.WeekdayWednesday})
		createCapacityEnrollment(t, s, templateID, s.studentA, futureStart, &futureEnd, &period.ID, []int{timetable.WeekdayWednesday})
		createCapacitySupervisor(t, s, templateID, s.staffA, windowStart, &windowEnd, &period.ID)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Equal(t, 1, got.RequiredStaffCount)
		assert.Equal(t, 1, got.AssignedStaffCount,
			"out-of-window unstaffed Wednesdays must not become candidates")
	})

	t.Run("applies the calendar period week cycle", func(t *testing.T) {
		anchor := calendar.NewDate(2025, 9, 1)
		period := createTemplateTestPeriodRange(
			t,
			s.db,
			"TplOccurrenceABWeek",
			anchor,
			calendar.NewDate(2025, 9, 14),
			2,
			&anchor,
		)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-AB-Week", period.ID,
			[]int{timetable.WeekdayMonday}, 2)
		weekAEnd := calendar.NewDate(2025, 9, 2)
		createCapacityEnrollment(t, s, templateID, s.studentA, anchor, &weekAEnd, &period.ID, nil)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Zero(t, got.RequiredStaffCount,
			"the week-A roster must not create demand for a week-B-only schedule")
		assert.Zero(t, got.AssignedStaffCount)
	})

	t.Run("removes cancelled recurrence dates from staffing", func(t *testing.T) {
		date := calendar.NewDate(2025, 9, 3)
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceCancelled",
			calendar.NewDate(2025, 9, 1), calendar.NewDate(2025, 9, 7), 1, nil)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Cancelled", period.ID,
			[]int{timetable.WeekdayWednesday}, 0)
		end := date.AddDays(1)
		createCapacityEnrollment(t, s, templateID, s.studentA, date, &end, &period.ID, nil)
		_, err := s.owner.CreateActivityException(s.ctx, timetable.ActivityExceptionInput{
			ActivityGroupID: templateID,
			ExceptionDate:   date.String(),
			ExceptionType:   timetable.ActivityExceptionCancelled,
		})
		require.NoError(t, err)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Zero(t, got.RequiredStaffCount)
		assert.Zero(t, got.AssignedStaffCount)
	})

	t.Run("ignores schedules without a materializable timeframe", func(t *testing.T) {
		date := calendar.NewDate(2025, 9, 1)
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceNoTimeframe",
			date, calendar.NewDate(2025, 9, 7), 1, nil)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-No-Timeframe", period.ID,
			[]int{timetable.WeekdayMonday}, 0)
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
		date := calendar.NewDate(2025, 9, 1)
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceNoRoom",
			date, calendar.NewDate(2025, 9, 7), 1, nil)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-No-Room", period.ID,
			[]int{timetable.WeekdayMonday}, 0)
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
		date := calendar.NewDate(2025, 9, 1)
		period := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceRoomOverride",
			date, calendar.NewDate(2025, 9, 7), 1, nil)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Room-Override", period.ID,
			[]int{timetable.WeekdayMonday}, 0)
		createCapacityEnrollment(t, s, templateID, s.studentA, date, nil, &period.ID, nil)

		_, err := s.db.NewUpdate().
			Table("activities.groups").
			Set("planned_room_id = NULL").
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Where("id = ?", templateID).
			Exec(s.ctx)
		require.NoError(t, err)
		_, err = s.owner.CreateActivityException(s.ctx, timetable.ActivityExceptionInput{
			ActivityGroupID: templateID,
			ExceptionDate:   date.String(),
			ExceptionType:   timetable.ActivityExceptionModified,
			RoomID:          &s.roomID,
		})
		require.NoError(t, err)

		got := listCapacityTemplate(t, router, period.ID, templateID)
		assert.Equal(t, 1, got.RequiredStaffCount,
			"the materializer uses the exception room as the effective room")
		assert.Zero(t, got.AssignedStaffCount)
	})

	t.Run("keeps simultaneous explicit periods as separate occurrences", func(t *testing.T) {
		start := calendar.NewDate(2025, 9, 1)
		end := calendar.NewDate(2025, 9, 7)
		periodP := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceExplicitP", start, end, 1, nil)
		periodQ := createTemplateTestPeriodRange(t, s.db, "TplOccurrenceExplicitQ", start, end, 1, nil)

		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Explicit-Periods", periodP.ID,
			[]int{timetable.WeekdayMonday}, 0)
		_, err := s.db.NewUpdate().
			Table("activities.groups").
			Set("calendar_period_id = NULL").
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Where("id = ?", templateID).
			Exec(s.ctx)
		require.NoError(t, err)

		endTime := "13:50:00"
		timeframe, err := s.owner.CreateTimeframe(s.ctx, timetable.TimeframeInput{
			StartTime:   "13:00:00",
			EndTime:     &endTime,
			IsActive:    true,
			Description: "Tpl-Occurrence-Explicit-Periods-Q",
		})
		require.NoError(t, err)

		timeframeID := timeframe.ID
		_, err = s.owner.CreateSchedule(s.ctx, timetable.ScheduleInput{
			Weekday:          timetable.WeekdayMonday,
			TimeframeID:      &timeframeID,
			ActivityGroupID:  templateID,
			WeekPattern:      0,
			CalendarPeriodID: &periodQ.ID,
		})
		require.NoError(t, err)

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
			calendar.NewDate(2025, 9, 1), calendar.NewDate(2025, 9, 7), 1, nil)
		templateID := createCapacityTemplate(t, router, s, "Tpl-Occurrence-Inactive", period.ID,
			[]int{timetable.WeekdayMonday}, 0)
		endDate := period.StartDate.AddDays(1)
		createCapacityEnrollment(t, s, templateID, s.studentA, period.StartDate, &endDate, &period.ID, nil)
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
	validFrom calendar.Date,
	validUntil *calendar.Date,
	periodID *int64,
	selectedWeekdays []int,
) {
	t.Helper()
	createTemplateEnrollment(t, s, timetable.StudentEnrollmentInput{
		StudentID:        studentID,
		ActivityGroupID:  templateID,
		ValidFrom:        validFrom.String(),
		ValidUntil:       templateDateStringPtr(validUntil),
		CalendarPeriodID: periodID,
		SelectedWeekdays: selectedWeekdays,
	})
}

func createCapacitySupervisor(
	t *testing.T,
	s *templateSetup,
	templateID, staffID int64,
	validFrom calendar.Date,
	validUntil *calendar.Date,
	periodID *int64,
) {
	t.Helper()
	createTemplateSupervisor(t, s, timetable.PlannedSupervisorInput{
		StaffID:          staffID,
		GroupID:          templateID,
		ValidFrom:        validFrom.String(),
		ValidUntil:       templateDateStringPtr(validUntil),
		CalendarPeriodID: periodID,
	})
}

// createTemplateEnrollment writes one roster row through the Timetable owner.
func createTemplateEnrollment(t *testing.T, s *templateSetup, input timetable.StudentEnrollmentInput) {
	t.Helper()
	_, err := s.owner.CreateStudentEnrollment(s.ctx, input)
	require.NoError(t, err)
}

// createTemplateSupervisor writes one planned supervision row through the
// Timetable owner.
func createTemplateSupervisor(t *testing.T, s *templateSetup, input timetable.PlannedSupervisorInput) {
	t.Helper()
	_, err := s.owner.CreatePlannedSupervisor(s.ctx, input)
	require.NoError(t, err)
}

func setCapacityScheduleWindow(
	t *testing.T,
	s *templateSetup,
	templateID int64,
	weekday int,
	validFrom, validUntil *calendar.Date,
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
	t.Parallel()

	row := templateRow{TemplateListRow: timetable.TemplateListRow{
		ScheduleID:         9,
		Weekday:            1,
		StartTime:          testpkg.StrPtr("14:00"),
		EndTime:            testpkg.StrPtr("15:00"),
		WeekPattern:        0,
		ScheduleValidFrom:  testpkg.StrPtr("2026-05-04"),
		ScheduleValidUntil: testpkg.StrPtr("2026-06-01"),
	}}

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

// templateTestPeriod is a schedule.calendar_periods fixture row.
type templateTestPeriod struct {
	ID              int64          `bun:"id,pk,autoincrement"`
	TenantID        int64          `bun:"tenant_id,notnull"`
	Name            string         `bun:"name,notnull"`
	PeriodType      string         `bun:"period_type,notnull"`
	StartDate       calendar.Date  `bun:"start_date,notnull"`
	EndDate         calendar.Date  `bun:"end_date,notnull"`
	WeekCycleLength int            `bun:"week_cycle_length,notnull"`
	WeekCycleAnchor *calendar.Date `bun:"week_cycle_anchor"`
	IsActive        bool           `bun:"is_active,notnull"`
}

func createTemplateTestPeriod(t *testing.T, db *bun.DB, name string) *templateTestPeriod {
	t.Helper()
	return createTemplateTestPeriodRange(
		t,
		db,
		name,
		calendar.NewDate(2026, 1, 1),
		calendar.NewDate(2026, 12, 31),
		1,
		nil,
	)
}

func createTemplateTestPeriodRange(
	t *testing.T,
	db *bun.DB,
	name string,
	startDate, endDate calendar.Date,
	weekCycleLength int,
	weekCycleAnchor *calendar.Date,
) *templateTestPeriod {
	t.Helper()
	period := &templateTestPeriod{
		TenantID:        testpkg.Tenant(t),
		Name:            fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		PeriodType:      schoolcalendar.PeriodTypeCustom,
		StartDate:       startDate,
		EndDate:         endDate,
		WeekCycleLength: weekCycleLength,
		WeekCycleAnchor: weekCycleAnchor,
		IsActive:        true,
	}
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
	t.Parallel()

	mat := &mockMaterializationService{result: &timetable.MaterializationResult{}}
	s := buildTemplateModule(t, mat)
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
	t.Parallel()

	mat := &mockMaterializationService{result: &timetable.MaterializationResult{}}
	s := buildTemplateModule(t, mat)
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
	t.Parallel()

	mat := &mockMaterializationService{result: &timetable.MaterializationResult{}}
	s := buildTemplateModule(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	stRepo := mustTimetableTestRepositories(s.db).ShiftType
	st := newCreateArg(stRepo.Create)
	st.Name, st.Color, st.IsActive = fmt.Sprintf("Betreuung-%d", time.Now().UnixNano()), "#83CD2D", true
	require.NoError(t, stRepo.Create(s.ctx, st))
	t.Cleanup(func() { _ = stRepo.Delete(s.ctx, st.ID) })
	require.NoError(t, s.owner.SetCategoryShiftTypeID(s.ctx, s.category.ID, &st.ID))

	body := createTemplateBody(s, "Tpl-ShiftBadge")
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

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
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	period := createTemplateTestPeriod(t, s.db, "TplStartDatePeriod")

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

	assertTemplateRosterValidFrom(t, s, created.TemplateID, calendar.NewDate(2026, 8, 13))
}

// #2135: without a pinned calendar period the start_date still stamps the
// schedule and roster validity (schedules resolve their period per date).
func TestTemplateCreateWithStartDateWithoutPeriod(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, nil)
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

	assertTemplateRosterValidFrom(t, s, created.TemplateID, calendar.NewDate(2026, 8, 13))
}

// #2135: start_date format and period-bounds violations are 400s, and the
// omitted field keeps the legacy behavior (roster anchored on the period
// start, schedules open-ended).
func TestTemplateCreateStartDateValidation(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, nil)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	period := createTemplateTestPeriod(t, s.db, "TplStartDateValidationPeriod")

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
	assertTemplateRosterValidFrom(t, s, created.TemplateID, calendar.Date(period.StartDate))
}

// assertTemplateRosterValidFrom checks that every enrollment and supervisor
// row of the template carries the expected valid_from anchor.
func assertTemplateRosterValidFrom(
	t *testing.T,
	s *templateSetup,
	templateID int64,
	expected calendar.Date,
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
