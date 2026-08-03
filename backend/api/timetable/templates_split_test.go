// Handler tests for POST /templates/{id}/split (WP-B3).
//
// Hermetic: real repos + real split service against the test DB (fixtures via
// buildTemplateSetup); materialization mocked so no calendar period is
// required. The router mirrors the production wiring: tenant context +
// permission middleware, so the 403 path exercises the real gate.
package timetable

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// attachSplitService wires a real TemplateSplitService (real repos, mocked
// materialization) into the test resource. Same package, so the unexported
// field is assignable.
func attachSplitService(s *templateSetup, mat scheduleSvc.MaterializationService) {
	attachSplitServiceWithValidator(s, mat, func(context.Context, int64) error { return nil })
}

func attachSplitServiceWithValidator(
	s *templateSetup,
	mat scheduleSvc.MaterializationService,
	validate func(context.Context, int64) error,
) {
	s.res.TemplateSplitService = scheduleSvc.NewTemplateSplitService(scheduleSvc.TemplateSplitDependencies{
		GroupRepo:                  activitiesRepo.NewGroupRepository(s.db),
		CategoryRepo:               activitiesRepo.NewCategoryRepository(s.db),
		ScheduleRepo:               activitiesRepo.NewScheduleRepository(s.db),
		EnrollmentRepo:             activitiesRepo.NewStudentEnrollmentRepository(s.db),
		SupervisorRepo:             activitiesRepo.NewSupervisorPlannedRepository(s.db),
		InstanceRepo:               scheduleRepo.NewActivityInstanceRepository(s.db),
		TimeframeRepo:              scheduleRepo.NewTimeframeRepository(s.db),
		Materialization:            mat,
		InstanceService:            s.res.InstanceService,
		ValidateCareOfferingSeries: validate,
		DB:                         s.db,
	})
}

// splitRouter mounts the split route behind the production permission gate
// plus the create route (ungated, setup convenience).
func splitRouter(parentCtx context.Context, res *Resource, perms []string) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			ctx = context.WithValue(ctx, jwt.CtxPermissions, perms)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Use(tenant.TenantTxMiddleware(res.DB))
	r.Post("/templates", res.createTemplate)
	r.Get("/templates", res.listTemplates)
	r.Get("/templates/{id}", res.getTemplate)
	r.Put("/templates/{id}", res.updateTemplate)
	r.With(authorize.RequiresPermission(permissions.SchedulesManage)).
		Post("/templates/{id}/split", res.splitTemplate)
	r.With(authorize.RequiresPermission(permissions.SchedulesManage)).
		Post("/templates/{id}/end", res.endTemplate)
	return r
}

// createSourceTemplate creates the template the split tests operate on.
func createSourceTemplate(t *testing.T, router chi.Router, s *templateSetup, name string) createTemplateResponse {
	t.Helper()
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", createTemplateBody(s, name))
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[createTemplateResponse](t, w)
}

