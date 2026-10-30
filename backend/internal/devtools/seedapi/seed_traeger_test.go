package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	var invitations, assignments, operatorInvitations []map[string]any
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
		if r.URL.Path == "/operator/organizations/12/traeger-members" {
			assignments = append(assignments, body)
		}
		if r.URL.Path == "/operator/organizations/12/traeger-members/invitations" {
			operatorInvitations = append(operatorInvitations, body)
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
	assert.Equal(t, []map[string]any{
		{"email": "admin@example.test", "roles": []any{"administration", "billing"}},
	}, assignments, "the admin holds the Verwaltung and the Abrechnung, granted in one assignment")
	operatorInvite := requests["/operator/organizations/12/traeger-members/invitations"]
	assert.Equal(t, "Bearer operator-token", operatorInvite.authorization, "the moto team invites with the operator session")
	assert.Equal(t, []map[string]any{
		{"email": "petra.hoffmann@ganztag-nord.example", "roles": []any{"administration", "billing"}},
	}, operatorInvitations, "the operator invites a person without a moto account, with two roles at once")
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
	var period map[string]any
	var registrations []map[string]any
	var correction, release map[string]any
	var proofs int
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
		case "/traeger/holiday-registrations/link-key/uploads":
			proofs++
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"token":"proof-token"}}`))
		case "/traeger/holiday-registrations/link-key":
			raw, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			var registration map[string]any
			assert.NoError(t, json.Unmarshal(raw, &registration))
			registrations = append(registrations, registration)
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":%d}}`, 8+len(registrations))
		case "/traeger/billing-runs/9", "/traeger/billing-runs/9/corrections":
			if r.Method == http.MethodPost {
				raw, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.NoError(t, json.Unmarshal(raw, &correction))
				w.WriteHeader(http.StatusCreated)
			}
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":9,"preview_count":1,"preview_cents":11250,` +
				`"documents":[{"registration_id":9,"children":[{"child_id":91,"total_cents":4500}]}]}}`))
		case "/traeger/billing-runs/9/release":
			raw, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.NoError(t, json.Unmarshal(raw, &release))
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":9,"status":"released"}}`))
		default:
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
		"POST /traeger/contribution-rules/ Bearer traeger-token",
		"POST /traeger/contribution-rules/ Bearer traeger-token",
		"POST /traeger/holiday-quotes/link-key ",
		"POST /traeger/holiday-registrations/link-key ",
		"POST /traeger/holiday-registrations/link-key/uploads ",
		"POST /traeger/holiday-registrations/link-key ",
		"GET /traeger/holiday-periods/5/registrations Bearer traeger-token",
		"POST /traeger/holiday-contributions/5/registrations/9/confirm Bearer traeger-token",
		"PUT /traeger/number-ranges/holiday Bearer traeger-token",
		"POST /traeger/billing-runs/ Bearer traeger-token",
		"GET /traeger/billing-runs/9 Bearer traeger-token",
		"POST /traeger/billing-runs/9/corrections Bearer traeger-token",
		"POST /traeger/billing-runs/9/release Bearer traeger-token",
		"GET /traeger/billing-runs/9/export Bearer traeger-token",
	}, calls, "the registrations go through the public link without a session; the Träger-Büro reviews them with its session")
	assert.Equal(t, map[string]any{"registration_id": float64(9), "child_id": float64(91), "total_cents": float64(3500),
		"reason": "Am ersten Ferientag krank, Tag wird erstattet."}, correction, "the Abrechnung lowers one child with a reason")
	assert.Equal(t, map[string]any{"document_count": float64(1), "total_cents": float64(11250)}, release,
		"the release names the preview it checked")
	weeks, ok := period["weeks"].([]any)
	require.True(t, ok)
	require.Len(t, weeks, 2)
	assert.Equal(t, float64(31), weeks[0].(map[string]any)["host_school_id"])
	assert.Equal(t, float64(31), weeks[1].(map[string]any)["host_school_id"], "the profile's school hosts both weeks, never the manual test school")
	require.Len(t, registrations, 2)
	registration := registrations[0]
	children, ok := registration["children"].([]any)
	require.True(t, ok)
	require.Len(t, children, 2)
	assert.Equal(t, float64(31), children[0].(map[string]any)["home_school_id"])
	assert.Equal(t, float64(0), children[1].(map[string]any)["home_school_id"], "the second sibling is a holiday guest")
	assert.Equal(t, []any{
		map[string]any{"week_id": float64(51), "time_model_id": float64(61)},
		map[string]any{"week_id": float64(52), "time_model_id": float64(63)},
	}, children[1].(map[string]any)["bookings"])
	duplicate := registrations[1]
	assert.NotEqual(t, registration["guardian"], duplicate["guardian"], "another parent registers the child again")
	duplicateChildren, ok := duplicate["children"].([]any)
	require.True(t, ok)
	require.Len(t, duplicateChildren, 1)
	assert.Equal(t, children[0].(map[string]any)["first_name"], duplicateChildren[0].(map[string]any)["first_name"])
	assert.Equal(t, children[0].(map[string]any)["birth_date"], duplicateChildren[0].(map[string]any)["birth_date"],
		"same name and date of birth: a possible duplicate")
	assert.Equal(t, []any{map[string]any{"week_id": float64(52), "time_model_id": float64(63)}}, duplicateChildren[0].(map[string]any)["bookings"])
	assert.Equal(t, 1, proofs, "the second parent uploads the proof of benefits through the public link")
	assert.Equal(t, true, duplicate["benefits_declared"])
	assert.Equal(t, []any{"proof-token"}, duplicate["benefits_proof_tokens"], "the registration names the uploaded proof")
	assert.Nil(t, registration["benefits_proof_tokens"], "the siblings send no proof")
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

