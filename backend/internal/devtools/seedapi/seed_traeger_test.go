package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type seedTraegerRequest struct {
	authorization string
	body          map[string]any
}

func TestSeedTraegerBueroStepAssignsTheSchoolAdminThroughTheOperatorRoute(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	requests := map[string]seedTraegerRequest{}
	var invitations []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, http.MethodPost, r.Method)
		raw, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		var body map[string]any
		assert.NoError(t, json.Unmarshal(raw, &body))
		mu.Lock()
		requests[r.URL.Path] = seedTraegerRequest{authorization: r.Header.Get("Authorization"), body: body}
		if r.URL.Path == "/traeger/accounts/invitations" {
			invitations = append(invitations, body)
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"traeger-refresh"}`))
		default:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"account_id":7,"roles":["administration"]}}`))
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	tenantAuth := AuthRef{Kind: AuthBearer, Token: "tenant-token"}
	client.BindAuth(tenantAuth)
	runtime := &Runtime{
		Client:       client,
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		Bootstrap:    &bootstrapSeedState{OrganizationID: 12, AdminEmail: "admin@example.test", AdminPassword: "Geheim1234%"},
	}

	require.NoError(t, (seedTraegerBueroStep{}).Run(context.Background(), runtime))
	assign := requests["/operator/organizations/12/traeger-members"]
	assert.Equal(t, "Bearer operator-token", assign.authorization)
	assert.Equal(t, map[string]any{"email": "admin@example.test", "role": "administration"}, assign.body)
	login := requests["/traeger/auth/login"]
	assert.Empty(t, login.authorization, "the Träger sign-in carries no session")
	assert.Equal(t, map[string]any{"email": "admin@example.test", "password": "Geheim1234%", "organization_id": float64(12)}, login.body)
	invite := requests["/traeger/accounts/invitations"]
	assert.Equal(t, "Bearer traeger-token", invite.authorization, "the invitation uses the Träger session")
	assert.Equal(t, []map[string]any{
		{"email": "jana.krause@ganztag-nord.example", "roles": []any{"billing"}},
		{"email": "tobias.engel@ganztag-nord.example", "roles": []any{"personnel"}},
	}, invitations, "one colleague per role besides the Verwaltung, with personal addresses")
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
		Bootstrap: &bootstrapSeedState{SchoolID: 31, AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
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
	assert.Equal(t, float64(31), weeks[1].(map[string]any)["host_school_id"], "the profile's school hosts both weeks, never the manual test school")
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

// The Träger plans the two Herbstferien weeks the school plans around, under
// the name of that period, and opens registration on the day it publishes.
func TestSeedHolidayPeriodCoversTheSeededHerbstferien(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, time.September, 23)
	period := seedHolidayPeriod(today, 31)
	assert.Equal(t, "Ferienbetreuung Herbstferien 2026", period["name"])
	assert.Equal(t, "2026-09-23", period["registration_opens_on"])
	assert.Equal(t, "2026-10-05", period["registration_closes_on"], "a week before the Ferien")
	weeks := period["weeks"].([]map[string]any)
	require.Len(t, weeks, 2)
	ferienStart, ferienEnd := seedHerbstferien(seedDate{Time: today.UTCMidnight()})
	assert.Equal(t, ferienStart.String(), weeks[0]["starts_on"])
	assert.Equal(t, "2026-10-12", weeks[0]["starts_on"])
	assert.Equal(t, "2026-10-16", weeks[0]["ends_on"])
	assert.Equal(t, "2026-10-19", weeks[1]["starts_on"])
	assert.Equal(t, ferienEnd.String(), weeks[1]["ends_on"])
}

// A Träger that does not list the profile's school publishes nothing: the
// weeks would otherwise go to a school no family of the demo attends.
func TestSeedTraegerHolidayPeriodStepRequiresTheProfileSchool(t *testing.T) {
	t.Parallel()

	var created bool
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
		case "/traeger/schools/":
			_, _ = w.Write([]byte(`{"status":"success","data":[{"id":32}]}`))
		default:
			created = true
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
		}
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:    newTestClient(srv.URL, false),
		Bootstrap: &bootstrapSeedState{SchoolID: 31, AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
	}
	require.ErrorContains(t, (seedTraegerHolidayPeriodStep{}).Run(context.Background(), runtime), "does not list school 31")
	assert.False(t, created)
}

func TestSeedTraegerSettingsStepSetsAValueThroughTheTraegerPortal(t *testing.T) {
	t.Parallel()

	var calls []string
	var authorization string
	var value map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		raw, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		if r.URL.Path == "/traeger/auth/login" {
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"refresh"}`))
			return
		}
		authorization = r.Header.Get("Authorization")
		assert.NoError(t, json.Unmarshal(raw, &value))
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:    newTestClient(srv.URL, false),
		Bootstrap: &bootstrapSeedState{AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
	}
	require.NoError(t, (seedTraegerSettingsStep{}).Run(context.Background(), runtime))
	assert.Equal(t, []string{"POST /traeger/auth/login", "PUT /traeger/settings/values/operations.parent_news_enabled"}, calls)
	assert.Equal(t, "Bearer traeger-token", authorization, "the value is written with the Träger session")
	assert.Equal(t, true, value["value"])
}

// Every Träger sign-in mails a second-factor code and Identity & Access mails
// at most three per account in fifteen minutes (#4651), so the Träger steps
// of one run share one session instead of signing in each.
func TestSeedTraegerStepsShareOneTraegerSession(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	logins := 0
	var traegerAuthorizations []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			logins++
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
			return
		case "/traeger/schools/":
			_, _ = w.Write([]byte(`{"status":"success","data":[{"id":31}]}`))
		case "/traeger/holiday-periods/":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
		case "/traeger/holiday-periods/5/publish":
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5,"status":"published","public_token":"link-key","weeks":[` +
				`{"id":51,"time_models":[{"id":61},{"id":62}]},{"id":52,"time_models":[{"id":63},{"id":64}]}]}}`))
		case "/traeger/holiday-registrations/link-key":
			// The public registration of the holiday step (#4661) has no
			// session by design; it is no Träger request of the shared one.
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":9}}`))
			return
		default:
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
		}
		if r.URL.Path != "/operator/organizations/12/traeger-members" {
			traegerAuthorizations = append(traegerAuthorizations, r.Header.Get("Authorization"))
		}
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:       newTestClient(srv.URL, false),
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		Bootstrap:    &bootstrapSeedState{OrganizationID: 12, SchoolID: 31, AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
	}
	require.NoError(t, (seedTraegerBueroStep{}).Run(context.Background(), runtime))
	require.NoError(t, (seedTraegerHolidayPeriodStep{}).Run(context.Background(), runtime))
	require.NoError(t, (seedTraegerSettingsStep{}).Run(context.Background(), runtime))
	assert.Equal(t, 1, logins, "the Träger steps sign in once")
	require.NotEmpty(t, traegerAuthorizations)
	for _, authorization := range traegerAuthorizations {
		assert.Equal(t, "Bearer traeger-token", authorization, "every Träger request uses the shared session")
	}
}