func createJahrgangSourceTemplate(
	t *testing.T,
	router chi.Router,
	s *templateSetup,
	name string,
	grade int,
) createTemplateResponse {
	t.Helper()
	body := createTemplateBody(s, name)
	body["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
	body["target_grade_level"] = grade
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[createTemplateResponse](t, w)
}

func templateSchedules(t *testing.T, s *templateSetup, templateID int64) []*activitiesModel.Schedule {
	t.Helper()
	rows, err := activitiesRepo.NewScheduleRepository(s.db).FindByGroupID(s.ctx, templateID)
	require.NoError(t, err)
	return rows
}

func unusedTemplateClockWindow(t *testing.T, s *templateSetup, seed int64) (string, string) {
	t.Helper()
	for attempt := 0; attempt < 1200; attempt++ {
		startMinute := 1 + (int(seed%1200)+attempt*17)%1200
		endMinute := startMinute + 5
		startRaw := fmt.Sprintf("%02d:%02d", startMinute/60, startMinute%60)
		endRaw := fmt.Sprintf("%02d:%02d", endMinute/60, endMinute%60)
		start, err := parseClockTime(startRaw)
		require.NoError(t, err)
		end, err := parseClockTime(endRaw)
		require.NoError(t, err)
		rows, err := s.res.TimetableData.GetTimeframesByTimeRange(s.ctx, start, end)
		require.NoError(t, err)
		exactMatch := false
		for _, row := range rows {
			if row != nil && row.EndTime != nil &&
				timezone.SameClockTime(row.StartTime, start) &&
				timezone.SameClockTime(*row.EndTime, end) {
				exactMatch = true
				break
			}
		}
		if !exactMatch {
			return startRaw, endRaw
		}
	}
	require.FailNow(t, "could not find an unused timeframe window")
	return "", ""
}

func splitBody(s *templateSetup, name string, effective timezone.Date) map[string]any {
	body := createTemplateBody(s, name)
	body["effective_date"] = effective.String()
	delete(body, "materialize_from")
	delete(body, "materialize_to")
	return body
}

func endBody(effective timezone.Date) map[string]any {
	return map[string]any{"effective_date": effective.String()}
}

func TestTemplateSplitHandler_HappyPath(t *testing.T) {
	mat := &mockMaterializationService{
		result: &scheduleSvc.MaterializationResult{InstancesCreated: 5},
	}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, "Tpl-Split-Quelle")

	effective := timezone.TodayDate().AddDays(7)
	body := splitBody(s, "Tpl-Split-Nachfolger", effective)
	body["materialize_from"] = effective.String()
	body["materialize_to"] = effective.AddDays(6).String()

	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	resp := decodeTemplateData[splitTemplateResponse](t, w)
	assert.Equal(t, created.TemplateID, resp.OldTemplateID)
	require.NotZero(t, resp.NewTemplateID)
	assert.NotEqual(t, resp.OldTemplateID, resp.NewTemplateID)
	assert.Len(t, resp.ScheduleIDs, 2, "one schedule per requested weekday")
	assert.Equal(t, 0, resp.DeletedInstances, "no planned instances existed")
	assert.Equal(t, 5, resp.InstancesCreated, "materialization count surfaces on the wire")
	assert.Equal(t, scheduleSvc.MaterializationSourceManual, mat.source)
}

// TestTemplateSplitHandler_RejectsOverlongNotes guards the #1837 follow-up:
// the split note (nullableStr) shadows the embedded updateTemplateRequest.Notes,
// so the split Bind must enforce the same 2000-char limit as create/update.
func TestTemplateSplitHandler_RejectsOverlongNotes(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, "Tpl-Split-LongNote-Quelle")

	effective := timezone.TodayDate().AddDays(7)
	body := splitBody(s, "Tpl-Split-LongNote", effective)
	body["notes"] = strings.Repeat("x", 2001)

	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"split must reject notes over 2000 chars like create/update; body=%s", w.Body.String())
}

