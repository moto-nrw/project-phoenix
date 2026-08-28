package timetable

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubInstanceSeriesConverter struct {
	result *scheduleSvc.ConvertInstanceToSeriesResult
	err    error
	input  *scheduleSvc.ConvertInstanceToSeriesInput
}

func (s *stubInstanceSeriesConverter) ConvertInstanceToSeries(
	_ context.Context,
	in scheduleSvc.ConvertInstanceToSeriesInput,
) (*scheduleSvc.ConvertInstanceToSeriesResult, error) {
	s.input = &in
	return s.result, s.err
}

func conversionRouter(parentCtx context.Context, res *Resource) chi.Router {
	return conversionRouterWithOpts(parentCtx, res, false)
}

// conversionRouterWithOpts mirrors production when withTenantTx is true: the
// ambient TenantTxMiddleware that convert-to-series runs under.
func conversionRouterWithOpts(parentCtx context.Context, res *Resource, withTenantTx bool) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	if withTenantTx {
		r.Use(tenant.TenantTxMiddleware(res.DB))
	}
	r.Post("/instances/{id}/convert-to-series", res.convertInstanceToSeries)
	r.Get("/instances", res.listInstances)
	r.Get("/templates", res.listTemplates)
	return r
}

func TestConvertInstanceToSeries_MapsRequestAndResponse(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	defer s.cleanupFn()
	period := createTemplateTestPeriod(t, s.db, "Tpl-Convert-Period")
	t.Cleanup(func() {
		_, err := s.db.NewDelete().Table("schedule.calendar_periods").Where("id = ?", period.ID).Exec(s.ctx)
		require.NoError(t, err)
	})

	converter := &stubInstanceSeriesConverter{result: &scheduleSvc.ConvertInstanceToSeriesResult{
		TemplateID:       81,
		TimeframeID:      91,
		ScheduleIDs:      []int64{101, 102},
		LinkedInstanceID: 71,
	}}
	s.res.InstanceSeriesConverter = converter
	router := conversionRouter(s.ctx, s.res)
	body := createTemplateBody(s, "Tpl-Convert-HTTP")
	body["calendar_period_id"] = period.ID
	body["start_date"] = "2026-05-04"
	body["instance_notes"] = "Tageshinweis"

	w := doTemplateJSON(t, router, http.MethodPost, "/instances/71/convert-to-series", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	got := decodeTemplateData[convertInstanceToSeriesResponse](t, w)
	assert.Equal(t, int64(81), got.TemplateID)
	assert.Equal(t, int64(71), got.LinkedInstanceID)
	require.NotNil(t, converter.input)
	assert.Equal(t, int64(71), converter.input.InstanceID)
	assert.Equal(t, "Tpl-Convert-HTTP", converter.input.Template.Name)
	assert.Equal(t, "2026-05-04", converter.input.Template.ScheduleValidFrom.String())
	require.NotNil(t, converter.input.InstanceNotes)
	assert.Equal(t, "Tageshinweis", *converter.input.InstanceNotes)
}

func TestConvertInstanceToSeries_RequiresStartDate(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	defer s.cleanupFn()
	converter := &stubInstanceSeriesConverter{}
	s.res.InstanceSeriesConverter = converter
	router := conversionRouter(s.ctx, s.res)
	body := createTemplateBody(s, "Tpl-Convert-MissingDate")

	w := doTemplateJSON(t, router, http.MethodPost, "/instances/71/convert-to-series", body)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "start_date")
	assert.Nil(t, converter.input)
}

func TestConvertInstanceToSeries_RejectsInvalidID(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	defer s.cleanupFn()
	s.res.InstanceSeriesConverter = &stubInstanceSeriesConverter{}
	router := conversionRouter(s.ctx, s.res)

	w := doTemplateJSON(t, router, http.MethodPost, "/instances/nope/convert-to-series", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, w.Code, fmt.Sprintf("body=%s", w.Body.String()))
}

