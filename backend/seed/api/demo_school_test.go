package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopedEmail(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "demo1@mail.de", scopedEmail("demo1@mail.de", ""))
	assert.Equal(t, "demo1.ogs-nord@mail.de", scopedEmail("demo1@mail.de", "ogs-nord"))
	assert.Equal(t, "no-at-sign", scopedEmail("no-at-sign", "ogs-nord"))
}

func TestAccountScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options SeedOptions
		want    string
	}{
		{name: "no slug keeps the local credentials", options: SeedOptions{}, want: ""},
		{name: "the profile's own slug keeps the local credentials", options: SeedOptions{TenantSlug: DefaultProfileKey}, want: ""},
		{name: "a slug from outside scopes the accounts", options: SeedOptions{TenantSlug: "ogs-nord"}, want: "ogs-nord"},
		{name: "randomize has its own suffixes", options: SeedOptions{TenantSlug: "ogs-nord", Randomize: true}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			seeder := NewSeeder(newSeedTestAdapter("http://localhost:8080"), newSeedTestRandom(), false, tt.options)
			assert.Equal(t, tt.want, seeder.accountScope())
		})
	}
}

func TestFullDemoWorkflowOnlyProfileSkipsFurtherSchools(t *testing.T) {
	t.Parallel()

	furtherSchools := func(options SeedOptions) int {
		count := 0
		for _, step := range fullDemoWorkflow(&Seeder{options: options}).Steps {
			switch step.(type) {
			case manualProfileStep, seedEnrollmentWeeklyProfileStep, seedEnrollmentBookingsProfileStep:
				count++
			}
		}
		return count
	}
	assert.Equal(t, 3, furtherSchools(SeedOptions{}))
	assert.Equal(t, 0, furtherSchools(SeedOptions{OnlyProfile: DefaultProfileKey}))

	restricted := fullDemoWorkflow(&Seeder{options: SeedOptions{OnlyProfile: DefaultProfileKey}}).Steps
	_, writesState := restricted[len(restricted)-2].(buildStateStep)
	assert.True(t, writesState, "a restricted run still writes its seed state")
}

func TestSeedRejectsUnknownOnlyProfile(t *testing.T) {
	t.Parallel()

	seeder := NewSeeder(newSeedTestAdapter("http://localhost:8080"), newSeedTestRandom(), false, SeedOptions{OnlyProfile: ManualProfileKey})
	_, err := seeder.Seed(context.Background(), "operator@example.test", "secret", "1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"vollbetrieb"`)
}

// A second demo school finds the Demo-Träger of the first one and carries its
// slug in the admin account, so both runs fit into one database.
func TestBootstrapTenant_DemoSchoolJoinsExistingOrganization(t *testing.T) {
	t.Parallel()

	requests := make(map[string]map[string]any)
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		var body map[string]any
		if r.Body != nil && r.Method == seedHTTPMethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			requests[r.URL.Path] = body
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/operator/organizations" && r.Method == seedHTTPMethodPost:
			w.WriteHeader(seedHTTPStatusConflict)
			_, _ = fmt.Fprint(w, `{"error":"slug already exists"}`)
		case r.URL.Path == "/operator/organizations":
			_, _ = fmt.Fprint(w, `{"data":[{"id":4,"slug":"other"},{"id":4207,"slug":"demo-traeger-nord"}]}`)
		case r.URL.Path == "/operator/schools":
			_, _ = fmt.Fprint(w, `{"data":{"id":2,"subdomain":"ogs-nord"}}`)
		case r.URL.Path == "/operator/schools/2/invite-admin":
			_, _ = fmt.Fprint(w, `{"data":{"token":"invite-token"}}`)
		case r.URL.Path == "/auth/invitations/invite-token/accept":
			_, _ = fmt.Fprint(w, `{"status":"success"}`)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	seeder := NewSeeder(newSeedTestAdapter(srv.URL), newSeedTestRandom(), false, SeedOptions{
		TenantSlug: "ogs-nord", SchoolName: "OGS Nord", OnlyProfile: DefaultProfileKey,
	})
	seeder.client.BindAuth(AuthRef{Kind: AuthBearer, Token: "operator"})
	bootstrap, err := seeder.bootstrapTenant(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(4207), bootstrap.OrganizationID)
	assert.InDelta(t, 4207, requests["/operator/schools"]["organization_id"], 0)
	assert.Equal(t, "OGS Nord", requests["/operator/schools"]["name"])
	assert.Equal(t, "ogs-nord", requests["/operator/schools"]["slug"])
	assert.Equal(t, "vollbetrieb-admin.ogs-nord@example.test", requests["/operator/schools/2/invite-admin"]["email"])
	assert.Equal(t, "OGS Nord", bootstrap.SchoolName)
}

func TestFixedSeeder_SeedStaffAccounts_CarriesAccountScope(t *testing.T) {
	t.Parallel()

	var registered []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		registered = append(registered, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":%d,"school_identity":{"person_id":"%d"}}}`, len(registered), len(registered))
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.accountScope = "ogs-nord"
	fs.roleIDs["admin"], fs.roleIDs["user"], fs.roleIDs["guest"] = 1, 2, 3

	require.NoError(t, fs.seedStaffAccounts(context.Background(), &FixedResult{}))
	require.Len(t, registered, len(DemoStaff))
	assert.Equal(t, "demo1.ogs-nord@mail.de", registered[0]["email"])
	assert.Equal(t, "demo1-ogs-nord", registered[0]["username"])
	assert.Equal(t, "demo1.ogs-nord@mail.de", fs.staffCredentials[0].Email)
}
