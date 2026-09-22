package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSeedSchoolSetupStepCompletesTheWizard pins that the demo school keeps
// its configured presence mode, skips only the open steps, completes setup and
// hides the wizard, all through the wizard routes.
func TestSeedSchoolSetupStepCompletesTheWizard(t *testing.T) {
	t.Parallel()

	type request struct {
		method string
		path   string
		body   map[string]any
	}
	var requests []request
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		body := map[string]any{}
		if r.Method != http.MethodGet {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		requests = append(requests, request{method: r.Method, path: r.URL.Path, body: body})
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"basics":{"presence_mode":"binary"},"steps":[`+
				`{"key":"basics","applies":true,"done":false,"skipped":false},`+
				`{"key":"team","applies":true,"done":true,"skipped":false},`+
				`{"key":"rooms","applies":false,"done":false,"skipped":false},`+
				`{"key":"groups","applies":true,"done":false,"skipped":false},`+
				`{"key":"guardians","applies":true,"done":false,"skipped":true}]}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Token: "admin"}}
	require.NoError(t, (seedSchoolSetupStep{}).Run(t.Context(), rt))

	require.Len(t, requests, 5)
	assert.Equal(t, request{method: http.MethodGet, path: "/api/school-setup", body: map[string]any{}}, requests[0])
	assert.Equal(t, "/api/school-setup/basics", requests[1].path)
	assert.Equal(t, map[string]any{"presence_mode": "binary", "parent_app_used": true}, requests[1].body)
	assert.Equal(t, "/api/school-setup/steps/groups", requests[2].path, "only the open, applicable step is skipped")
	assert.Equal(t, map[string]any{"skipped": true}, requests[2].body)
	assert.Equal(t, "/api/school-setup/complete", requests[3].path)
	assert.Equal(t, "/api/school-setup/dismissal", requests[4].path)
	assert.Equal(t, map[string]any{"dismissed": true}, requests[4].body)
}