func TestConvertInstanceToSeries_PreservesTemplateValidationErrorContract(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	defer s.cleanupFn()
	period := createTemplateTestPeriod(t, s.db, "Tpl-Convert-Errors-Period")
	body := createTemplateBody(s, "Tpl-Convert-Errors")
	body["calendar_period_id"] = period.ID
	body["start_date"] = "2026-05-04"

	tests := []struct {
		name        string
		err         error
		wantMessage string
	}{
		{name: "inactive calendar period", err: scheduleSvc.ErrInstanceOutsideActiveCalendarPeriod, wantMessage: "instance date must lie within an active calendar period"},
		{name: "archived category", err: scheduleSvc.ErrCategoryNotAssignable, wantMessage: "category is archived or unavailable"},
		{name: "archived planning track", err: scheduleSvc.ErrPlanningTrackArchived, wantMessage: "planning track is archived or unavailable"},
		{name: "education group", err: &scheduleSvc.TemplateEducationGroupError{Err: errors.New("education group is unavailable")}, wantMessage: "education group is unavailable"},
		{name: "grade limit", err: scheduleSvc.ErrTemplateTargetGradeExceedsLimit, wantMessage: "template target grade exceeds tenant limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.res.InstanceSeriesConverter = &stubInstanceSeriesConverter{err: tt.err}
			w := doTemplateJSON(t, conversionRouter(s.ctx, s.res), http.MethodPost,
				"/instances/71/convert-to-series", body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
			assert.Contains(t, w.Body.String(), tt.wantMessage)
		})
	}
}

func TestConvertInstanceToSeries_LinksExistingOccurrenceAndRejectsRetry(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	period := createTemplateTestPeriod(t, s.db, "Tpl-Convert-Atomic-Period")
	date := timezone.NewDate(2026, 8, 10) // Monday, matching createTemplateBody.
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title:         "Tpl-Convert-Atomic-Seed",
		StartHHMM:     "12:00",
		EndHHMM:       "12:50",
		IsSpontaneous: true,
	})

	router := conversionRouter(s.ctx, s.res)
	body := createTemplateBody(s, "Tpl-Convert-Atomic")
	body["calendar_period_id"] = period.ID
	body["start_date"] = date.String()
	body["weekdays"] = []int{activitiesModel.WeekdayMonday}

	createdW := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/instances/%d/convert-to-series", instance.ID), body)
	require.Equal(t, http.StatusCreated, createdW.Code, "body=%s", createdW.Body.String())
	created := decodeTemplateData[convertInstanceToSeriesResponse](t, createdW)
	assert.Equal(t, instance.ID, created.LinkedInstanceID)
	assert.NotZero(t, created.TemplateID)

	listW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/instances?from=%s&to=%s", date, date), nil)
	require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
	listed := decodeTemplateData[weeklyInstancesResponse](t, listW)
	require.Len(t, listed.Instances, 1, "conversion must reuse the occurrence instead of adding a parallel row")
	assert.Equal(t, instance.ID, listed.Instances[0].ID)
	require.NotNil(t, listed.Instances[0].ActivityGroupID)
	assert.Equal(t, created.TemplateID, *listed.Instances[0].ActivityGroupID)
	assert.ElementsMatch(t, []int64{s.studentA, s.studentB}, listed.Instances[0].StudentIDs)

	// Materializer marker: seed must carry the template period so resync and
	// FindPlannedTemplateBackedFrom treat it as a series occurrence.
	seed, err := repositories.NewFactory(s.db).ActivityInstance.FindByID(s.ctx, instance.ID)
	require.NoError(t, err)
	require.NotNil(t, seed.CalendarPeriodID)
	assert.Equal(t, period.ID, *seed.CalendarPeriodID)
	assert.False(t, seed.IsSpontaneous)

	retryW := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/instances/%d/convert-to-series", instance.ID), body)
	assert.Equal(t, http.StatusConflict, retryW.Code, "body=%s", retryW.Body.String())

	templatesW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
	require.Equal(t, http.StatusOK, templatesW.Code, "body=%s", templatesW.Body.String())
	templates := decodeTemplateData[listTemplatesResponse](t, templatesW)
	count := 0
	for _, candidate := range templates.Templates {
		if candidate.Name == "Tpl-Convert-Atomic" {
			count++
		}
	}
	assert.Equal(t, 1, count, "retry must not create another series")
}