func TestTemplateUpdateHandler_EnforcesTenantGradeLevelMax(t *testing.T) {
	s := buildTemplateSetup(t, nil)
	defer s.cleanupFn()
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	s.res.SettingsService = templateGradeSettings(13, nil)
	created := createJahrgangSourceTemplate(t, router, s, "Tpl-GradeCap-Update-Source", 5)

	// Lowering the tenant cap must not strand an existing legacy series.
	s.res.SettingsService = templateGradeSettings(4, nil)
	unchanged := createTemplateBody(s, "Tpl-GradeCap-Update-Unchanged")
	unchanged["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
	unchanged["target_grade_level"] = 5
	unchangedW := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", created.TemplateID), unchanged)
	require.Equal(t, http.StatusOK, unchangedW.Code, "body=%s", unchangedW.Body.String())

	rejectedName := fmt.Sprintf("Tpl-GradeCap-Update-Rejected-%d", time.Now().UnixNano())
	startTime, endTime := unusedTemplateClockWindow(t, s, created.TemplateID)
	rejected := createTemplateBody(s, rejectedName)
	rejected["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
	rejected["target_grade_level"] = 6
	rejected["start_time"] = startTime
	rejected["end_time"] = endTime
	rejectedW := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", created.TemplateID), rejected)
	require.Equal(t, http.StatusBadRequest, rejectedW.Code, "body=%s", rejectedW.Body.String())
	require.Contains(t, rejectedW.Body.String(), "target_grade_level 6 exceeds tenant maximum 4")

	group, err := s.res.TimetableData.GetActivityGroup(s.ctx, created.TemplateID)
	require.NoError(t, err)
	assert.Equal(t, "Tpl-GradeCap-Update-Unchanged", group.Name)
	require.NotNil(t, group.TargetGradeLevel)
	assert.EqualValues(t, 5, *group.TargetGradeLevel)
	timeframes, err := scheduleRepo.NewTimeframeRepository(s.db).FindByDescription(s.ctx, rejectedName)
	require.NoError(t, err)
	assert.Empty(t, timeframes, "the handler-created timeframe must roll back with the rejected update")

	s.res.SettingsService = templateGradeSettings(0, errors.New("settings unavailable"))
	settingsFailure := createTemplateBody(s, "Tpl-GradeCap-Update-SettingsFailure")
	settingsFailureW := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", created.TemplateID), settingsFailure)
	require.Equal(t, http.StatusInternalServerError, settingsFailureW.Code, "body=%s", settingsFailureW.Body.String())

	group, err = s.res.TimetableData.GetActivityGroup(s.ctx, created.TemplateID)
	require.NoError(t, err)
	assert.Equal(t, "Tpl-GradeCap-Update-Unchanged", group.Name)
}

func TestTemplateSplitHandler_EnforcesTenantGradeLevelMax(t *testing.T) {
	t.Run("allows an unchanged legacy above-cap Jahrgang", func(t *testing.T) {
		s := buildTemplateSetup(t, nil)
		defer s.cleanupFn()
		attachSplitService(s, &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}})
		router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

		s.res.SettingsService = templateGradeSettings(13, nil)
		created := createJahrgangSourceTemplate(t, router, s, "Tpl-GradeCap-Split-Legacy", 5)
		s.res.SettingsService = templateGradeSettings(4, nil)
		body := splitBody(s, "Tpl-GradeCap-Split-Legacy-Successor", timezone.TodayDate().AddDays(7))
		body["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
		body["target_grade_level"] = 5

		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		result := decodeTemplateData[splitTemplateResponse](t, w)
		successor, err := s.res.TimetableData.GetActivityGroup(s.ctx, result.NewTemplateID)
		require.NoError(t, err)
		require.NotNil(t, successor.TargetGradeLevel)
		assert.EqualValues(t, 5, *successor.TargetGradeLevel)
	})

	t.Run("rejects a changed above-cap Jahrgang before capping the source", func(t *testing.T) {
		s := buildTemplateSetup(t, nil)
		defer s.cleanupFn()
		attachSplitService(s, &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}})
		router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

		s.res.SettingsService = templateGradeSettings(13, nil)
		created := createJahrgangSourceTemplate(t, router, s, "Tpl-GradeCap-Split-Rejected", 5)
		s.res.SettingsService = templateGradeSettings(4, nil)
		body := splitBody(s, "Tpl-GradeCap-Split-Rejected-Successor", timezone.TodayDate().AddDays(7))
		body["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
		body["target_grade_level"] = 6

		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
		require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		for _, row := range templateSchedules(t, s, created.TemplateID) {
			assert.Nil(t, row.ValidUntil)
		}
		series, err := activitiesRepo.NewGroupRepository(s.db).FindTemplateSeries(s.ctx, created.TemplateID)
		require.NoError(t, err)
		assert.Len(t, series, 1)
	})

	t.Run("settings failure returns 500 without mutating the source", func(t *testing.T) {
		s := buildTemplateSetup(t, nil)
		defer s.cleanupFn()
		attachSplitService(s, &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}})
		router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

		s.res.SettingsService = templateGradeSettings(13, nil)
		created := createJahrgangSourceTemplate(t, router, s, "Tpl-GradeCap-Split-Settings", 5)
		s.res.SettingsService = templateGradeSettings(0, errors.New("settings unavailable"))
		body := splitBody(s, "Tpl-GradeCap-Split-Settings-Successor", timezone.TodayDate().AddDays(7))
		body["target_group_type"] = activitiesModel.TargetGroupTypeJahrgang
		body["target_grade_level"] = 5

		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
		require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
		for _, row := range templateSchedules(t, s, created.TemplateID) {
			assert.Nil(t, row.ValidUntil)
		}
		series, err := activitiesRepo.NewGroupRepository(s.db).FindTemplateSeries(s.ctx, created.TemplateID)
		require.NoError(t, err)
		assert.Len(t, series, 1)
	})
}

