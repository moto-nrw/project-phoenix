package common_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chi returns the route context to its pool once the handler returns, so a
// runtime event raised by a goroutine outliving the request must reuse the
// route captured when the response was flushed (#2953).
func TestTenantRuntimeObserverKeepsRouteForLateEvents(t *testing.T) {
	t.Parallel()
	var observed []common.TenantRuntimeObservation
	var lateCtx context.Context
	router := chi.NewRouter()
	router.Use(common.TenantRuntimeObserverMiddleware(func(observation common.TenantRuntimeObservation) {
		observed = append(observed, observation)
	}))
	router.Post("/api/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		lateCtx = r.Context()
		w.WriteHeader(http.StatusAccepted)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/jobs/7", nil))
	require.Empty(t, observed)
	// Simulate the pool handing the route context to another request.
	chi.RouteContext(lateCtx).Reset()

	tenant.ObserveMissingTenant(lateCtx, tenant.ErrTenantRequired)

	require.Len(t, observed, 1)
	assert.Equal(t, http.StatusAccepted, observed[0].Status)
	assert.Equal(t, "/api/jobs/{id}", observed[0].Route)
	assert.Equal(t, common.TenantRuntimeMissingTenant, observed[0].Event.Kind)
}