func TestConvertInstanceToSeries_UsesOfferingRosterForExistingSeed(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	period := createTemplateTestPeriod(t, s.db, "Tpl-Convert-Source-Period")
	date := timezone.NewDate(2026, 8, 10) // Monday.
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title:         "Tpl-Convert-Source-Seed",
		StartHHMM:     "12:00",
		EndHHMM:       "12:50",
		IsSpontaneous: true,
	})

	repoFactory := repositories.NewFactory(s.db)
	timetableData := testTimetableDataWithOfferingCallbacks(
		s.db,
		nil,
		func(context.Context, []int64, []int64, *int64) error { return nil },
		func(ctx context.Context, in scheduleSvc.OfferingRosterResyncInput) error {
			for _, enrollment := range []*activitiesModel.StudentEnrollment{
				{
					StudentID:        s.studentA,
					ActivityGroupID:  in.TemplateID,
					ValidFrom:        in.EffectiveFrom,
					CalendarPeriodID: in.CalendarPeriodID,
					SelectedWeekdays: []int{activitiesModel.WeekdayMonday},
				},
				{
					StudentID:        s.studentB,
					ActivityGroupID:  in.TemplateID,
					ValidFrom:        in.EffectiveFrom,
					CalendarPeriodID: in.CalendarPeriodID,
					SelectedWeekdays: []int{activitiesModel.WeekdayTuesday},
				},
			} {
				enrollment.SetTenantID(testpkg.Tenant(t))
				if err := repoFactory.StudentEnrollment.Create(ctx, enrollment); err != nil {
					return err
				}
			}
			return nil
		},
	)
	s.res.TimetableData = timetableData
	s.res.InstanceSeriesConverter = scheduleSvc.NewInstanceSeriesConversionService(
		scheduleSvc.InstanceSeriesConversionDependencies{
			DB:              s.db,
			InstanceRepo:    repoFactory.ActivityInstance,
			InstanceService: s.res.InstanceService,
			TimetableData:   timetableData,
		},
	)
	router := conversionRouter(s.ctx, s.res)
	body := createTemplateBody(s, "Tpl-Convert-Source")
	body["calendar_period_id"] = period.ID
	body["start_date"] = date.String()
	body["weekdays"] = []int{activitiesModel.WeekdayMonday}
	body["target_group_type"] = activitiesModel.TargetGroupTypeAngebot
	body["source_care_offering_ids"] = []int64{17}
	body["student_ids"] = []int64{}

	createdW := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/instances/%d/convert-to-series", instance.ID), body)
	require.Equal(t, http.StatusCreated, createdW.Code, "body=%s", createdW.Body.String())

	listW := doTemplateJSON(t, router, http.MethodGet,
		fmt.Sprintf("/instances?from=%s&to=%s", date, date), nil)
	require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
	listed := decodeTemplateData[weeklyInstancesResponse](t, listW)
	require.Len(t, listed.Instances, 1)
	assert.Equal(t, instance.ID, listed.Instances[0].ID)
	assert.Equal(t, []int64{s.studentA}, listed.Instances[0].StudentIDs,
		"the converted Monday seed must derive its children from the offering roster and booked weekday")
}

func TestConvertInstanceToSeries_RollsBackTemplateWhenLinkFails(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	period := createTemplateTestPeriod(t, s.db, "Tpl-Convert-Rollback-Period")
	date := timezone.NewDate(2026, 8, 10)
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title:         "Tpl-Convert-Rollback-Seed",
		StartHHMM:     "12:00",
		EndHHMM:       "12:50",
		IsSpontaneous: true,
	})

	failingInstanceService := &mockInstanceService{updateErr: errors.New("link failed")}
	repoFactory := repositories.NewFactory(s.db)
	s.res.InstanceSeriesConverter = scheduleSvc.NewInstanceSeriesConversionService(
		scheduleSvc.InstanceSeriesConversionDependencies{
			DB:              s.db,
			InstanceRepo:    repoFactory.ActivityInstance,
			InstanceService: failingInstanceService,
			TimetableData:   s.res.TimetableData,
		},
	)
	router := conversionRouter(s.ctx, s.res)
	body := createTemplateBody(s, "Tpl-Convert-Rollback")
	body["calendar_period_id"] = period.ID
	body["start_date"] = date.String()
	body["weekdays"] = []int{activitiesModel.WeekdayMonday}

	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/instances/%d/convert-to-series", instance.ID), body)
	assert.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())

	templatesW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
	require.Equal(t, http.StatusOK, templatesW.Code, "body=%s", templatesW.Body.String())
	templates := decodeTemplateData[listTemplatesResponse](t, templatesW)
	for _, candidate := range templates.Templates {
		assert.NotEqual(t, "Tpl-Convert-Rollback", candidate.Name, "failed link must roll back the new series")
	}
}