func TestTemplateSplitHandler_IncompatibleCareLinkRollsBackOn400(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitServiceWithValidator(s, mat, func(context.Context, int64) error {
		return fmt.Errorf("%w: linked care offering would become invalid", enrollmentModel.ErrCareOfferingInvalid)
	})
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})
	created := createSourceTemplate(t, router, s, "Tpl-Split-Rollback-Quelle")

	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", created.TemplateID),
		splitBody(s, "Tpl-Split-Rollback-Nachfolger", timezone.TodayDate().AddDays(7)))
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"code":"timetable.template_care_offering_conflict"`)
	assert.Contains(t, w.Body.String(), "Die Änderung ist nicht möglich")
	assert.NotContains(t, w.Body.String(), "linked care offering")

	schedules := templateSchedules(t, s, created.TemplateID)
	require.NotEmpty(t, schedules)
	for _, schedule := range schedules {
		assert.Nil(t, schedule.ValidUntil,
			"the ambient TenantTxMiddleware must roll back the provisional source cap")
	}
	series, err := activitiesRepo.NewGroupRepository(s.db).FindTemplateSeries(s.ctx, created.TemplateID)
	require.NoError(t, err)
	require.Len(t, series, 1, "the rejected successor must not commit on a 400 response")
	assert.Equal(t, created.TemplateID, series[0].ID)
}

func TestRenderTemplateSplitError_RosterRebaseConflictUsesStable409(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/templates/42/split", nil)
	recorder := httptest.NewRecorder()

	renderTemplateSplitError(
		recorder,
		req,
		fmt.Errorf("split failed: %w", scheduleSvc.ErrTemplateRosterRebaseConflict),
	)

	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"timetable.template_roster_rebase_conflict"`)
	assert.Contains(t, recorder.Body.String(), "geschützte Kinderzuordnungen")
}

func TestTemplateSplitHandler_CareValidatorInfrastructureFailureReturnsGeneric500(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitServiceWithValidator(s, mat, func(context.Context, int64) error {
		return errors.New("synthetic database details")
	})
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})
	created := createSourceTemplate(t, router, s, "Tpl-Split-Infra-Quelle")

	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", created.TemplateID),
		splitBody(s, "Tpl-Split-Infra-Nachfolger", timezone.TodayDate().AddDays(7)))
	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
	assert.NotContains(t, w.Body.String(), "synthetic database details")

	schedules := templateSchedules(t, s, created.TemplateID)
	require.NotEmpty(t, schedules)
	for _, schedule := range schedules {
		assert.Nil(t, schedule.ValidUntil)
	}
	series, err := activitiesRepo.NewGroupRepository(s.db).FindTemplateSeries(s.ctx, created.TemplateID)
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, created.TemplateID, series[0].ID)
}

