package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoutePatternFallsBackForUnmatchedRequests(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)

	assert.Equal(t, "unmatched", RoutePattern(request))
}
