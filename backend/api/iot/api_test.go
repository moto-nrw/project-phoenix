package iot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// delegateHandler Tests
// =============================================================================

func TestDelegateHandler_ForwardsRequest(t *testing.T) {
	// Create a subrouter with a test endpoint
	subrouter := chi.NewRouter()
	subrouter.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("delegated response"))
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "delegated response", w.Body.String())
}

func TestDelegateHandler_PostRequest(t *testing.T) {
	subrouter := chi.NewRouter()
	subrouter.Post("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("post response"))
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("POST", "/data", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "post response", w.Body.String())
}

// =============================================================================
// NewResource Tests
// =============================================================================

func TestNewResource(t *testing.T) {
	deps := ServiceDependencies{
		IoTService:        nil,
		UsersService:      nil,
		ActiveService:     nil,
		ActivitiesService: nil,
		ConfigService:     nil,
		FacilityService:   nil,
		EducationService:  nil,
		FeedbackService:   nil,
	}

	resource := NewResource(deps)

	require.NotNil(t, resource)
	assert.Nil(t, resource.IoTService)
	assert.Nil(t, resource.UsersService)
	assert.Nil(t, resource.ActiveService)
	assert.Nil(t, resource.ActivitiesService)
	assert.Nil(t, resource.ConfigService)
	assert.Nil(t, resource.FacilityService)
	assert.Nil(t, resource.EducationService)
	assert.Nil(t, resource.FeedbackService)
}

// =============================================================================
// ServiceDependencies Tests
// =============================================================================

func TestServiceDependencies_Struct(t *testing.T) {
	// Verify struct fields exist
	deps := ServiceDependencies{}

	assert.Nil(t, deps.IoTService)
	assert.Nil(t, deps.UsersService)
	assert.Nil(t, deps.ActiveService)
	assert.Nil(t, deps.ActivitiesService)
	assert.Nil(t, deps.ConfigService)
	assert.Nil(t, deps.FacilityService)
	assert.Nil(t, deps.EducationService)
	assert.Nil(t, deps.FeedbackService)
}

// =============================================================================
// Resource Struct Tests
// =============================================================================

func TestResource_Struct(t *testing.T) {
	// Verify Resource struct can be instantiated
	resource := &Resource{}

	assert.Nil(t, resource.IoTService)
	assert.Nil(t, resource.UsersService)
	assert.Nil(t, resource.ActiveService)
	assert.Nil(t, resource.ActivitiesService)
	assert.Nil(t, resource.ConfigService)
	assert.Nil(t, resource.FacilityService)
	assert.Nil(t, resource.EducationService)
	assert.Nil(t, resource.FeedbackService)
}