func TestTemplateUpdateHandler_IncompatibleCareLinkRollsBackOn400(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})
	created := createSourceTemplate(t, router, s, "Tpl-Update-Rollback-Quelle")

	beforeGroup, err := s.res.TimetableData.GetActivityGroup(s.ctx, created.TemplateID)
	require.NoError(t, err)
	beforeSchedules := templateSchedules(t, s, created.TemplateID)
	beforeEnrollments, err := activitiesRepo.NewStudentEnrollmentRepository(s.db).FindByGroupID(s.ctx, created.TemplateID)
	require.NoError(t, err)
	beforeSupervisors, err := activitiesRepo.NewSupervisorPlannedRepository(s.db).FindByGroupID(s.ctx, created.TemplateID)
	require.NoError(t, err)

	updateName := fmt.Sprintf("Tpl-Update-Rollback-%d", time.Now().UnixNano())
	startTime, endTime := unusedTemplateClockWindow(t, s, created.TemplateID)
	timeframeRepo := scheduleRepo.NewTimeframeRepository(s.db)
	existingTimeframes, err := timeframeRepo.FindByDescription(s.ctx, updateName)
	require.NoError(t, err)
	require.Empty(t, existingTimeframes)

	validatorReached := false
	s.res.TimetableData = testTimetableDataWithCareValidator(s.db, func(ctx context.Context, templateID int64) error {
		provisionalTimeframes, lookupErr := timeframeRepo.FindByDescription(ctx, updateName)
		if lookupErr != nil {
			return lookupErr
		}
		if len(provisionalTimeframes) != 1 {
			return fmt.Errorf("expected one provisional timeframe, got %d", len(provisionalTimeframes))
		}
		provisionalGroup, lookupErr := activitiesRepo.NewGroupRepository(s.db).FindByID(ctx, templateID)
		if lookupErr != nil {
			return lookupErr
		}
		if provisionalGroup.Name != updateName {
			return fmt.Errorf("expected provisional group name %q, got %q", updateName, provisionalGroup.Name)
		}
		validatorReached = true
		return fmt.Errorf("%w: linked care offering would become invalid", enrollmentModel.ErrCareOfferingInvalid)
	})

	updateBody := createTemplateBody(s, updateName)
	updateBody["weekdays"] = []int{activitiesModel.WeekdayFriday}
	updateBody["start_time"] = startTime
	updateBody["end_time"] = endTime
	updateBody["student_ids"] = []int64{s.studentB}
	updateBody["staff_ids"] = []int64{s.staffA}
	updateBody["primary_staff_id"] = s.staffA
	updateW := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusBadRequest, updateW.Code, "body=%s", updateW.Body.String())
	assert.Contains(t, updateW.Body.String(), `"code":"timetable.template_care_offering_conflict"`)
	assert.True(t, validatorReached, "the test must fail after the provisional writes, not during preflight")

	afterTimeframes, err := timeframeRepo.FindByDescription(s.ctx, updateName)
	require.NoError(t, err)
	assert.Empty(t, afterTimeframes, "the unique timeframe created before validation must roll back")
	afterGroup, err := s.res.TimetableData.GetActivityGroup(s.ctx, created.TemplateID)
	require.NoError(t, err)
	assert.Equal(t, beforeGroup.Name, afterGroup.Name)
	assert.Equal(t, beforeGroup.Type, afterGroup.Type)

	afterSchedules := templateSchedules(t, s, created.TemplateID)
	require.Len(t, afterSchedules, len(beforeSchedules))
	for i := range beforeSchedules {
		assert.Equal(t, beforeSchedules[i].ID, afterSchedules[i].ID)
		assert.Equal(t, beforeSchedules[i].Weekday, afterSchedules[i].Weekday)
		assert.Equal(t, beforeSchedules[i].TimeframeID, afterSchedules[i].TimeframeID)
	}
	afterEnrollments, err := activitiesRepo.NewStudentEnrollmentRepository(s.db).FindByGroupID(s.ctx, created.TemplateID)
	require.NoError(t, err)
	assert.ElementsMatch(t, enrollmentIDs(beforeEnrollments), enrollmentIDs(afterEnrollments))
	afterSupervisors, err := activitiesRepo.NewSupervisorPlannedRepository(s.db).FindByGroupID(s.ctx, created.TemplateID)
	require.NoError(t, err)
	assert.ElementsMatch(t, supervisorIDs(beforeSupervisors), supervisorIDs(afterSupervisors))
}

func enrollmentIDs(rows []*activitiesModel.StudentEnrollment) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

func supervisorIDs(rows []*activitiesModel.SupervisorPlanned) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

