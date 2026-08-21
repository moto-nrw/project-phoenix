package common_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// WithIdentityRequestCache returns its input unchanged when a cache is already
// attached, so comparing the handler-observed context with a re-wrapped copy
// proves the middleware attached the cache (#2099).
func TestRequestIdentityCacheMiddlewareAttachesCache(t *testing.T) {
	t.Parallel()

	var observed context.Context
	handler := common.RequestIdentityCacheMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		observed = r.Context()
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	require.NotNil(t, observed)
	assert.Equal(t, observed, usercontext.WithIdentityRequestCache(observed),
		"middleware must have attached the request cache (idempotent re-wrap returns the same context)")
}
