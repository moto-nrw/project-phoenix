package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedWeekProtocolStepUsesTheWorkerThenTheNormalCloseRoute(t *testing.T) {
	t.Parallel()
	var requests []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/week-protocols/seed":
			require.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "true", r.Header.Get(seedTokenHeader))
			_, _ = fmt.Fprint(w, `{"data":{"seeded":true}}`)
		case "/api/week-protocols":
			require.Equal(t, http.MethodGet, r.Method)
			_, _ = fmt.Fprint(w, `{"data":[{"id":"4711"}]}`)
		case "/api/week-protocols/4711/close":
			require.Equal(t, http.MethodPost, r.Method)
			_, _ = fmt.Fprint(w, `{"data":{"id":"4711","status":"closed"}}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	rt.Client.BindAuth(AuthRef{Kind: AuthBearer, Token: "admin"})
	require.NoError(t, (seedWeekProtocolStep{}).Run(t.Context(), rt))
	assert.Equal(t, []string{
		"POST /api/week-protocols/seed",
		"GET /api/week-protocols",
		"POST /api/week-protocols/4711/close",
	}, requests)
}
