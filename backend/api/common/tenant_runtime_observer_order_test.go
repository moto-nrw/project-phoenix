package common

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantRuntimeObserverSerializesConcurrentDelivery(t *testing.T) {
	t.Parallel()
	firstEvent := errors.New("first event")
	secondEvent := errors.New("second event")
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondReturned := make(chan struct{})
	secondDelivered := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(releaseFirst)
		w.WriteHeader(http.StatusOK)
		go func() {
			tenant.ObserveMissingTenant(r.Context(), firstEvent)
		}()
		select {
		case <-firstStarted:
		case <-time.After(time.Second):
			require.Fail(t, "first event was not delivered")
			return
		}
		go func() {
			tenant.ObserveMissingTenant(r.Context(), secondEvent)
			close(secondReturned)
		}()
		<-secondReturned
		select {
		case <-secondDelivered:
			t.Fatal("second event overtook the first event")
		default:
		}
	})
	wrapped := TenantRuntimeObserverMiddleware(func(observation TenantRuntimeObservation) {
		switch {
		case errors.Is(observation.Event.Err, firstEvent):
			close(firstStarted)
			<-releaseFirst
		case errors.Is(observation.Event.Err, secondEvent):
			close(secondDelivered)
		}
	})(handler)

	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/sse/events", nil))

	select {
	case <-secondDelivered:
	case <-time.After(time.Second):
		require.Fail(t, "second event was not delivered")
	}
}

func TestTenantRuntimeObserverDeliversEventsAfterFlush(t *testing.T) {
	t.Parallel()
	var observed []TenantRuntimeObservation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		tenant.ObserveMissingTenant(r.Context(), tenant.ErrTenantRequired)
		require.Len(t, observed, 1)
	})
	wrapped := TenantRuntimeObserverMiddleware(collectObservations(&observed))(handler)

	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/sse/events", nil))

	require.Len(t, observed, 1)
	assert.Equal(t, http.StatusOK, observed[0].Status)
}
