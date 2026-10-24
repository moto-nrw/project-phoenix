package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedAnalyticsConsentStepRecordsBothAnswersThroughTheConsentRoute(t *testing.T) {
	t.Parallel()

	type decision struct {
		token   string
		granted any
	}
	var decisions []decision
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"staff-token"}}`)
		case "/parent/auth/login":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
		case "/analytics-consent/":
			assert.Equal(t, "PUT", r.Method)
			body := map[string]any{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			decisions = append(decisions, decision{token: r.Header.Get("Authorization"), granted: body["granted"]})
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"granted":null}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	rt := &Runtime{
		Client:      newTestClient(srv.URL, false),
		Bootstrap:   &bootstrapSeedState{TenantSlug: "demo"},
		FixedSeeder: &FixedSeeder{staffCredentials: []StaffCredentials{{Email: "caregiver@example.test", Password: "Staff1234%"}}},
		Parents:     []ParentCredentials{{Email: "parent@example.test", Password: "Parent1234%"}},
	}
	rt.Adapter = rt.Client.adapter

	require.NoError(t, (seedAnalyticsConsentStep{}).Run(t.Context(), rt))
	assert.Equal(t, []decision{
		{token: "Bearer staff-token", granted: true},
		{token: "Bearer parent-token", granted: false},
	}, decisions)
}