// The first seeded parent reads the prefill of the published link with the
// parents portal session (#4662); a family without a known child fails the
// step, so seed-smoke proves the prefill contract.
func TestSeedTraegerHolidayPeriodStepReadsTheParentPrefill(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		children string
		wantErr  bool
	}{
		{name: "known child", children: `[{"student_id":"7"}]`},
		{name: "no known child", children: `[]`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var prefill string
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/traeger/auth/login":
					_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
				case "/traeger/schools/":
					_, _ = w.Write([]byte(`{"status":"success","data":[{"id":31}]}`))
				case "/traeger/holiday-periods/":
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
				case "/traeger/holiday-periods/5/publish":
					_, _ = w.Write([]byte(`{"status":"success","data":{"id":5,"status":"published","public_token":"link-key","weeks":[` +
						`{"id":51,"time_models":[{"id":61},{"id":62}]},{"id":52,"time_models":[{"id":63}]}]}}`))
				case "/traeger/holiday-registrations/link-key/uploads":
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"status":"success","data":{"token":"proof-token"}}`))
				case "/parent/auth/login":
					_, _ = w.Write([]byte(`{"status":"success","data":{"access_token":"parent-token"}}`))
				case "/parent/holiday-registrations/link-key/family":
					prefill = r.Method + " " + r.Header.Get("Authorization")
					_, _ = w.Write([]byte(`{"status":"success","data":{"guardian":{},"children":` + tc.children + `}}`))
				case "/traeger/billing-runs/9", "/traeger/billing-runs/9/corrections":
					_, _ = w.Write([]byte(`{"status":"success","data":{"id":9,"documents":[{"registration_id":9,"children":[{"child_id":91}]}]}}`))
				default:
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"status":"success","data":{"id":9}}`))
				}
			})
			defer srv.Close()

			client := newTestClient(srv.URL, false)
			runtime := &Runtime{
				Adapter:   client.adapter,
				Client:    client,
				Bootstrap: &bootstrapSeedState{SchoolID: 31, AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
				Parents:   []ParentCredentials{{Email: "eltern@example.test", Password: testpkg.NewTestPassword()}},
			}
			err := (seedTraegerHolidayPeriodStep{}).Run(context.Background(), runtime)
			assert.Equal(t, "GET Bearer parent-token", prefill, "the prefill is read with the parent's session")
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// The hosting school's admin reads the participant list of its first week
// with the school session (#4665); a week without a confirmed child fails the
// step, so seed-smoke proves the host access contract.
func TestSeedTraegerHolidayPeriodStepReadsTheHostedParticipantList(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		weeks   string
		wantErr bool
	}{
		{name: "confirmed children", weeks: `[{"id":51,"participants":2}]`},
		{name: "no confirmed child", weeks: `[{"id":51,"participants":0}]`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var reads []string
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/traeger/auth/login":
					_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
				case "/traeger/schools/":
					_, _ = w.Write([]byte(`{"status":"success","data":[{"id":31}]}`))
				case "/traeger/holiday-periods/5/publish":
					_, _ = w.Write([]byte(`{"status":"success","data":{"id":5,"status":"published","public_token":"link-key","weeks":[` +
						`{"id":51,"time_models":[{"id":61},{"id":62}]},{"id":52,"time_models":[{"id":63}]}]}}`))
				case "/api/holiday-weeks/":
					reads = append(reads, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
					_, _ = w.Write([]byte(`{"status":"success","data":` + tc.weeks + `}`))
				case "/api/holiday-weeks/51/participants":
					reads = append(reads, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
					_, _ = w.Write([]byte(`{"status":"success","data":{"participants":[]}}`))
				case "/traeger/holiday-registrations/link-key/uploads":
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"status":"success","data":{"token":"proof-token"}}`))
				case "/api/holiday-weeks/51/guest-group":
					// The holiday guests join the new group (#4666).
					reads = append(reads, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
					_, _ = w.Write([]byte(`{"status":"success","data":{"week_id":51,"group_id":5,"guests":1}}`))
				case "/traeger/billing-runs/5", "/traeger/billing-runs/5/corrections":
					// The billing run of the same step (#4668) checks one invoice.
					_, _ = w.Write([]byte(`{"status":"success","data":{"id":5,"documents":[{"registration_id":9,"children":[{"child_id":91}]}]}}`))
				default:
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
				}
			})
			defer srv.Close()

			runtime := &Runtime{
				Client:     newTestClient(srv.URL, false),
				Bootstrap:  &bootstrapSeedState{SchoolID: 31, AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
				TenantAuth: AuthRef{Kind: AuthBearer, Label: "school admin", Token: "school-token"},
			}
			err := (seedTraegerHolidayPeriodStep{}).Run(context.Background(), runtime)
			if tc.wantErr {
				assert.ErrorContains(t, err, "no confirmed child")
				assert.Equal(t, []string{"GET /api/holiday-weeks/ Bearer school-token"}, reads)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{
				"GET /api/holiday-weeks/ Bearer school-token",
				"GET /api/holiday-weeks/51/participants Bearer school-token",
				"PUT /api/holiday-weeks/51/guest-group Bearer school-token",
			}, reads, "the hosting school reads its list and takes its holiday guests with its own school session")
		})
	}
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
		case "/traeger/holiday-registrations/link-key", "/traeger/holiday-quotes/link-key", "/traeger/holiday-registrations/link-key/uploads":
			// The public registration of the holiday step (#4661), its
			// contribution (#4664) and its proof of benefits (#5238) have no
			// session by design; they are no Träger requests of the shared
			// one.
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":9,"token":"proof-token"}}`))
			return
		case "/traeger/billing-runs/5", "/traeger/billing-runs/5/corrections":
			// The billing run of the holiday step (#4668) checks one invoice.
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5,"documents":[{"registration_id":9,"children":[{"child_id":91}]}]}}`))
		default:
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
		}
		if !strings.HasPrefix(r.URL.Path, "/operator/organizations/12/traeger-members") {
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

// A second demo school of the same Träger finds the Träger's rules already
// kept (#4664): the overlap refusal leaves them as they are.
func TestSeedContributionRulesKeepTheTraegersRules(t *testing.T) {
	t.Parallel()

	posts := 0
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		posts++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":409,"code":"parent_billing.contribution_rule_overlap","detail":"overlap"}`))
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	require.NoError(t, seedContributionRules(client, AuthRef{Kind: AuthBearer, Token: "traeger-token"}, calendar.NewDate(2026, time.September, 23)))
	assert.Equal(t, 2, posts, "both kinds were offered once")
}
