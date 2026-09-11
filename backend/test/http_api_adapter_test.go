package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPAPIAdapterCheckHealthRejectsNonOKSuccessStatus(t *testing.T) {
	t.Parallel()

	server := NewHTTPTestServer(func(response HTTPResponseWriter, _ *HTTPRequest) {
		response.WriteHeader(HTTPStatusNoContent)
	})
	defer server.Close()

	err := NewHTTPAPIAdapter(server.URL).CheckHealth(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GET /health failed: status 204")
}
