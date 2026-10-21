package compose

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// delegateHandler Tests
// =============================================================================

func TestDelegateHandler_ForwardsRequest(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
// Router Tests
// =============================================================================

func TestResourceRouterFailsFastWithoutFeedbackGraph(t *testing.T) {
	t.Parallel()
	assert.PanicsWithValue(t, "IoT feedback: all dependencies are required", func() {
		(&Resource{}).Router()
	})
}
