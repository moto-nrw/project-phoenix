package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedExternalCaregiversStepDoesNotDuplicateExistingCaregivers(t *testing.T) {
	t.Parallel()

	externals := make([]seedExternalCaregiver, 0, 3)
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/staff/":
			data := make([]map[string]any, 0, len(externals))
			for _, external := range externals {
				data = append(data, map[string]any{
					"is_external":           true,
					"external_organization": external.Organization,
					"person": map[string]string{
						"first_name": external.FirstName,
						"last_name":  external.LastName,
					},
				})
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/staff/externals":
			var external seedExternalCaregiver
			require.NoError(t, json.NewDecoder(r.Body).Decode(&external))
			externals = append(externals, external)
			_, _ = w.Write([]byte(`{"status":"success"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	step := seedExternalCaregiversStep{}
	require.NoError(t, step.Run(t.Context(), rt))
	require.NoError(t, step.Run(t.Context(), rt))

	assert.Len(t, externals, 3)
}
