package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedHomeLayoutStepUsesStartPageAPIs(t *testing.T) {
	t.Parallel()

	var requests []struct {
		path string
		body map[string]any
	}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		body := map[string]any{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.Path, body: body})
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Token: "admin"}}
	require.NoError(t, (seedHomeLayoutStep{}).Run(t.Context(), rt))

	require.Len(t, requests, 2)
	assert.Equal(t, "/api/settings/home-layout", requests[0].path)
	assert.Equal(t, map[string]any{
		"overrides": map[string]any{"section.birthdays": false},
	}, requests[0].body)
	assert.Equal(t, "/api/settings/home-layout/policies", requests[1].path)
	assert.Equal(t, map[string]any{
		"policies": map[string]any{
			"tile.students_sick": "required",
			"tile.students_home": "disabled",
		},
	}, requests[1].body)
}