// A 4xx after CreateTemplate must not commit under TenantTxMiddleware: nested
// WithTenantTx reuses the outer tx, and middleware only auto-rolls back 5xx.
func TestConvertInstanceToSeries_RollsBackOrphanSeriesOn4xxLinkFailure(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	period := createTemplateTestPeriod(t, s.db, "Tpl-Convert-4xx-Period")
	// Existing seed is a weekday; conversion start_date moves it onto a
	// different weekend day → UpdatePlanned returns ErrInstanceWeekend (400)
	// after CreateTemplate has already written the series graph.
	seedDate := timezone.NewDate(2026, 8, 10)     // Monday
	weekendStart := timezone.NewDate(2026, 8, 15) // Saturday
	instance := testpkg.CreateTestActivityInstance(t, s.db, seedDate, s.roomID, testpkg.ActivityInstanceOpts{
		Title:         "Tpl-Convert-4xx-Seed",
		StartHHMM:     "12:00",
		EndHHMM:       "12:50",
		IsSpontaneous: true,
	})

	router := conversionRouterWithOpts(s.ctx, s.res, true)
	body := createTemplateBody(s, "Tpl-Convert-4xx-Orphan")
	body["calendar_period_id"] = period.ID
	body["start_date"] = weekendStart.String()
	body["weekdays"] = []int{activitiesModel.WeekdayMonday}

	w := doTemplateJSON(t, router, http.MethodPost,
		fmt.Sprintf("/instances/%d/convert-to-series", instance.ID), body)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "Monday to Friday")

	// Read outside the request transaction to see committed state.
	templatesW := doTemplateJSON(t, conversionRouter(s.ctx, s.res), http.MethodGet, "/templates", nil)
	require.Equal(t, http.StatusOK, templatesW.Code, "body=%s", templatesW.Body.String())
	templates := decodeTemplateData[listTemplatesResponse](t, templatesW)
	for _, candidate := range templates.Templates {
		assert.NotEqual(t, "Tpl-Convert-4xx-Orphan", candidate.Name,
			"TenantTxMiddleware must roll back the new series on a 4xx link failure")
	}

	seed, err := repositories.NewFactory(s.db).ActivityInstance.FindByID(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.Nil(t, seed.ActivityGroupID, "seed must stay unlinked when convert rolls back")
	assert.True(t, seed.IsSpontaneous)
	assert.Equal(t, seedDate, seed.Date)
}

func TestConvertInstanceToSeries_MarksRollbackOnServiceError(t *testing.T) {
	t.Parallel()

	s := buildTemplateSetup(t, &mockMaterializationService{})
	defer s.cleanupFn()
	period := createTemplateTestPeriod(t, s.db, "Tpl-Convert-MarkRollback-Period")
	t.Cleanup(func() {
		_, err := s.db.NewDelete().Table("schedule.calendar_periods").Where("id = ?", period.ID).Exec(s.ctx)
		require.NoError(t, err)
	})

	s.res.InstanceSeriesConverter = &stubInstanceSeriesConverter{err: scheduleSvc.ErrInstanceAlreadyInSeries}
	body := createTemplateBody(s, "Tpl-Convert-MarkRollback")
	body["calendar_period_id"] = period.ID
	body["start_date"] = "2026-05-04"

	// Inject a rollback marker without full TenantTxMiddleware so we can assert
	// the handler requested rollback on a mapped 4xx service error.
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.WithRollbackMarker(tenant.WithTenantID(r.Context(), tenant.FromContext(s.ctx)))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	var sawRollback bool
	router.Post("/instances/{id}/convert-to-series", func(w http.ResponseWriter, r *http.Request) {
		s.res.convertInstanceToSeries(w, r)
		sawRollback = tenant.RollbackRequested(r.Context())
	})

	w := doTemplateJSON(t, router, http.MethodPost, "/instances/71/convert-to-series", body)
	assert.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.True(t, sawRollback, "convert service errors must mark the ambient tenant tx for rollback")
}
