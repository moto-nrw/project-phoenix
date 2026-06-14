// Handler tests for POST /templates/{id}/split (WP-B3).
//
// Hermetic: real repos + real split service against the test DB (fixtures via
// buildTemplateSetup); materialization mocked so no calendar period is
// required. The router mirrors the production wiring: tenant context +
// permission middleware, so the 403 path exercises the real gate.
package timetable

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// attachSplitService wires a real TemplateSplitService (real repos, mocked
// materialization) into the test resource. Same package, so the unexported
// field is assignable.
func attachSplitService(s *templateSetup, mat scheduleSvc.MaterializationService) {
	s.res.templateSplitService = scheduleSvc.NewTemplateSplitService(scheduleSvc.TemplateSplitDependencies{
		GroupRepo:       activitiesRepo.NewGroupRepository(s.db),
		ScheduleRepo:    activitiesRepo.NewScheduleRepository(s.db),
		EnrollmentRepo:  activitiesRepo.NewStudentEnrollmentRepository(s.db),
		SupervisorRepo:  activitiesRepo.NewSupervisorPlannedRepository(s.db),
		InstanceRepo:    scheduleRepo.NewActivityInstanceRepository(s.db),
		TimeframeRepo:   scheduleRepo.NewTimeframeRepository(s.db),
		Materialization: mat,
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
	r.Post("/templates", res.createTemplate)
	r.With(authorize.RequiresPermission(permissions.SchedulesManage)).
		Post("/templates/{id}/split", res.splitTemplate)
	return r
}

// createSourceTemplate creates the template the split tests operate on.
func createSourceTemplate(t *testing.T, router chi.Router, s *templateSetup, name string) createTemplateResponse {
	t.Helper()
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", createTemplateBody(s, name))
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	return decodeTemplateData[createTemplateResponse](t, w)
}

func splitBody(s *templateSetup, name string, effective timezone.Date) map[string]any {
	body := createTemplateBody(s, name)
	body["effective_date"] = effective.String()
	delete(body, "materialize_from")
	delete(body, "materialize_to")
	return body
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
