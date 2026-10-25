package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSeedTraegerBueroStepAssignsTheSchoolAdminThroughTheOperatorRoute(t *testing.T) {
	t.Parallel()

	var authorization, path string
	var body map[string]string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, http.MethodPost, r.Method)
		path = r.URL.Path
		authorization = r.Header.Get("Authorization")
		raw, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.NoError(t, json.Unmarshal(raw, &body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"success","data":{"account_id":7,"roles":["administration"]}}`))
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	tenantAuth := AuthRef{Kind: AuthBearer, Token: "tenant-token"}
	client.BindAuth(tenantAuth)
	runtime := &Runtime{
		Client:       client,
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		Bootstrap:    &bootstrapSeedState{OrganizationID: 12, AdminEmail: "admin@example.test"},
	}

	require.NoError(t, (seedTraegerBueroStep{}).Run(context.Background(), runtime))
	assert.Equal(t, "/operator/organizations/12/traeger-members", path)
	assert.Equal(t, "Bearer operator-token", authorization)
	assert.Equal(t, map[string]string{"email": "admin@example.test", "role": "administration"}, body)
	assert.Equal(t, tenantAuth, client.auth, "the step restores the client's own session")
}

func TestSeedTraegerHolidayPeriodStepPublishesAPeriodAtTheTraegerSchools(t *testing.T) {
	t.Parallel()

	var calls []string
	var period, registration map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
		case "/traeger/schools/":
			_, _ = w.Write([]byte(`{"status":"success","data":[{"id":31},{"id":32}]}`))
		case "/traeger/holiday-periods/":
			raw, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.NoError(t, json.Unmarshal(raw, &period))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
		case "/traeger/holiday-periods/5/publish":
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5,"status":"published","public_token":"link-key","weeks":[` +
				`{"id":51,"time_models":[{"id":61},{"id":62}]},{"id":52,"time_models":[{"id":63},{"id":64}]}]}}`))
		default:
			raw, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.NoError(t, json.Unmarshal(raw, &registration))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":9}}`))
		}
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:    newTestClient(srv.URL, false),
		Bootstrap: &bootstrapSeedState{AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
	}
	require.NoError(t, (seedTraegerHolidayPeriodStep{}).Run(context.Background(), runtime))
	assert.Equal(t, []string{
		"POST /traeger/auth/login ",
		"GET /traeger/schools/ Bearer traeger-token",
		"POST /traeger/holiday-periods/ Bearer traeger-token",
		"POST /traeger/holiday-periods/5/publish Bearer traeger-token",
		"POST /traeger/holiday-registrations/link-key ",
	}, calls, "the registration goes through the public link without a session")
	weeks, ok := period["weeks"].([]any)
	require.True(t, ok)
	require.Len(t, weeks, 2)
	assert.Equal(t, float64(31), weeks[0].(map[string]any)["host_school_id"])
	assert.Equal(t, float64(32), weeks[1].(map[string]any)["host_school_id"], "the weeks take turns at the Träger's schools")
	children, ok := registration["children"].([]any)
	require.True(t, ok)
	require.Len(t, children, 2)
	assert.Equal(t, float64(31), children[0].(map[string]any)["home_school_id"])
	assert.Equal(t, float64(0), children[1].(map[string]any)["home_school_id"], "the second sibling is a holiday guest")
	assert.Equal(t, []any{
		map[string]any{"week_id": float64(51), "time_model_id": float64(61)},
		map[string]any{"week_id": float64(52), "time_model_id": float64(63)},
	}, children[1].(map[string]any)["bookings"])
}
