package timetablehttp

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListTemplatesQueryBudget keeps the period-scoped roster-maintenance
// lookup batched: the list must not add a query for each returned Regeltermin.
func TestListTemplatesQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	sources, err := RootCompositions.OfferingSources(t, s.db)
	require.NoError(t, err)
	s.res.OfferingSourceOptions = sources
	counter := testpkg.CaptureQueries(t, s.db)
	router := testpkg.MeasuredTenantRequest(t, s.db, testpkg.TenantIDFromContext(s.ctx), counter, templateQueryBudgetRouter(t, s.ctx, s.res))
	period := createTemplateTestPeriod(t, s.db, "Tpl-Query-Budget-Period")

	created := 0
	addTemplates := func(n int) {
		for range n {
			body := createTemplateBody(s, fmt.Sprintf("Budget-Regeltermin-%d", created))
			body["calendar_period_id"] = period.ID
			w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
			require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
			created++
		}
	}

	run := func() int {
		counter.Reset()
		w := doTemplateJSON(t, router, http.MethodGet, fmt.Sprintf("/templates?period_id=%d", period.ID), nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		templates := decodeTemplateData[listTemplatesResponse](t, w).Templates
		require.Len(t, templates, created)
		for _, template := range templates {
			require.NotNil(t, template.RosterMaintenance,
				"period-scoped list must include the roster-maintenance indicator")
		}
		return counter.Total()
	}

	addTemplates(3)
	smallCount := run()

	addTemplates(5)
	largeCount := run()

	t.Logf("query budget: 3 templates → %d queries, 8 templates → %d queries", smallCount, largeCount)
	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of the number of templates (no per-template N+1)")
	querybudgets.AssertQueryBudget(t, "api.timetable.templates.list", counter.Queries())
}

// TestListTargetedTemplatesQueryBudget measures the populated display target
// path independently of the empty-target list above.
func TestListTargetedTemplatesQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	sources, err := RootCompositions.OfferingSources(t, s.db)
	require.NoError(t, err)
	s.res.OfferingSourceOptions = sources
	counter := testpkg.CaptureQueries(t, s.db)
	router := testpkg.MeasuredTenantRequest(t, s.db, testpkg.TenantIDFromContext(s.ctx), counter, templateQueryBudgetRouter(t, s.ctx, s.res))
	period := createTemplateTestPeriod(t, s.db, "Targeted-Query-Budget-Period")
	created := 0
	add := func(n int) {
		for range n {
			body := createTemplateBody(s, fmt.Sprintf("Targeted-Budget-%d", created))
			body["calendar_period_id"] = period.ID
			body["student_ids"] = []int64{}
			body["target_group_type"] = "klasse"
			body["targets"] = []map[string]any{{"type": "klasse", "school_class": "3a"}}
			w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
			require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
			created++
		}
	}
	run := func() int {
		counter.Reset()
		w := doTemplateJSON(t, router, http.MethodGet, fmt.Sprintf("/templates?period_id=%d", period.ID), nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		rows := decodeTemplateData[listTemplatesResponse](t, w).Templates
		require.Len(t, rows, created)
		for _, row := range rows {
			assert.Equal(t, 2, row.EnrollmentCount)
			require.Len(t, row.Targets, 1)
		}
		return counter.Total()
	}
	add(3)
	small := run()
	add(5)
	large := run()
	t.Logf("query budget: 3 targeted templates → %d queries, 8 targeted templates → %d queries", small, large)
	assert.Equal(t, small, large)
	querybudgets.AssertQueryBudget(t, "api.timetable.templates.list.targeted", counter.Queries())
}

func templateQueryBudgetRouter(t *testing.T, parentCtx context.Context, resource *Resource) chi.Router {
	t.Helper()
	tenantID := tenant.FromContext(parentCtx)
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			ctx := testpkg.WithTestTenantRuntime(t, request.Context())
			ctx = tenant.WithTenantID(ctx, tenantID)
			next.ServeHTTP(w, request.WithContext(ctx))
		})
	})
	router.Get("/templates", resource.listTemplates)
	router.Post("/templates", resource.createTemplate)
	return router
}
