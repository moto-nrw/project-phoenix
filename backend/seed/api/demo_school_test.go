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

// The visitor of a public demo school appears exactly once among the staff:
// as the caregiver who leads a group, with every function until the demo
// roles exist. A seed person of the same name takes the displaced name.
func TestFixedSeeder_SeedStaffAccounts_NamesOneCaregiverAfterTheVisitor(t *testing.T) {
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
	fs.accountScope = "ogs-nord-k3m9xp"
	fs.visitorName = "  Anna   Müller "
	fs.roleIDs["admin"], fs.roleIDs["user"], fs.roleIDs["guest"] = 1, 2, 3

	require.NoError(t, fs.seedStaffAccounts(context.Background(), &FixedResult{}))
	require.Len(t, registered, len(DemoStaff))
	names := make(map[string]int)
	for _, body := range registered {
		names[fmt.Sprintf("%s %s", body["first_name"], body["last_name"])]++
		assert.Contains(t, body["email"], ".ogs-nord-k3m9xp@", "every account keeps a synthetic, scoped address")
	}
	assert.Equal(t, 1, names["Anna Müller"], "the visitor's name appears exactly once")
	assert.Equal(t, 1, names["Julia Klein"], "the seed person of the same name takes the displaced name")
	assert.Equal(t, "Anna", registered[visitorStaffIndex]["first_name"])
	assert.InDelta(t, 1, registered[visitorStaffIndex]["role_id"], 0)
	assert.Equal(t, "Julia Klein", fs.staffCredentials[visitorStaffIndex].Name, "the internal key keeps the seed name")
}

func TestVisitorDisplayName(t *testing.T) {
	t.Parallel()

	first, last := visitorDisplayName("", true, "Julia", "Klein", "Julia", "Klein")
	assert.Equal(t, "Julia Klein", first+" "+last, "without a visitor the seed names stay")
	first, last = visitorDisplayName("Maria von Berg", true, "Julia", "Klein", "Julia", "Klein")
	assert.Equal(t, []string{"Maria", "von Berg"}, []string{first, last})
	first, last = visitorDisplayName("Kim", true, "Sabine", "Schneider", "Sabine", "Schneider")
	assert.Equal(t, []string{"Kim", "Schneider"}, []string{first, last}, "a single name keeps the family name of the child")
	first, last = visitorDisplayName("Kim Beispiel", false, "Petra", "Meyer", "Sabine", "Schneider")
	assert.Equal(t, "Petra Meyer", first+" "+last)
}

// A repeated seed finds the school a broken attempt left under its slug. It
// moves that school aside and seeds under a fresh account scope.
func TestBootstrapTenant_ReplacesTheSchoolOfABrokenSeed(t *testing.T) {
	t.Parallel()

	var creations int
	var renamed map[string]any
	var deleted bool
	var invited map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		var body map[string]any
		if r.Body != nil && r.Method != seedHTTPMethodGet {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/operator/organizations":
			_, _ = fmt.Fprint(w, `{"data":{"id":7}}`)
		case r.URL.Path == "/operator/schools" && r.Method == seedHTTPMethodPost:
			creations++
			if creations == 1 {
				w.WriteHeader(seedHTTPStatusConflict)
				_, _ = fmt.Fprint(w, `{"error":"subdomain already exists"}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"data":{"id":12,"subdomain":"ogs-nord-k3m9xp"}}`)
		case r.URL.Path == "/operator/schools":
			_, _ = fmt.Fprint(w, `{"data":[{"id":3,"organization_id":7,"name":"Andere","subdomain":"andere"},{"id":11,"organization_id":7,"name":"OGS Nord","subdomain":"ogs-nord-k3m9xp"}]}`)
		case r.URL.Path == "/operator/schools/11" && r.Method == seedHTTPMethodPut:
			renamed = body
			_, _ = fmt.Fprint(w, `{"data":{"id":11}}`)
		case r.URL.Path == "/operator/schools/11" && r.Method == seedHTTPMethodDelete:
			deleted = true
			_, _ = fmt.Fprint(w, `{"data":{"id":11}}`)
		case r.URL.Path == "/operator/schools/12/invite-admin":
			invited = body
			_, _ = fmt.Fprint(w, `{"data":{"token":"invite-token"}}`)
		case r.URL.Path == "/auth/invitations/invite-token/accept":
			_, _ = fmt.Fprint(w, `{"status":"success"}`)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	seeder := NewSeeder(newSeedTestAdapter(srv.URL), newSeedTestRandom(), false, SeedOptions{
		TenantSlug: "ogs-nord-k3m9xp", SchoolName: "OGS Nord", OnlyProfile: DefaultProfileKey,
		ReplaceAbandoned: true, AccountScope: "ogs-nord-k3m9xp-2",
	})
	seeder.client.BindAuth(AuthRef{Kind: AuthBearer, Token: "operator"})
	bootstrap, err := seeder.bootstrapTenant(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(12), bootstrap.SchoolID)
	assert.Equal(t, "x11-ogs-nord-k3m9xp", renamed["subdomain"], "the unique subdomain is freed before the soft delete")
	assert.Equal(t, false, renamed["active"])
	assert.True(t, deleted)
	assert.Equal(t, "vollbetrieb-admin.ogs-nord-k3m9xp-2@example.test", invited["email"], "the abandoned school keeps its accounts, so the repetition needs its own scope")
}

// The demo role parent (#3468) signs the visitor in as the parent whose
// guardian carries the visitor's name; the internal key keeps the seed name.
func TestVisitorParentAccountID(t *testing.T) {
	t.Parallel()

	profile := &SeedProfile{}
	assert.Zero(t, VisitorParentAccountID(profile), "a school without parents names none")
	visitor := DemoGuardians[visitorGuardianIndex]
	coParent := DemoGuardians[visitorGuardianIndex+1]
	profile.Credentials.Parents = []ParentCredentials{
		{Name: coParent.FirstName + " " + coParent.LastName, AccountID: 7},
		{Name: visitor.FirstName + " " + visitor.LastName, AccountID: 9},
	}
	assert.Equal(t, int64(9), VisitorParentAccountID(profile))
}
