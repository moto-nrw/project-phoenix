package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestCalDAVExtensionMethodsReachHandlerUnchanged(t *testing.T) {
	t.Parallel()

	for method := range calDAVExtensionMethods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			var received string
			router := chi.NewRouter()
			router.Handle("/api/caldav/*", restoreCalDAVMethod(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received = r.Method
				w.WriteHeader(http.StatusNoContent)
			})))
			api := &API{Router: router}

			response := httptest.NewRecorder()
			api.ServeHTTP(response, httptest.NewRequest(method, "/api/caldav/principal/", nil))

			assert.Equal(t, http.StatusNoContent, response.Code)
			assert.Equal(t, method, received)
		})
	}
}

func TestCalDAVMethodNormalizationDoesNotCaptureOtherPaths(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()
	router.HandleFunc("/other/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	api := &API{Router: router}

	response := httptest.NewRecorder()
	api.ServeHTTP(response, httptest.NewRequest("PROPFIND", "/other/resource", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
}

func TestCalDAVExtensionMethodReachesSlashlessRoot(t *testing.T) {
	t.Parallel()

	var received string
	router := chi.NewRouter()
	router.Handle("/api/caldav", restoreCalDAVMethod(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Method
		w.WriteHeader(http.StatusNoContent)
	})))
	api := &API{Router: router}

	response := httptest.NewRecorder()
	api.ServeHTTP(response, httptest.NewRequest("PROPFIND", "/api/caldav", nil))

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "PROPFIND", received)
}

func TestCalDAVExtensionMethodReachesWellKnownDiscovery(t *testing.T) {
	t.Parallel()

	var received string
	router := chi.NewRouter()
	router.Handle("/.well-known/caldav", restoreCalDAVMethod(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Method
		w.WriteHeader(http.StatusNoContent)
	})))
	api := &API{Router: router}

	response := httptest.NewRecorder()
	api.ServeHTTP(response, httptest.NewRequest("PROPFIND", "/.well-known/caldav", nil))

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "PROPFIND", received)
}
