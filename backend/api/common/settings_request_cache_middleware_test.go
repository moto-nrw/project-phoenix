package common_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// WithSettingsRequestCache returns its input unchanged when a cache is already
// attached, so comparing the handler-observed context with a re-wrapped copy
// proves the middleware attached the cache.
func TestRequestSettingsCacheMiddlewareAttachesCache(t *testing.T) {
	var observed context.Context
	handler := common.RequestSettingsCacheMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		observed = r.Context()
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	require.NotNil(t, observed)
	assert.Equal(t, observed, configSvc.WithSettingsRequestCache(observed),
		"middleware must have attached the request cache (idempotent re-wrap returns the same context)")
}
