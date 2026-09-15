package timetable

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	apiTest "github.com/moto-nrw/project-phoenix/api/testutil"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListTemplatesQueryBudget keeps the roster-maintenance lookup batched:
// the list must not add a query for each returned Regeltermin.
func TestListTemplatesQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	repos := mustTimetableTestRepositories(s.db)
	_, settings := apiTest.SetupSettingsModule(t)
	lister, ok := enrollmentSvc.NewDecisionService(enrollmentSvc.DecisionServiceConfig{
		CareOfferingRepo:  repos.CareOffering,
		ActivityGroupRepo: repos.ActivityGroup,
		Settings:          settings.Settings,
	}).(enrollmentSvc.OfferingSourceOptionLister)
	require.True(t, ok)
	s.res.OfferingSourceOptions = lister
	router := templateQueryBudgetRouter(t, s.ctx, s.res)

	created := 0
	addTemplates := func(n int) {
		for range n {
			w := doTemplateJSON(t, router, http.MethodPost, "/templates", createTemplateBody(s, fmt.Sprintf("Budget-Regeltermin-%d", created)))
			require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, s.db)
	run := func() int {
		counter.Reset()
		w := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.Len(t, decodeTemplateData[listTemplatesResponse](t, w).Templates, created)
		return counter.Total()
	}

	addTemplates(3)
	smallCount := run()

	addTemplates(5)
	largeCount := run()

	t.Logf("query budget: 3 templates → %d queries, 8 templates → %d queries", smallCount, largeCount)
	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of the number of templates (no per-template N+1)")
	testpkg.AssertQueryBudget(t, "api.timetable.templates.list", counter.Queries())
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