func TestTemplateSplitHandler_UpdateSuccessorPreservesValidFrom(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, "Tpl-Split-Update-Quelle")
	effective := timezone.TodayDate().AddDays(7)
	splitW := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", created.TemplateID),
		splitBody(s, "Tpl-Split-Update-Nachfolger", effective))
	require.Equal(t, http.StatusOK, splitW.Code, "body=%s", splitW.Body.String())
	split := decodeTemplateData[splitTemplateResponse](t, splitW)

	updateBody := createTemplateBody(s, "Tpl-Split-Update-Nachfolger-Bearbeitet")
	updateBody["weekdays"] = []int{activitiesModel.WeekdayTuesday, activitiesModel.WeekdayThursday}
	updateW := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", split.NewTemplateID), updateBody)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())

	schedules := templateSchedules(t, s, split.NewTemplateID)
	require.Len(t, schedules, 2)
	for _, schedule := range schedules {
		require.NotNil(t, schedule.ValidFrom, "editing the successor must not erase its split start")
		assert.Equal(t, effective, *schedule.ValidFrom)
		assert.Nil(t, schedule.ValidUntil)
	}

	enrollments, err := activitiesRepo.NewStudentEnrollmentRepository(s.db).FindByGroupID(s.ctx, split.NewTemplateID)
	require.NoError(t, err)
	activeEnrollments := 0
	for _, enrollment := range enrollments {
		if enrollment.ValidUntil == nil {
			activeEnrollments++
			assert.Equal(t, effective, enrollment.ValidFrom,
				"successor roster must inherit the split start, not the period start")
			continue
		}
		assert.False(t, enrollment.ValidUntil.Before(enrollment.ValidFrom),
			"retired successor roster bounds must not invert")
	}
	assert.Equal(t, 2, activeEnrollments)

	supervisors, err := activitiesRepo.NewSupervisorPlannedRepository(s.db).FindByGroupID(s.ctx, split.NewTemplateID)
	require.NoError(t, err)
	activeSupervisors := 0
	for _, supervisor := range supervisors {
		if supervisor.ValidUntil == nil {
			activeSupervisors++
			assert.Equal(t, effective, supervisor.ValidFrom,
				"successor supervision must inherit the split start")
			continue
		}
		assert.False(t, supervisor.ValidUntil.Before(supervisor.ValidFrom),
			"retired successor supervision bounds must not invert")
	}
	assert.Equal(t, 2, activeSupervisors)
}

func TestTemplateUpdateHandler_RejectsInconsistentValidityEnvelopeWithoutMutation(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, "Tpl-Update-Inconsistent-Quelle")
	before := templateSchedules(t, s, created.TemplateID)
	require.Len(t, before, 2)
	inconsistentFrom := timezone.TodayDate().AddDays(14)
	_, err := s.db.NewUpdate().
		Model((*activitiesModel.Schedule)(nil)).
		ModelTableExpr(`activities.schedules AS "schedule"`).
		Set("valid_from = ?", inconsistentFrom).
		Where(`"schedule".id = ?`, before[0].ID).
		Where(`"schedule".tenant_id = ?`, tenant.FromContext(s.ctx)).
		Exec(s.ctx)
	require.NoError(t, err)

	updateBody := createTemplateBody(s, "Tpl-Update-Inconsistent-Must-Rollback")
	updateW := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	require.Equal(t, http.StatusInternalServerError, updateW.Code, "body=%s", updateW.Body.String())

	after := templateSchedules(t, s, created.TemplateID)
	require.Len(t, after, 2, "inconsistent schedules must not be deleted")
	assert.Equal(t, []int64{before[0].ID, before[1].ID}, []int64{after[0].ID, after[1].ID})
	require.NotNil(t, after[0].ValidFrom)
	assert.Equal(t, inconsistentFrom, *after[0].ValidFrom)
	assert.Nil(t, after[1].ValidFrom)

	group, err := s.res.TimetableData.GetActivityGroup(s.ctx, created.TemplateID)
	require.NoError(t, err)
	assert.Equal(t, "Tpl-Update-Inconsistent-Quelle", group.Name,
		"validity validation must run before template fields change")
}

func TestTemplateSplitHandler_BadEffectiveDate(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, "Tpl-Split-DatumQuelle")

	t.Run("malformed date", func(t *testing.T) {
		body := splitBody(s, "Tpl-Split-Datum", timezone.TodayDate())
		body["effective_date"] = "12.07.2026"
		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	})

	t.Run("missing date", func(t *testing.T) {
		body := splitBody(s, "Tpl-Split-Datum", timezone.TodayDate())
		delete(body, "effective_date")
		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	})

	t.Run("past date", func(t *testing.T) {
		body := splitBody(s, "Tpl-Split-Datum", timezone.TodayDate().AddDays(-1))
		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	})
}

func TestTemplateSplitHandler_UnknownTemplate(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	body := splitBody(s, "Tpl-Split-Unbekannt", timezone.TodayDate().AddDays(7))
	w := doTemplateJSON(t, router, http.MethodPost, "/templates/999999999/split", body)
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestTemplateSplitHandler_ForbiddenForReadOnly(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)

	adminRouter := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})
	created := createSourceTemplate(t, adminRouter, s, "Tpl-Split-NurLesen")

	readOnlyRouter := splitRouter(s.ctx, s.res, []string{permissions.SchedulesRead})
	body := splitBody(s, "Tpl-Split-NurLesenNeu", timezone.TodayDate().AddDays(7))
	w := doTemplateJSON(t, readOnlyRouter, http.MethodPost,
		fmt.Sprintf("/templates/%d/split", created.TemplateID), body)
	assert.Equal(t, http.StatusForbidden, w.Code, "body=%s", w.Body.String())
}

func TestTemplateEndHandler_HappyPath(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, "Tpl-End-Quelle")
	effective := timezone.TodayDate().AddDays(7)

	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/end", created.TemplateID), endBody(effective))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	resp := decodeTemplateData[endTemplateResponse](t, w)
	assert.Equal(t, created.TemplateID, resp.TemplateID)
	assert.Equal(t, effective.String(), resp.EffectiveDate)
	assert.Equal(t, 0, resp.DeletedInstances)
}

func TestTemplateEndHandler_RemovesTemplateFromActiveCRUD(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, "Tpl-End-Hidden")
	effective := timezone.TodayDate().AddDays(7)

	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/templates/%d/end", created.TemplateID), endBody(effective))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	listW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
	require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
	list := decodeTemplateData[listTemplatesResponse](t, listW)
	for _, tpl := range list.Templates {
		assert.NotEqual(t, created.TemplateID, tpl.ID, "ended template must not remain in the active template list")
	}

	period := createTemplateTestPeriod(t, s.db, "Tpl-End-Hidden-Read")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID) })
	getW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil)
	assert.Equal(t, http.StatusNotFound, getW.Code, "body=%s", getW.Body.String())

	updateBody := createTemplateBody(s, "Tpl-End-Hidden-Resurrected")
	putW := doTemplateJSON(t, router, http.MethodPut,
		fmt.Sprintf("/templates/%d", created.TemplateID), updateBody)
	assert.Equal(t, http.StatusNotFound, putW.Code, "body=%s", putW.Body.String())
}

func TestTemplateEndHandler_BadEffectiveDate(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	created := createSourceTemplate(t, router, s, "Tpl-End-DatumQuelle")

	t.Run("malformed date", func(t *testing.T) {
		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/end", created.TemplateID),
			map[string]any{"effective_date": "12.07.2026"})
		assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	})

	t.Run("missing date", func(t *testing.T) {
		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/end", created.TemplateID), map[string]any{})
		assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	})

	t.Run("past date", func(t *testing.T) {
		w := doTemplateJSON(t, router, http.MethodPost,
			fmt.Sprintf("/templates/%d/end", created.TemplateID),
			endBody(timezone.TodayDate().AddDays(-1)))
		assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	})
}

func TestTemplateEndHandler_UnknownTemplate(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)
	router := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})

	w := doTemplateJSON(t, router, http.MethodPost, "/templates/999999999/end",
		endBody(timezone.TodayDate().AddDays(7)))
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestTemplateEndHandler_ForbiddenForReadOnly(t *testing.T) {
	mat := &mockMaterializationService{result: &scheduleSvc.MaterializationResult{}}
	s := buildTemplateSetup(t, mat)
	defer s.cleanupFn()
	attachSplitService(s, mat)

	adminRouter := splitRouter(s.ctx, s.res, []string{permissions.SchedulesManage})
	created := createSourceTemplate(t, adminRouter, s, "Tpl-End-NurLesen")

	readOnlyRouter := splitRouter(s.ctx, s.res, []string{permissions.SchedulesRead})
	w := doTemplateJSON(t, readOnlyRouter, http.MethodPost,
		fmt.Sprintf("/templates/%d/end", created.TemplateID),
		endBody(timezone.TodayDate().AddDays(7)))
	assert.Equal(t, http.StatusForbidden, w.Code, "body=%s", w.Body.String())
}
